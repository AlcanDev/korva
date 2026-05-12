package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Phase 5 — Obsidian-flavored markdown export.
//
// ExportObsidian walks the local vault and writes one markdown file per
// observation under a directory layout Obsidian understands:
//
//	<out>/
//	  README.md                   ← root index of every project
//	  <project>/
//	    _index.md                 ← per-project index, grouped by type
//	    <type>/                   ← decisions, patterns, bugfixes, …
//	      <topic-key or short id>.md
//
// Each note carries a YAML frontmatter block (id, project, type, title,
// topic_key, tags, author, created_at) followed by the observation content
// and — at the bottom — a Related section with [[wikilinks]] to every
// observation reachable via the relations table.
//
// The renderer is deliberately split from the orchestrator so tests can pin
// the markdown shape without ever touching the filesystem.

// ObsidianExportOptions narrows the export by project and/or type. Both are
// optional; the zero value exports everything.
type ObsidianExportOptions struct {
	Project string // "" = all
	Type    string // "" = all
}

// ObsidianExportResult reports what the export wrote.
type ObsidianExportResult struct {
	OutDir       string         `json:"out_dir"`
	FileCount    int            `json:"file_count"`
	ProjectCount int            `json:"project_count"`
	ByProject    map[string]int `json:"by_project"`
	ByType       map[string]int `json:"by_type"`
	GeneratedAt  time.Time      `json:"generated_at"`
}

// ExportObsidian runs the full export against `out`. It creates the directory
// tree if it doesn't exist and is safe to re-run — every file is rewritten
// from the current store state.
func (s *Store) ExportObsidian(out string, opts ObsidianExportOptions) (*ObsidianExportResult, error) {
	if out == "" {
		return nil, fmt.Errorf("out directory is required")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, fmt.Errorf("create out dir: %w", err)
	}

	// Page through every observation. We use Search("") with a generous limit
	// and offset paging so the export scales beyond the default 20-row cap
	// without loading the entire DB at once.
	const pageSize = 500
	var all []Observation
	for offset := 0; ; offset += pageSize {
		filters := SearchFilters{Limit: pageSize, Offset: offset, Project: opts.Project}
		if opts.Type != "" {
			filters.Type = ObservationType(opts.Type)
		}
		page, err := s.Search("", filters)
		if err != nil {
			return nil, fmt.Errorf("list observations: %w", err)
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		if len(page) < pageSize {
			break
		}
	}

	// Build the id → topic-key lookup so wikilinks resolve to the same
	// filename the renderer will produce. Observations without a topic_key
	// fall back to their short id.
	idToSlug := make(map[string]string, len(all))
	for _, o := range all {
		idToSlug[o.ID] = noteSlug(o)
	}

	result := &ObsidianExportResult{
		OutDir:      out,
		ByProject:   map[string]int{},
		ByType:      map[string]int{},
		GeneratedAt: time.Now().UTC(),
	}

	// Bucket observations per project so we can emit per-project indexes.
	projectBuckets := map[string]*projectBucket{}

	for _, o := range all {
		rels, err := s.GetRelations(o.ID)
		if err != nil {
			return nil, fmt.Errorf("relations for %s: %w", o.ID, err)
		}
		content := RenderObsidianNote(o, rels, idToSlug)

		projDir := filepath.Join(out, sanitizeSegment(o.Project))
		typeDir := filepath.Join(projDir, sanitizeSegment(string(o.Type)))
		if err := os.MkdirAll(typeDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", typeDir, err)
		}
		notePath := filepath.Join(typeDir, idToSlug[o.ID]+".md")
		if err := os.WriteFile(notePath, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", notePath, err)
		}

		b, ok := projectBuckets[o.Project]
		if !ok {
			b = &projectBucket{byType: map[string][]Observation{}, typeCounts: map[string]int{}}
			projectBuckets[o.Project] = b
		}
		b.obs = append(b.obs, o)
		b.byType[string(o.Type)] = append(b.byType[string(o.Type)], o)
		b.typeCounts[string(o.Type)]++

		result.FileCount++
		result.ByProject[o.Project]++
		result.ByType[string(o.Type)]++
	}

	// Per-project index pages.
	for project, b := range projectBuckets {
		projDir := filepath.Join(out, sanitizeSegment(project))
		idx := renderProjectIndex(project, b.obs, b.byType, idToSlug)
		if err := os.WriteFile(filepath.Join(projDir, "_index.md"), []byte(idx), 0o644); err != nil {
			return nil, fmt.Errorf("write project index for %s: %w", project, err)
		}
	}
	result.ProjectCount = len(projectBuckets)

	// Root README.
	root := renderRootIndex(result, projectBuckets)
	if err := os.WriteFile(filepath.Join(out, "README.md"), []byte(root), 0o644); err != nil {
		return nil, fmt.Errorf("write root README: %w", err)
	}
	return result, nil
}

// RenderObsidianNote produces the markdown body for a single observation.
// Pure function — easy to test in isolation.
func RenderObsidianNote(o Observation, rels *ObservationRelations, idToSlug map[string]string) string {
	var b strings.Builder

	// YAML frontmatter — quoted scalars on every value so colons in titles
	// don't break the parser.
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %q\n", o.ID)
	fmt.Fprintf(&b, "project: %q\n", o.Project)
	fmt.Fprintf(&b, "type: %q\n", string(o.Type))
	fmt.Fprintf(&b, "title: %q\n", o.Title)
	if o.TopicKey != "" {
		fmt.Fprintf(&b, "topic_key: %q\n", o.TopicKey)
	}
	if o.Author != "" {
		fmt.Fprintf(&b, "author: %q\n", o.Author)
	}
	if len(o.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, t := range o.Tags {
			fmt.Fprintf(&b, "  - %q\n", t)
		}
	}
	if !o.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "created_at: %q\n", o.CreatedAt.UTC().Format(time.RFC3339))
	}
	b.WriteString("---\n\n")

	// Title as the H1 so Obsidian's link preview shows the same string as the
	// frontmatter field. Identical content under the frontmatter would be
	// rendered as plain text otherwise.
	fmt.Fprintf(&b, "# %s\n\n", o.Title)

	// Body.
	body := strings.TrimSpace(o.Content)
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n")
	}

	// Related observations — emit only links to notes we actually wrote.
	if rels != nil && (len(rels.AsSource) > 0 || len(rels.AsTarget) > 0) {
		b.WriteString("\n## Related\n\n")
		appendRelations(&b, "source", rels.AsSource, o.ID, idToSlug)
		appendRelations(&b, "target", rels.AsTarget, o.ID, idToSlug)
	}
	return b.String()
}

