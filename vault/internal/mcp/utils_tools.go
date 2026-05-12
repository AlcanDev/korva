package mcp

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alcandev/korva/vault/internal/detect"
	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 3 — utility MCP tools that broaden how agents interact with the Vault
// without needing to round-trip through the operator surfaces:
//
//   vault_current_project    — answers "which project am I in?" with rich
//                              context (auto-detection source, available
//                              alternatives, similar project warnings).
//   vault_suggest_topic_key  — converts a freeform title into a stable upsert
//                              key plus a list of near-existing keys in the
//                              same project, so callers consolidate evolving
//                              knowledge instead of accumulating duplicates.
//   vault_capture_passive    — parses common markdown section headings
//                              ("## Key Learnings:", "## Decisions:", etc.)
//                              from a freeform block of text and saves each
//                              bullet as its own observation.

// ── vault_current_project ────────────────────────────────────────────────────

// toolCurrentProject responds with an enriched envelope that lets the agent
// reason about *how* the project name was reached, not just what it is.
// Errors only on truly impossible failure modes — the contract is to always
// return something so the agent never has to special-case an unknown CWD.
func (s *Server) toolCurrentProject(args map[string]any) (any, error) {
	workingDir := stringArg(args, "working_dir")
	if workingDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			workingDir = cwd
		}
	}
	dr := detect.Project(workingDir)

	resp := map[string]any{
		"project":            dr.Project,
		"project_source":     dr.Source,
		"cwd":                workingDir,
		"available_projects": dr.AvailableProjects,
	}
	if dr.Warning != "" {
		resp["warning"] = dr.Warning
	}

	// Compare against known projects in the vault to surface "your detected name
	// has a near-duplicate already in storage" warnings. This is the same
	// signal the operator-side conflict workflow exposes for observations.
	if dr.Project != "" {
		if projects, err := s.store.ListProjects(); err == nil {
			names := make([]string, 0, len(projects))
			for _, p := range projects {
				names = append(names, p.Name)
			}
			if similar := detect.FindSimilarProjects(dr.Project, names); len(similar) > 0 {
				resp["similar_projects"] = similar
				resp["similar_tip"] = "Project names with the same normalized form already exist. Consider passing 'project' explicitly or running 'korva projects consolidate' to merge them."
			}
		}
	}
	return resp, nil
}

// ── vault_suggest_topic_key ──────────────────────────────────────────────────

// topicKeyTitleSlug squeezes a title into a kebab-case slug suitable for use
// as a topic_key. Rules:
//   - lowercase
//   - non-alnum runs collapse to a single "-"
//   - trim leading/trailing "-"
//   - cap to 60 chars (keeps DB indexes happy)
func topicKeyTitleSlug(title string) string {
	if title == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(title))
	prevSep := true
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevSep = false
			continue
		}
		if !prevSep {
			b.WriteByte('-')
			prevSep = true
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > 60 {
		out = out[:60]
		out = strings.TrimRight(out, "-")
	}
	return out
}

// toolSuggestTopicKey proposes a stable topic_key from a title (plus an
// optional type prefix) and reports near-matches against existing keys in
// the same project. The caller pastes the returned key into vault_save's
// `topic_key` argument to get upsert semantics.
func (s *Server) toolSuggestTopicKey(args map[string]any) (any, error) {
	title := strings.TrimSpace(stringArg(args, "title"))
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	slug := topicKeyTitleSlug(title)
	if slug == "" {
		return nil, fmt.Errorf("could not derive a topic_key from title %q", title)
	}

	obsType := stringArg(args, "type")
	if obsType != "" {
		slug = topicKeyTitleSlug(obsType) + "/" + slug
	}

	project := stringArg(args, "project")
	resp := map[string]any{
		"topic_key": slug,
		"project":   project,
		"type":      obsType,
	}

	// Best-effort similar-key lookup. We compare normalised forms so
	// "architecture/auth-model" and "architecture/auth_model" collide.
	if project != "" {
		similar, err := s.findSimilarTopicKeys(project, slug)
		if err == nil && len(similar) > 0 {
			resp["similar_existing_keys"] = similar
			resp["similar_tip"] = "Topic keys with the same normalized form already exist. Consider reusing one to upsert into the existing row instead of branching the knowledge graph."
		}
	}
	return resp, nil
}

// findSimilarTopicKeys queries existing topic_keys in the project and returns
// those whose normalised form matches the candidate slug. Read-only.
func (s *Server) findSimilarTopicKeys(project, slug string) ([]string, error) {
	rows, err := s.store.DB().Query(
		`SELECT DISTINCT topic_key FROM observations
		  WHERE project = ? AND topic_key IS NOT NULL AND topic_key != ''`,
		project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	want := detect.NormalizeProjectName(slug)
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		if k == slug {
			continue
		}
		if detect.NormalizeProjectName(k) == want {
			out = append(out, k)
		}
	}
	return out, rows.Err()
}

