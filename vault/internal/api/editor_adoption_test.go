package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// seedInteractionsForAdoption inserts N interactions per editor over
// the recent past so the adoption endpoint has something to count.
func seedInteractionsForAdoption(t *testing.T, s *store.Store) {
	t.Helper()
	now := time.Now().UTC()
	seed := []struct {
		editor string
		count  int
	}{
		{"cursor", 5},
		{"claude", 3},
		{"", 2}, // anonymous
	}
	for _, sd := range seed {
		for i := 0; i < sd.count; i++ {
			in := store.Interaction{
				Project:       "p",
				Agent:         "a",
				Editor:        sd.editor,
				PromptExcerpt: "x",
				CreatedAt:     now.Add(-time.Duration(i) * time.Minute),
			}
			if _, err := s.SaveInteraction(in); err != nil {
				t.Fatalf("seed: %v", err)
			}
		}
	}
}

func TestAdminEditorAdoption_AggregatesAndOrders(t *testing.T) {
	s := newAPITestStore(t)
	seedInteractionsForAdoption(t, s)
	h := adminEditorAdoption(s)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/editor/adoption", nil)
	h(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload editorAdoptionPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.WindowDays != 7 {
		t.Errorf("default window = %d, want 7", payload.WindowDays)
	}
	if payload.Total != 10 {
		t.Errorf("total = %d, want 10", payload.Total)
	}
	if len(payload.Rows) != 3 {
		t.Fatalf("rows = %d, want 3 (cursor, claude, empty)", len(payload.Rows))
	}
	// Ordered by count DESC: cursor (5) → claude (3) → "" (2).
	if payload.Rows[0].Editor != "cursor" || payload.Rows[0].Count != 5 {
		t.Errorf("rows[0] = %+v", payload.Rows[0])
	}
	if payload.Rows[1].Editor != "claude" || payload.Rows[1].Count != 3 {
		t.Errorf("rows[1] = %+v", payload.Rows[1])
	}
	if payload.Rows[2].Editor != "" || payload.Rows[2].Count != 2 {
		t.Errorf("rows[2] = %+v", payload.Rows[2])
	}
}

func TestAdminEditorAdoption_CapsWindowDays(t *testing.T) {
	s := newAPITestStore(t)
	h := adminEditorAdoption(s)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/editor/adoption?days=9999", nil)
	h(rec, r)

	var payload editorAdoptionPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload.WindowDays != adoptionMaxDays {
		t.Errorf("window = %d, want capped at %d", payload.WindowDays, adoptionMaxDays)
	}
}

func TestAdminEditorAdoption_AcceptsCustomWindow(t *testing.T) {
	s := newAPITestStore(t)
	h := adminEditorAdoption(s)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/editor/adoption?days=30", nil)
	h(rec, r)

	var payload editorAdoptionPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload.WindowDays != 30 {
		t.Errorf("window = %d, want 30", payload.WindowDays)
	}
}

// ─────────────────────── Phase 19.D — unified HTTP + MCP adoption ──────────

// seedMCPCalls inserts N mcp_calls rows per editor for the
// unification tests.
func seedMCPCalls(t *testing.T, s *store.Store, editor string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := s.LogCall(store.CallLog{
			Tool:   "vault_status",
			Editor: editor,
			Status: "ok",
		}); err != nil {
			t.Fatalf("seed mcp_calls: %v", err)
		}
	}
}

func TestAdminEditorAdoption_UnionsHTTPAndMCPChannels(t *testing.T) {
	s := newAPITestStore(t)
	// HTTP: 5 cursor + 2 anonymous.
	seedInteractionsForAdoption(t, s)
	// MCP: 10 cursor (top up) + 4 codex (new editor not seen on HTTP).
	seedMCPCalls(t, s, "cursor", 10)
	seedMCPCalls(t, s, "codex", 4)
	// One MCP call from a "claude" client to verify the alias path
	// still aggregates correctly with the HTTP claude entries.
	seedMCPCalls(t, s, "claude", 2)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/editor/adoption", nil)
	adminEditorAdoption(s)(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var payload editorAdoptionPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}

	// Totals: cursor 5+10=15, claude 3+2=5, codex 4, "" 2 → grand 26.
	if payload.Total != 26 {
		t.Errorf("total = %d, want 26", payload.Total)
	}
	want := map[string]struct {
		count, http, mcp int
	}{
		"cursor": {15, 5, 10},
		"claude": {5, 3, 2},
		"codex":  {4, 0, 4},
		"":       {2, 2, 0},
	}
	got := make(map[string]store.EditorAdoptionRow, len(payload.Rows))
	for _, r := range payload.Rows {
		got[r.Editor] = r
	}
	for editor, w := range want {
		r, ok := got[editor]
		if !ok {
			t.Errorf("missing row for %q", editor)
			continue
		}
		if r.Count != w.count {
			t.Errorf("%q total = %d, want %d", editor, r.Count, w.count)
		}
		if r.ByChannel.HTTP != w.http {
			t.Errorf("%q http = %d, want %d", editor, r.ByChannel.HTTP, w.http)
		}
		if r.ByChannel.MCP != w.mcp {
			t.Errorf("%q mcp = %d, want %d", editor, r.ByChannel.MCP, w.mcp)
		}
	}
}

func TestAdminEditorAdoption_MCPOnlyDeploymentStillReports(t *testing.T) {
	// A vault with no HTTP wrappers (e.g. an editor that only uses
	// MCP stdio) must still surface adoption — Phase 18.C alone
	// missed this case.
	s := newAPITestStore(t)
	seedMCPCalls(t, s, "codex", 7)
	seedMCPCalls(t, s, "aider", 3)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/editor/adoption", nil)
	adminEditorAdoption(s)(rec, r)

	var payload editorAdoptionPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload.Total != 10 {
		t.Errorf("total = %d, want 10", payload.Total)
	}
	if len(payload.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(payload.Rows))
	}
	// Ordered by count desc.
	if payload.Rows[0].Editor != "codex" || payload.Rows[0].Count != 7 {
		t.Errorf("rows[0] = %+v", payload.Rows[0])
	}
	if payload.Rows[1].Editor != "aider" || payload.Rows[1].Count != 3 {
		t.Errorf("rows[1] = %+v", payload.Rows[1])
	}
}

func TestAdminEditorAdoption_FiltersToWindow(t *testing.T) {
	s := newAPITestStore(t)

	// One recent + one old. Window of 1 day should exclude the old one.
	if _, err := s.SaveInteraction(store.Interaction{
		Project: "p", Agent: "a", Editor: "claude", PromptExcerpt: "recent",
		CreatedAt: time.Now().UTC().Add(-1 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInteraction(store.Interaction{
		Project: "p", Agent: "a", Editor: "cursor", PromptExcerpt: "old",
		CreatedAt: time.Now().UTC().Add(-30 * 24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	h := adminEditorAdoption(s)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/editor/adoption?days=1", nil)
	h(rec, r)

	var payload editorAdoptionPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload.Total != 1 {
		t.Errorf("total = %d, want 1 (only the recent one)", payload.Total)
	}
	if len(payload.Rows) != 1 || payload.Rows[0].Editor != "claude" {
		t.Errorf("rows wrong: %+v", payload.Rows)
	}
}
