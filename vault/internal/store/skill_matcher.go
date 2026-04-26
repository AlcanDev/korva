// Package store — skill_matcher.go implements the Smart Skill Auto-Loader.
//
// The matcher inspects a developer's prompt + project context and returns the
// skills that should be silently injected into the AI's context. The goal is
// transparency: the developer types naturally, and Korva loads the right
// conventions, examples, and guardrails without the developer needing to
// invoke any skill explicitly.
//
// Match signal sources (in order of weight):
//  1. file_patterns  — strongest signal; the developer is editing matching files
//  2. keywords       — prompt contains a trigger keyword
//  3. projects       — listed project name matches the active project
//  4. tags           — prompt mentions a skill's tag
//
// The matcher is intentionally heuristic (no ML) so it stays fast, deterministic,
// and explainable. Each match comes with a `reason` field so the UI can show
// the developer WHY a skill was loaded.
package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// SkillTriggers is the JSON shape stored in skills.triggers.
type SkillTriggers struct {
	Keywords     []string `json:"keywords,omitempty"`
	Projects     []string `json:"projects,omitempty"`
	FilePatterns []string `json:"file_patterns,omitempty"`
	Priority     int      `json:"priority,omitempty"`
}

// MatchedSkill is one auto-selected skill returned by MatchSkills.
type MatchedSkill struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Body     string   `json:"body"`
	Tags     []string `json:"tags"`
	Score    float64  `json:"score"`  // 0-1 relevance
	Reason   string   `json:"reason"` // human-readable match explanation
	Priority int      `json:"priority"`
}

// SkillMatchInput is the context the matcher uses to score skills.
type SkillMatchInput struct {
	TeamID    string
	Project   string
	Prompt    string
	FilePaths []string // current working set of files (optional)
	Limit     int      // max skills to return (default 5)
}

// MatchSkills returns the most relevant skills for the given context.
// Only skills with auto_load=1 and non-empty triggers are considered.
func (s *Store) MatchSkills(in SkillMatchInput) ([]MatchedSkill, error) {
	if in.Limit <= 0 {
		in.Limit = 5
	}

	rows, err := s.db.Query(`
		SELECT id, name, body, COALESCE(tags, '[]'), COALESCE(triggers, '{}')
		  FROM skills
		 WHERE team_id = ? AND auto_load = 1`, in.TeamID)
	if err != nil {
		return nil, fmt.Errorf("match skills query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	prompt := strings.ToLower(in.Prompt)
	project := strings.ToLower(in.Project)

	var matches []MatchedSkill
	for rows.Next() {
		var id, name, body, tagsJSON, triggersJSON string
		if err := rows.Scan(&id, &name, &body, &tagsJSON, &triggersJSON); err != nil {
			return nil, err
		}

		var tags []string
		_ = json.Unmarshal([]byte(tagsJSON), &tags)

		var trig SkillTriggers
		if err := json.Unmarshal([]byte(triggersJSON), &trig); err != nil {
			continue // skip malformed triggers
		}

		score, reason := scoreSkill(trig, tags, prompt, project, in.FilePaths)
		if score <= 0 {
			continue
		}

		matches = append(matches, MatchedSkill{
			ID:       id,
			Name:     name,
			Body:     body,
			Tags:     tags,
			Score:    score,
			Reason:   reason,
			Priority: trig.Priority,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by (priority desc, score desc) so high-priority skills win ties.
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority > matches[j].Priority
		}
		return matches[i].Score > matches[j].Score
	})

	if len(matches) > in.Limit {
		matches = matches[:in.Limit]
	}
	return matches, nil
}

// scoreSkill computes a 0-1 relevance score using weighted signals.
//
// Weights are chosen so that file-pattern matches dominate (the developer is
// concretely working with matching code), followed by keyword matches in the
// prompt, then project membership, then tag matches.
func scoreSkill(trig SkillTriggers, tags []string, prompt, project string, filePaths []string) (float64, string) {
	const (
		wFilePattern = 0.45
		wKeyword     = 0.30
		wProject     = 0.15
		wTag         = 0.10
	)

	var score float64
	var reasons []string

	// 1. File-pattern matches — strongest signal.
	if len(trig.FilePatterns) > 0 && len(filePaths) > 0 {
		for _, pattern := range trig.FilePatterns {
			for _, path := range filePaths {
				if matched, _ := filepath.Match(pattern, path); matched {
					score += wFilePattern
					reasons = append(reasons, "file matches "+pattern)
					goto nextSignal // only count once per skill
				}
			}
		}
	}
nextSignal:

	// 2. Keyword matches in prompt.
	if len(trig.Keywords) > 0 && prompt != "" {
		for _, kw := range trig.Keywords {
			if strings.Contains(prompt, strings.ToLower(kw)) {
				score += wKeyword
				reasons = append(reasons, "prompt mentions \""+kw+"\"")
				break
			}
		}
	}

	// 3. Project listed in triggers.
	if len(trig.Projects) > 0 && project != "" {
		for _, p := range trig.Projects {
			if strings.EqualFold(p, project) {
				score += wProject
				reasons = append(reasons, "active project is "+project)
				break
			}
		}
	} else if len(trig.Projects) == 0 && len(trig.FilePatterns) == 0 && len(trig.Keywords) == 0 {
		// Skill has empty triggers but auto_load=1 — treat as global low-priority hint.
		score += 0.05
		reasons = append(reasons, "global skill")
	}

	// 4. Tag matches in prompt.
	if len(tags) > 0 && prompt != "" {
		for _, tag := range tags {
			if strings.Contains(prompt, strings.ToLower(tag)) {
				score += wTag
				reasons = append(reasons, "tag \""+tag+"\" in prompt")
				break
			}
		}
	}

	if score > 1 {
		score = 1
	}
	reason := strings.Join(reasons, "; ")
	return score, reason
}

// LogSkillActivation records that a skill was auto-loaded for telemetry.
// Failures are best-effort — auto-load must never block the AI loop.
func (s *Store) LogSkillActivation(skillID, teamID, project, promptHash, matchReason string, score float64) {
	id := newID()
	_, _ = s.db.Exec(`
		INSERT INTO skill_activations (id, skill_id, team_id, project, prompt_hash, match_score, match_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, skillID, teamID, project, promptHash, score, matchReason)
}
