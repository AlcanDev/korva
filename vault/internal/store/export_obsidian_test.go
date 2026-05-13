package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Phase 5.1 — Obsidian export tests.
//
// The renderer is exercised in isolation (no DB) so we can pin the markdown
// shape exactly; the orchestrator is tested end-to-end against an in-memory
// store to verify directory layout, idempotency, and filtering.

func TestRenderObsidianNote_FrontmatterAndBody(t *testing.T) {
	o := Observation{
		ID:        "01HXZ1234567890ABCDEFGHJKM",
		Project:   "korva",
		Type:      TypeDecision,
		Title:     "Adopt ULID: short, sortable identifiers",
		Content:   "We picked ULIDs over UUIDv4 because they sort lexically.\n",
		Tags:      []string{"storage", "identifiers"},
		Author:    "felipe",
		TopicKey:  "adopt-ulid",
		CreatedAt: time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC),
	}
	got := RenderObsidianNote(o, nil, map[string]string{o.ID: "adopt-ulid"})

	// Frontmatter checks
	wantFrontmatter := []string{
		`id: "01HXZ1234567890ABCDEFGHJKM"`,
		`project: "korva"`,
		`type: "decision"`,
		// Title contains a colon — quoting must survive that.
		`title: "Adopt ULID: short, sortable identifiers"`,
		`topic_key: "adopt-ulid"`,
		`author: "felipe"`,
		`tags:`,
		`  - "storage"`,
		`  - "identifiers"`,
		`created_at: "2026-05-12T14:00:00Z"`,
	}
	for _, want := range wantFrontmatter {
		if !strings.Contains(got, want) {
			t.Errorf("frontmatter missing %q in:\n%s", want, got)
		}
	}

	// Body checks
	if !strings.Contains(got, "# Adopt ULID: short, sortable identifiers\n") {
		t.Errorf("missing H1 title in body:\n%s", got)
	}
	if !strings.Contains(got, "We picked ULIDs over UUIDv4 because they sort lexically.") {
		t.Errorf("missing observation content in body:\n%s", got)
	}
}

func TestRenderObsidianNote_RelatedLinksUseWikilinks(t *testing.T) {
	self := Observation{ID: "self-1", Project: "korva", Type: TypeDecision, Title: "Self"}
	other := Observation{ID: "other-2", Project: "korva", Type: TypePattern, Title: "Other"}
	rels := &ObservationRelations{
		AsSource: []Relation{{ID: "rel-1", SourceID: self.ID, TargetID: other.ID, Relation: RelationSupersedes}},
		AsTarget: []Relation{{ID: "rel-2", SourceID: other.ID, TargetID: self.ID, Relation: RelationRelated}},
	}
	idToSlug := map[string]string{self.ID: "self-slug", other.ID: "other-slug"}

	got := RenderObsidianNote(self, rels, idToSlug)
	if !strings.Contains(got, "## Related") {
		t.Error("Related section missing")
	}
	if !strings.Contains(got, "**supersedes** → [[other-slug]]") {
		t.Errorf("outgoing supersedes link missing in:\n%s", got)
	}
	if !strings.Contains(got, "**related (incoming)** → [[other-slug]]") {
		t.Errorf("incoming related link missing in:\n%s", got)
	}
}

func TestRenderObsidianNote_OutOfScopeTargetShownInline(t *testing.T) {
	self := Observation{ID: "self-1", Project: "korva", Type: TypeDecision, Title: "Self"}
	rels := &ObservationRelations{
		AsSource: []Relation{{ID: "rel-1", SourceID: self.ID, TargetID: "ghost-id", Relation: RelationConflicts}},
	}
	idToSlug := map[string]string{self.ID: "self-slug"} // ghost-id missing

	got := RenderObsidianNote(self, rels, idToSlug)
	if !strings.Contains(got, "`ghost-id` (out of export scope)") {
		t.Errorf("expected out-of-scope marker for ghost-id, got:\n%s", got)
	}
}

