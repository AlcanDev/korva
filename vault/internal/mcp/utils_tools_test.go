package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

// ── vault_current_project ───────────────────────────────────────────────────

func TestToolCurrentProject_ReturnsDetectionEnvelope(t *testing.T) {
	srv := newSharedServer(t)
	tmp := t.TempDir()

	resp := dispatchOn(t, srv, "vault_current_project", map[string]any{
		"working_dir": tmp,
	})
	if resp["project"] == nil || resp["project"] == "" {
		t.Errorf("expected non-empty project, got %+v", resp)
	}
	if resp["project_source"] == "" {
		t.Errorf("expected project_source, got %+v", resp)
	}
	if resp["cwd"] != tmp {
		t.Errorf("cwd echoed wrong: %v", resp["cwd"])
	}
}

func TestToolCurrentProject_SurfacesSimilarProjects(t *testing.T) {
	srv := newSharedServer(t)
	// Seed an observation under a name that normalizes to the same form as
	// the one the .korva/config.json will request.
	if _, err := srv.store.Save(store.Observation{
		Project: "my-project", Type: store.TypeDecision, Title: "seed", Content: "x",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tmp := t.TempDir()
	mkConfig(t, tmp, "my_project")

	resp := dispatchOn(t, srv, "vault_current_project", map[string]any{
		"working_dir": tmp,
	})
	similar, _ := resp["similar_projects"].([]string)
	if len(similar) == 0 || similar[0] != "my-project" {
		t.Errorf("expected similar=[my-project], got %v", similar)
	}
	if resp["similar_tip"] == "" {
		t.Error("expected a similar_tip explanation")
	}
}

// ── vault_suggest_topic_key ─────────────────────────────────────────────────

func TestToolSuggestTopicKey_DerivesSlug(t *testing.T) {
	srv := newSharedServer(t)
	resp := dispatchOn(t, srv, "vault_suggest_topic_key", map[string]any{
		"title": "Use ULID for primary keys",
	})
	if resp["topic_key"] != "use-ulid-for-primary-keys" {
		t.Errorf("topic_key = %q, want use-ulid-for-primary-keys", resp["topic_key"])
	}
}

func TestToolSuggestTopicKey_PrefixesType(t *testing.T) {
	srv := newSharedServer(t)
	resp := dispatchOn(t, srv, "vault_suggest_topic_key", map[string]any{
		"title": "Adopt ULID",
		"type":  "decision",
	})
	if resp["topic_key"] != "decision/adopt-ulid" {
		t.Errorf("topic_key = %q, want decision/adopt-ulid", resp["topic_key"])
	}
}

func TestToolSuggestTopicKey_SurfacesSimilarKeys(t *testing.T) {
	srv := newSharedServer(t)
	// Pre-seed a row with a topic_key that normalizes the same way as the
	// suggestion ("adopt-ulid") but spelled differently ("adopt_ulid").
	if _, err := srv.store.Save(store.Observation{
		Project: "korva", Type: store.TypeDecision,
		Title: "Old title", Content: "x", TopicKey: "adopt_ulid",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := dispatchOn(t, srv, "vault_suggest_topic_key", map[string]any{
		"title":   "Adopt ULID",
		"project": "korva",
	})
	similar, _ := resp["similar_existing_keys"].([]string)
	if len(similar) == 0 || similar[0] != "adopt_ulid" {
		t.Errorf("expected similar=[adopt_ulid], got %v", similar)
	}
}

func TestToolSuggestTopicKey_RejectsEmptyTitle(t *testing.T) {
	srv := newSharedServer(t)
	if _, err := srv.dispatchInner("vault_suggest_topic_key", map[string]any{}); err == nil {
		t.Error("expected error for empty title")
	}
}

// ── vault_capture_passive ───────────────────────────────────────────────────

func TestToolCapturePassive_ExtractsBulletsBySection(t *testing.T) {
	srv := newSharedServer(t)
	text := `# Retrospective

Some preamble that should be ignored.

## Key Learnings:
- ULIDs are sortable and URL-safe
- Migrations should always be append-only

## Decisions
- Adopt ULID for primary keys
- Use SQLite WAL mode

## Bugfixes
- Fix race in cloud_outbox cleanup

That's it.`

	resp := dispatchOn(t, srv, "vault_capture_passive", map[string]any{
		"text":    text,
		"project": "korva",
	})
	saved, _ := resp["saved"].(int)
	if saved != 5 {
		t.Errorf("saved = %d, want 5", saved)
	}
	ids, _ := resp["ids"].([]string)
	if len(ids) != 5 {
		t.Errorf("ids len = %d, want 5", len(ids))
	}

	// Each section maps to its observation type.
	got, _ := srv.store.Search("", store.SearchFilters{Project: "korva", Limit: 50})
	if len(got) != 5 {
		t.Errorf("expected 5 stored observations, got %d", len(got))
	}
	typeCounts := map[string]int{}
	for _, o := range got {
		typeCounts[string(o.Type)]++
	}
	if typeCounts["learning"] != 2 {
		t.Errorf("expected 2 learnings, got %d", typeCounts["learning"])
	}
	if typeCounts["decision"] != 2 {
		t.Errorf("expected 2 decisions, got %d", typeCounts["decision"])
	}
	if typeCounts["bugfix"] != 1 {
		t.Errorf("expected 1 bugfix, got %d", typeCounts["bugfix"])
	}
}

func TestToolCapturePassive_FallsBackToDefaultType(t *testing.T) {
	srv := newSharedServer(t)
	text := `## Random Section

- some note
- another note`

	resp := dispatchOn(t, srv, "vault_capture_passive", map[string]any{
		"text":         text,
		"project":      "korva",
		"default_type": "context",
	})
	saved, _ := resp["saved"].(int)
	if saved != 2 {
		t.Fatalf("saved = %d, want 2", saved)
	}
	got, _ := srv.store.Search("", store.SearchFilters{Project: "korva", Limit: 50})
	for _, o := range got {
		if string(o.Type) != "context" {
			t.Errorf("type = %q, want context for unrecognized heading", o.Type)
		}
	}
}

func TestToolCapturePassive_RejectsEmptyInput(t *testing.T) {
	srv := newSharedServer(t)
	if _, err := srv.dispatchInner("vault_capture_passive", map[string]any{
		"text": "", "project": "korva",
	}); err == nil {
		t.Error("expected error for empty text")
	}
	if _, err := srv.dispatchInner("vault_capture_passive", map[string]any{
		"text": "## x\n- y", "project": "",
	}); err == nil {
		t.Error("expected error for empty project")
	}
}

// mkConfig drops a `.korva/config.json` under `dir` so detect.Project resolves
// to `project` without involving the real git/HOME state.
func mkConfig(t *testing.T, dir, project string) {
	t.Helper()
	korvaDir := filepath.Join(dir, ".korva")
	if err := os.MkdirAll(korvaDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := filepath.Join(korvaDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"project":"`+project+`"}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