// ── vault_capture_passive ────────────────────────────────────────────────────

// captureSectionHeading matches markdown second-level headings like
// "## Key Learnings:" or "## Decisions" (the trailing colon is optional).
// We anchor to '##' specifically because '#' alone matches H1 titles that are
// usually the document header, not a section we want to mine.
var captureSectionHeading = regexp.MustCompile(`(?m)^##\s+([^\n]+?)\s*:?\s*$`)

// captureBullet matches a single bullet item under a section. Accepts the
// common forms: "- foo", "* foo", "1. foo".
var captureBullet = regexp.MustCompile(`(?m)^\s*(?:[-*]\s+|\d+\.\s+)(.+?)\s*$`)

// captureSectionTypeMap maps recognized heading lowercased to an observation
// type. Headings outside this map fall back to default_type (or "learning").
var captureSectionTypeMap = map[string]string{
	"key learnings":   "learning",
	"learnings":       "learning",
	"lessons learned": "learning",
	"discoveries":     "discovery",
	"decisions":       "decision",
	"bugfixes":        "bugfix",
	"bug fixes":       "bugfix",
	"fixes":           "bugfix",
	"patterns":        "pattern",
	"antipatterns":    "antipattern",
	"anti-patterns":   "antipattern",
	"refactors":       "refactor",
	"refactoring":     "refactor",
	"incidents":       "incident",
	"features":        "feature",
}

// toolCapturePassive parses `text` for capture-friendly markdown sections and
// saves one observation per bullet found. The agent uses this to absorb
// retrospective notes, tool outputs, or chat transcripts without crafting
// individual vault_save calls.
func (s *Server) toolCapturePassive(args map[string]any) (any, error) {
	text := stringArg(args, "text")
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("text is required")
	}
	project := strings.TrimSpace(stringArg(args, "project"))
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	defaultType := stringArg(args, "default_type")
	if defaultType == "" {
		defaultType = "learning"
	}
	author := stringArg(args, "author")

	saved := []string{}
	skipped := []string{}

	sections := splitIntoSections(text)
	for _, sec := range sections {
		obsType := captureSectionTypeMap[strings.ToLower(strings.TrimSpace(sec.heading))]
		if obsType == "" {
			obsType = defaultType
		}
		for _, bullet := range captureBullet.FindAllStringSubmatch(sec.body, -1) {
			line := strings.TrimSpace(bullet[1])
			if line == "" {
				continue
			}
			id, err := s.store.Save(store.Observation{
				Project: project,
				Type:    store.ObservationType(obsType),
				Title:   truncatePassiveTitle(line, 100),
				Content: line,
				Author:  author,
				Tags:    []string{"captured", "passive"},
			})
			if err != nil {
				skipped = append(skipped, fmt.Sprintf("%s: %v", line, err))
				continue
			}
			saved = append(saved, id)
		}
	}

	return map[string]any{
		"saved":         len(saved),
		"skipped":       len(skipped),
		"ids":           saved,
		"skipped_lines": skipped,
		"section_count": len(sections),
	}, nil
}

// capturedSection is a parsed markdown chunk with its heading and body.
type capturedSection struct {
	heading string
	body    string
}

// splitIntoSections walks the headings the regex finds in `text` and returns
// the body between each heading and the next. A capture that finds no headings
// returns a single section with heading="" and the whole text as body so the
// caller still gets a chance to extract bullets.
func splitIntoSections(text string) []capturedSection {
	idx := captureSectionHeading.FindAllStringSubmatchIndex(text, -1)
	if len(idx) == 0 {
		return []capturedSection{{heading: "", body: text}}
	}
	out := make([]capturedSection, 0, len(idx))
	for i, m := range idx {
		// m = [matchStart, matchEnd, group1Start, group1End]
		headingStart, headingEnd := m[0], m[1]
		nameStart, nameEnd := m[2], m[3]
		bodyEnd := len(text)
		if i+1 < len(idx) {
			bodyEnd = idx[i+1][0]
		}
		out = append(out, capturedSection{
			heading: text[nameStart:nameEnd],
			body:    text[headingEnd:bodyEnd],
		})
		_ = headingStart
	}
	return out
}

// truncatePassiveTitle clips a bullet line at a reasonable title length while
// preserving full sentence-ish-ness by cutting on the last space.
func truncatePassiveTitle(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if idx := strings.LastIndexByte(cut, ' '); idx > max/2 {
		cut = cut[:idx]
	}
	return cut + "…"
}
