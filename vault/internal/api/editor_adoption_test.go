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
