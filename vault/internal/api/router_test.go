package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"context"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

const testAdminKey = "test-admin-key"

// newTestRouter creates an API router backed by an in-memory store.
// Admin endpoints are accessible using testAdminKey via X-Admin-Key header.
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return Router(context.Background(), s, RouterConfig{AdminKeyOverride: testAdminKey})
}

func TestHealthz(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz = %d, want 200", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want 'ok'", body["status"])
	}
}

func TestSaveObservation_Created(t *testing.T) {
	h := newTestRouter(t)

	payload := map[string]any{
		"project": "home-api",
		"type":    "decision",
		"title":   "Use hexagonal architecture",
		"content": "We decided to use ports and adapters pattern.",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/observations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("POST /api/v1/observations = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("response should contain a non-empty 'id'")
	}
}

func TestSaveObservation_BadBody(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/observations",
		bytes.NewReader([]byte("not-json")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", rec.Code)
	}
}

func TestGetObservation_NotFound(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/observations/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/v1/observations/nonexistent = %d, want 404", rec.Code)
	}
}

func TestGetObservation_Found(t *testing.T) {
	h := newTestRouter(t)

	// Save first
	payload := map[string]any{"project": "p", "type": "decision", "title": "T", "content": "C"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/observations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var created map[string]string
	json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"]

	// Now get
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/observations/"+id, nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("GET /api/v1/observations/%s = %d, want 200", id, rec2.Code)
	}
}

func TestSearch_ReturnsResults(t *testing.T) {
	h := newTestRouter(t)

	// Save something
	payload := map[string]any{"project": "my-api", "type": "pattern", "title": "Hexagonal", "content": "ports and adapters"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/observations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Search
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=hexagonal", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("GET /api/v1/search = %d, want 200", rec2.Code)
	}
}

func TestContext_ReturnsOK(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/context/home-api", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/context/home-api = %d, want 200", rec.Code)
	}
}

func TestTimeline_ReturnsOK(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/timeline/home-api", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/timeline/home-api = %d, want 200", rec.Code)
	}
}

func TestStartSession_Created(t *testing.T) {
	h := newTestRouter(t)
	payload := map[string]any{
		"project": "home-api", "team": "backend", "country": "CL",
		"agent": "copilot", "goal": "implement feature",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("POST /api/v1/sessions = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestStartSession_BadBody(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions",
		bytes.NewReader([]byte("not-json")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid session body, got %d", rec.Code)
	}
}

func TestEndSession_OK(t *testing.T) {
	h := newTestRouter(t)

	// Start a session first
	payload := map[string]any{"project": "p", "team": "t", "country": "CL", "agent": "copilot", "goal": "g"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var created map[string]string
	json.NewDecoder(rec.Body).Decode(&created)
	sessionID := created["session_id"]

	// End it
	endBody, _ := json.Marshal(map[string]string{"summary": "completed"})
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+sessionID, bytes.NewReader(endBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("PUT /api/v1/sessions/%s = %d, want 200", sessionID, rec2.Code)
	}
}

func TestSummary_ReturnsOK(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/summary/home-api", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/summary/home-api = %d, want 200", rec.Code)
	}
}

func TestStats_ReturnsOK(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/stats = %d, want 200", rec.Code)
	}
}

func TestSavePrompt_Created(t *testing.T) {
	h := newTestRouter(t)
	payload := map[string]any{
		"name":    "hexagonal-review",
		"content": "Review hexagonal boundaries...",
		"tags":    []string{"hex", "review"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("POST /api/v1/prompts = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestSavePrompt_BadBody(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prompts",
		bytes.NewReader([]byte("not-json")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid prompt body, got %d", rec.Code)
	}
}

func TestCORS_HeadersPresentOnGET(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/stats = %d, want 200", rec.Code)
	}
	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin == "" {
		t.Error("Access-Control-Allow-Origin header missing on GET response")
	}
	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Access-Control-Allow-Methods header missing on GET response")
	}
}

func TestAdminPurge_Forbidden_NoKey(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/purge", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Errorf("POST /admin/purge without key = %d, want 401 or 403", rec.Code)
	}
}

func TestListAllSessions_ReturnsOK(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/sessions/all", nil)
	req.Header.Set("X-Admin-Key", testAdminKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /admin/sessions/all = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if _, ok := resp["sessions"]; !ok {
		t.Error("response missing 'sessions' key")
	}
}

func TestTimeline_WithDateRange(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/timeline/home-api?from=2025-01-01T00:00:00Z&to=2025-12-31T23:59:59Z", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/timeline with date range = %d, want 200", rec.Code)
	}
}

func TestAdminDeleteObservation_Forbidden(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodDelete, "/admin/observations/obs-123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Errorf("DELETE /admin/observations without key = %d, want 401 or 403", rec.Code)
	}
}