func TestNoteSlug_PrefersTopicKeyThenShortID(t *testing.T) {
	tests := []struct {
		name string
		obs  Observation
		want string
	}{
		{"with topic key", Observation{ID: "01HXZABCDE", TopicKey: "adopt-ulid"}, "adopt-ulid"},
		{"topic key with slashes", Observation{ID: "01HXZABCDE", TopicKey: "decision/adopt-ulid"}, "decision-adopt-ulid"},
		{"no topic key", Observation{ID: "01HXZABCDE12345678"}, "12345678"},
		{"short id only", Observation{ID: "ABC"}, "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := noteSlug(tc.obs); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSanitizeSegment_StripsUnsafeChars(t *testing.T) {
	tests := []struct{ in, want string }{
		{"my-project", "my-project"},
		{"My Project!", "My-Project"},
		{"a//b\\c", "a-b-c"},
		{"   ", ""},
	}
	for _, tc := range tests {
		if got := sanitizeSegment(tc.in); got != tc.want {
			t.Errorf("sanitizeSegment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ── end-to-end against a real (in-memory) store ─────────────────────────────

func TestExportObsidian_WritesLayoutAndIndexes(t *testing.T) {
	s := newTestStore(t)
	seed := []struct {
		project  string
		obsType  ObservationType
		title    string
		topicKey string
	}{
		{"korva", TypeDecision, "Adopt ULID", "adopt-ulid"},
		{"korva", TypePattern, "Outbox pattern", "outbox"},
		{"vault-mcp", TypeDecision, "Use stdio transport", "stdio-transport"},
	}
	for _, s2 := range seed {
		if _, err := s.Save(Observation{
			Project: s2.project, Type: s2.obsType, Title: s2.title,
			Content: "Content for " + s2.title, TopicKey: s2.topicKey,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	out := t.TempDir()
	res, err := s.ExportObsidian(out, ObsidianExportOptions{})
	if err != nil {
		t.Fatalf("ExportObsidian: %v", err)
	}
	if res.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", res.FileCount)
	}
	if res.ProjectCount != 2 {
		t.Errorf("ProjectCount = %d, want 2", res.ProjectCount)
	}
	if res.ByProject["korva"] != 2 {
		t.Errorf("ByProject[korva] = %d, want 2", res.ByProject["korva"])
	}

	// Filesystem layout assertions.
	mustExist := []string{
		"README.md",
		"korva/_index.md",
		"korva/decision/adopt-ulid.md",
		"korva/pattern/outbox.md",
		"vault-mcp/_index.md",
		"vault-mcp/decision/stdio-transport.md",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}

	// The root README references both projects.
	root, _ := os.ReadFile(filepath.Join(out, "README.md"))
	if !strings.Contains(string(root), "[[korva/_index|korva]]") {
		t.Errorf("root README missing korva link:\n%s", root)
	}
	if !strings.Contains(string(root), "[[vault-mcp/_index|vault-mcp]]") {
		t.Errorf("root README missing vault-mcp link:\n%s", root)
	}

	// Project index includes type headings + wikilinks.
	korvaIdx, _ := os.ReadFile(filepath.Join(out, "korva", "_index.md"))
	if !strings.Contains(string(korvaIdx), "## decision (1)") {
		t.Errorf("korva index missing decision heading:\n%s", korvaIdx)
	}
	if !strings.Contains(string(korvaIdx), "[[adopt-ulid|Adopt ULID]]") {
		t.Errorf("korva index missing adopt-ulid wikilink:\n%s", korvaIdx)
	}
}

func TestExportObsidian_FilterByProject(t *testing.T) {
	s := newTestStore(t)
	for _, p := range []string{"korva", "vault-mcp", "beacon"} {
		if _, err := s.Save(Observation{
			Project: p, Type: TypeDecision, Title: p + " decision", Content: "x",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	out := t.TempDir()
	res, err := s.ExportObsidian(out, ObsidianExportOptions{Project: "korva"})
	if err != nil {
		t.Fatalf("ExportObsidian: %v", err)
	}
	if res.FileCount != 1 || res.ProjectCount != 1 {
		t.Errorf("filter by project: got files=%d projects=%d, want 1/1", res.FileCount, res.ProjectCount)
	}
	if _, err := os.Stat(filepath.Join(out, "korva")); err != nil {
		t.Error("korva directory should exist")
	}
	if _, err := os.Stat(filepath.Join(out, "vault-mcp")); !os.IsNotExist(err) {
		t.Error("vault-mcp should NOT exist when filtering on korva")
	}
}

func TestExportObsidian_IsIdempotent(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Save(Observation{
		Project: "korva", Type: TypeDecision, Title: "Adopt ULID",
		Content: "v1", TopicKey: "adopt-ulid",
	}); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	if _, err := s.ExportObsidian(out, ObsidianExportOptions{}); err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join(out, "korva", "decision", "adopt-ulid.md")
	first, _ := os.ReadFile(notePath)

	// Re-run unchanged — the same content should land in the same file.
	if _, err := s.ExportObsidian(out, ObsidianExportOptions{}); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(notePath)
	// `generated_at` in the root index changes, but individual notes don't
	// embed a timestamp — they should be byte-identical.
	if string(first) != string(second) {
		t.Errorf("note content drifted across runs:\n--- first ---\n%s\n--- second ---\n%s",
			first, second)
	}
}

func TestExportObsidian_RejectsEmptyOutDir(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ExportObsidian("", ObsidianExportOptions{}); err == nil {
		t.Error("expected error for empty out dir")
	}
}
