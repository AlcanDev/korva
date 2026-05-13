package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestAdminSessionReplay_NotFound(t *testing.T) {
	s := newAPITestStore(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/sessions/missing/replay", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	adminSessionReplay(s)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAdminSessionReplay_RequiresID(t *testing.T) {
	s := newAPITestStore(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/sessions//replay", nil)
	// No PathValue set — handler must reject.
	rec := httptest.NewRecorder()
	adminSessionReplay(s)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminSessionReplay_ReturnsTimeline(t *testing.T) {
	s := newAPITestStore(t)
	sessID, err := s.SessionStart("korva", "platform", "CL", "agent", "explore ULID")
	if err != nil {
		t.Fatalf("session start: %v", err)
	}
	// Save 2 observations bound to the session.
	if _, err := s.Save(store.Observation{
		Project: "korva", Type: store.TypeDecision,
		Title: "Use ULID", Content: "Sortable + URL-safe.", SessionID: sessID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(store.Observation{
		Project: "korva", Type: store.TypePattern,
		Title: "Outbox", Content: "Decouples cloud failures.", SessionID: sessID,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/sessions/"+sessID+"/replay", nil)
	req.SetPathValue("id", sessID)
	rec := httptest.NewRecorder()
	adminSessionReplay(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp ReplayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.SessionID != sessID {
		t.Errorf("SessionID = %q, want %q", resp.SessionID, sessID)
	}
	if resp.Project != "korva" {
		t.Errorf("Project = %q, want korva", resp.Project)
	}
	// 1 session_start + 2 observations = at least 3 entries.
	if resp.Total < 3 {
		t.Errorf("Total = %d, want >= 3", resp.Total)
	}
	// First entry must be the session_start.
	if resp.Entries[0].Kind != ReplayKindSessionStart {
		t.Errorf("first entry = %s, want session_start", resp.Entries[0].Kind)
	}
}