func appendRelations(b *strings.Builder, side string, rels []Relation, selfID string, idToSlug map[string]string) {
	for _, r := range rels {
		var otherID string
		var verb string
		if side == "source" {
			otherID = r.TargetID
			verb = string(r.Relation)
		} else {
			otherID = r.SourceID
			verb = string(r.Relation) + " (incoming)"
		}
		if otherID == selfID {
			continue
		}
		slug, ok := idToSlug[otherID]
		if !ok {
			// Pointer to a row outside the current export scope — surface as
			// a plain code reference instead of a dead link.
			fmt.Fprintf(b, "- **%s** → `%s` (out of export scope)\n", verb, otherID)
			continue
		}
		fmt.Fprintf(b, "- **%s** → [[%s]]\n", verb, slug)
	}
}

// noteSlug picks a filename for an observation. Topic-key wins because it's
// stable across re-saves; otherwise we use the last 8 chars of the ULID for
// brevity — enough to be unique in practice without an unwieldy 26-char file.
func noteSlug(o Observation) string {
	if o.TopicKey != "" {
		s := sanitizeSegment(o.TopicKey)
		if s != "" {
			return s
		}
	}
	if len(o.ID) >= 8 {
		return strings.ToLower(o.ID[len(o.ID)-8:])
	}
	return strings.ToLower(o.ID)
}

// sanitizeSegment turns a project / type / topic-key into a safe path segment
// (forward-slash separators in topic keys would otherwise create directories).
var unsafeSegmentChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = unsafeSegmentChars.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// renderProjectIndex builds the per-project _index.md.
func renderProjectIndex(project string, obs []Observation, byType map[string][]Observation, idToSlug map[string]string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "project: %q\n", project)
	fmt.Fprintf(&b, "type: %q\n", "index")
	fmt.Fprintf(&b, "generated_at: %q\n", time.Now().UTC().Format(time.RFC3339))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", project)
	fmt.Fprintf(&b, "%d observation(s) across %d type(s).\n\n", len(obs), len(byType))

	// Sort types alphabetically for deterministic output.
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, t := range types {
		fmt.Fprintf(&b, "## %s (%d)\n\n", t, len(byType[t]))
		// Newest-first inside each section.
		group := append([]Observation{}, byType[t]...)
		sort.Slice(group, func(i, j int) bool { return group[i].CreatedAt.After(group[j].CreatedAt) })
		for _, o := range group {
			slug := idToSlug[o.ID]
			fmt.Fprintf(&b, "- [[%s|%s]]\n", slug, o.Title)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// projectBucket gathers per-project observations during the export pass so
// we can emit one _index.md per project at the end.
type projectBucket struct {
	obs        []Observation
	byType     map[string][]Observation
	typeCounts map[string]int
}

// renderRootIndex builds <out>/README.md — a top-level table of projects.
func renderRootIndex(r *ObsidianExportResult, buckets map[string]*projectBucket) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "type: %q\n", "korva-export-root")
	fmt.Fprintf(&b, "generated_at: %q\n", r.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "file_count: %d\n", r.FileCount)
	fmt.Fprintf(&b, "project_count: %d\n", r.ProjectCount)
	b.WriteString("---\n\n")
	b.WriteString("# Korva Vault — Obsidian export\n\n")
	fmt.Fprintf(&b, "Generated %s. %d observations across %d project(s).\n\n",
		r.GeneratedAt.Format(time.RFC3339), r.FileCount, r.ProjectCount)

	// Project list sorted alphabetically.
	projects := make([]string, 0, len(buckets))
	for p := range buckets {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	b.WriteString("## Projects\n\n")
	for _, p := range projects {
		dir := sanitizeSegment(p)
		fmt.Fprintf(&b, "- [[%s/_index|%s]] — %d observation(s)\n", dir, p, len(buckets[p].obs))
	}
	return b.String()
}
