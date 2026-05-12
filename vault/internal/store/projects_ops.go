package store

import (
	"fmt"

	"github.com/alcandev/korva/vault/internal/detect"
)

// Phase 4 — project hygiene operations.
//
// These helpers back the `korva projects` CLI and the matching Observatory
// admin endpoints. They lean on the existing ListProjects + MergeProjects to
// stay consistent with the rest of the store layer; new responsibilities are
// scoped to *describing* the state (suggestions, empty projects) and
// *cleaning up the orphans* a long-running install accumulates.

// ConsolidationProposal groups project names that fold to the same normalized
// form. The proposed canonical is the variant with the most observations —
// callers can override it before invoking MergeProjects.
type ConsolidationProposal struct {
	Canonical string         `json:"canonical"`
	Variants  []ProjectStats `json:"variants"`
}

// SuggestConsolidations groups ListProjects entries by NormalizeProjectName
// and returns groups whose variant count is ≥ 2. The order inside each group
// puts the canonical first; the remaining entries are the merge candidates.
func (s *Store) SuggestConsolidations() ([]ConsolidationProposal, error) {
	projects, err := s.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}

	buckets := make(map[string][]ProjectStats)
	for _, p := range projects {
		key := detect.NormalizeProjectName(p.Name)
		if key == "" {
			continue
		}
		buckets[key] = append(buckets[key], p)
	}

	out := make([]ConsolidationProposal, 0, len(buckets))
	for _, group := range buckets {
		if len(group) < 2 {
			continue
		}
		// Pick the variant with the most observations as canonical. Ties keep
		// the order returned by ListProjects (observation count desc), which
		// also breaks ties deterministically by row order.
		bestIdx := 0
		for i := 1; i < len(group); i++ {
			if group[i].ObservationCount > group[bestIdx].ObservationCount {
				bestIdx = i
			}
		}
		canonical := group[bestIdx].Name
		// Move canonical to position 0 so the response is "{ canonical, [variants...] }".
		group[0], group[bestIdx] = group[bestIdx], group[0]
		out = append(out, ConsolidationProposal{Canonical: canonical, Variants: group})
	}
	return out, nil
}

// EmptyProject summarizes a project that has zero observations but still owns
// sessions or prompts. These are the rows `korva projects prune` would clean.
type EmptyProject struct {
	Project      string `json:"project"`
	SessionCount int    `json:"session_count"`
	PromptCount  int    `json:"prompt_count"`
}

// PruneOptions controls PruneEmptyProjects's behavior.
type PruneOptions struct {
	// Apply executes the cleanup. When false the call is a dry-run that only
	// counts what would be removed.
	Apply bool
}

// PruneResult reports the outcome of a PruneEmptyProjects call.
type PruneResult struct {
	Empty           []EmptyProject `json:"empty"`
	SessionsRemoved int64          `json:"sessions_removed"`
	PromptsRemoved  int64          `json:"prompts_removed"`
	DryRun          bool           `json:"dry_run"`
}

// PruneEmptyProjects finds projects that own sessions or prompts but no
// observations, optionally deletes those orphan rows, and returns the
// summary. Useful after migrating, renaming, or long-running installs where
// abandoned MCP sessions accumulate without ever saving a single observation.
func (s *Store) PruneEmptyProjects(opts PruneOptions) (*PruneResult, error) {
	// Project names that appear in sessions but have no observations. The
	// `prompts` table has no project column today, so the empty-project
	// signal lives entirely in sessions; the PromptCount field is reserved
	// for a future migration that adds project scoping to prompts.
	rows, err := s.db.Query(`
		SELECT s.project,
		       COUNT(*) AS session_count,
		       0       AS prompt_count
		  FROM sessions s
		 WHERE s.project <> ''
		   AND NOT EXISTS (SELECT 1 FROM observations o WHERE o.project = s.project)
		 GROUP BY s.project
		 ORDER BY s.project`,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning empty projects: %w", err)
	}
	defer rows.Close()

	result := &PruneResult{DryRun: !opts.Apply}
	for rows.Next() {
		var ep EmptyProject
		if err := rows.Scan(&ep.Project, &ep.SessionCount, &ep.PromptCount); err != nil {
			return nil, err
		}
		result.Empty = append(result.Empty, ep)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if !opts.Apply {
		return result, nil
	}

	// Apply mode: delete sessions/prompts owned by empty projects.
	for _, ep := range result.Empty {
		res, err := s.db.Exec(`DELETE FROM sessions WHERE project = ?`, ep.Project)
		if err != nil {
			return nil, fmt.Errorf("pruning sessions for %q: %w", ep.Project, err)
		}
		n, _ := res.RowsAffected()
		result.SessionsRemoved += n
		// prompts has no project column today; leave the prompt prune
		// hook in place for when it does (forward-compatible plan).
	}
	return result, nil
}
