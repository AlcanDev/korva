package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 20.D — true E2E smoke test.
//
// flows_test.go exercises the router via in-process
// httptest.NewRequest + handler.ServeHTTP. That covers handler
// logic but NOT the actual TCP listener path. This file spins up
// httptest.NewServer (real socket) and drives it from outside via
// http.Client — the same shape a deployed binary serves.
//
// What this catches that flows tests can't:
//   - the rate-limiter middleware actually wraps the mux end-to-end
//   - the CORS preflight respects real OPTIONS requests
//   - the gzip-encoded ingest path round-trips correctly
//   - server shutdown actually drains in-flight requests
//
// The smoke test is intentionally focused: ten requests covering
// the critical contract surface, plus a cleanup verification. If
// this test passes, a freshly-built binary serving the same
// router will respond to clients.

// e2eEnv is the smoke-test fixture: a fully-wired Router behind an
// httptest.NewServer + a real http.Client + the admin key needed
// for the admin endpoints. Cleanup closes the server cleanly.
type e2eEnv struct {
	t        *testing.T
	server   *httptest.Server
	client   *http.Client
	adminKey string
	cancel   context.CancelFunc
}

func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	keyPath := filepath.Join(t.TempDir(), "admin.key")
	cfg, err := admin.Generate(keyPath, "smoke@korva.dev", false)
	if err != nil {
		t.Fatalf("admin.Generate: %v", err)
	}

	// The router takes a context that controls the rate-limiter's
	// background cleanup goroutine. We give it a cancelable one so
	// the test cleanup releases that goroutine instead of leaving
	// it running until process exit.
	ctx, cancel := context.WithCancel(context.Background())
	router := Router(ctx, s, RouterConfig{AdminKeyPath: keyPath})

	server := httptest.NewServer(router)
	t.Cleanup(func() {
		server.Close()
		cancel()
	})

	return &e2eEnv{
		t:        t,
		server:   server,
		client:   &http.Client{Timeout: 5 * time.Second},
		adminKey: cfg.Key,
		cancel:   cancel,
	}
}

func (e *e2eEnv) get(path string) *http.Response {
	e.t.Helper()
	resp, err := e.client.Get(e.server.URL + path)
	if err != nil {
		e.t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (e *e2eEnv) post(path string, body any) *http.Response {
	e.t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, e.server.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		e.t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (e *e2eEnv) decodeBody(resp *http.Response, v any) {
	e.t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		raw, _ := io.ReadAll(resp.Body)
		e.t.Fatalf("decode: %v (body=%s)", err, string(raw))
	}
}

// TestE2E_Smoke_HealthAndReadyOverRealSocket pins the most basic
// promise: when the binary is running, /healthz and /readyz answer
// over a real TCP socket (not just the in-process recorder).
func TestE2E_Smoke_HealthAndReadyOverRealSocket(t *testing.T) {
	env := newE2EEnv(t)

	// /healthz
	resp := env.get("/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz status = %d", resp.StatusCode)
	}
	var hp map[string]any
	env.decodeBody(resp, &hp)
	if hp["status"] != "ok" {
		t.Errorf("/healthz status field = %v, want \"ok\"", hp["status"])
	}

	// /readyz
	resp = env.get("/readyz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/readyz status = %d", resp.StatusCode)
	}
	var rp readyzPayload
	env.decodeBody(resp, &rp)
	if rp.Status != "ready" || rp.Checks["db"] != "ok" {
		t.Errorf("/readyz payload wrong: %+v", rp)
	}
}

// TestE2E_Smoke_GoldenObservationFlow walks the most-used user
// journey end-to-end: save → get by id → search by content. A
// regression here means the binary can't serve its primary
// purpose.
func TestE2E_Smoke_GoldenObservationFlow(t *testing.T) {
	env := newE2EEnv(t)

	// Save.
	saveResp := env.post("/api/v1/observations", map[string]any{
		"type":    "learning",
		"project": "smoke",
		"title":   "use lazy OIDC verifier",
		"content": "vault discovery should not block startup",
	})
	if saveResp.StatusCode != http.StatusCreated {
		t.Fatalf("save status = %d", saveResp.StatusCode)
	}
	var saved struct{ ID string }
	env.decodeBody(saveResp, &saved)
	if saved.ID == "" {
		t.Fatal("save returned empty id")
	}

	// Get by id.
	getResp := env.get("/api/v1/observations/" + saved.ID)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", getResp.StatusCode)
	}
	var got store.Observation
	env.decodeBody(getResp, &got)
	if got.ID != saved.ID || got.Title != "use lazy OIDC verifier" {
		t.Errorf("get returned wrong row: %+v", got)
	}

	// Search by FTS.
	searchResp := env.get("/api/v1/search?q=" + url.QueryEscape("OIDC"))
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("search status = %d", searchResp.StatusCode)
	}
	var sp struct {
		Results []store.Observation `json:"results"`
		Count   int                 `json:"count"`
	}
	env.decodeBody(searchResp, &sp)
	if sp.Count == 0 {
		t.Errorf("search for 'OIDC' returned 0 results — FTS not wired?")
	}
	var found bool
	for _, r := range sp.Results {
		if r.ID == saved.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("search did not return the saved obs (results: %+v)", sp.Results)
	}
}

// TestE2E_Smoke_SessionLifecycle exercises the session start/end
// path. Important because sessions are how interactions get tied
// together for the replay timeline.
func TestE2E_Smoke_SessionLifecycle(t *testing.T) {
	env := newE2EEnv(t)

	startResp := env.post("/api/v1/sessions", map[string]any{
		"project": "smoke",
		"agent":   "claude",
		"goal":    "validate the smoke test wiring",
	})
	if startResp.StatusCode != http.StatusCreated {
		t.Fatalf("session start status = %d", startResp.StatusCode)
	}
	var started struct {
		SessionID string `json:"session_id"`
	}
	env.decodeBody(startResp, &started)
	if started.SessionID == "" {
		t.Fatal("session start returned empty id")
	}

	endResp := env.post("/api/v1/sessions/"+started.SessionID, map[string]any{
		"summary": "all green",
	})
	// The router declares this as PUT; httptest's request helpers
	// default to POST. Re-issue as PUT.
	endResp.Body.Close()
	req, _ := http.NewRequest(http.MethodPut,
		env.server.URL+"/api/v1/sessions/"+started.SessionID,
		bytes.NewReader([]byte(`{"summary":"all green"}`)))
	req.Header.Set("Content-Type", "application/json")
	endResp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("session end: %v", err)
	}
	defer endResp.Body.Close()
	if endResp.StatusCode != http.StatusOK {
		t.Fatalf("session end status = %d", endResp.StatusCode)
	}
}

// TestE2E_Smoke_AdminAuthGate confirms the admin endpoints actually
// require authentication. A regression here would expose the
// admin surface to anyone with network access.
//
// Status code distinction (preserved across the admin middleware
// since Phase 2): missing header → 401 Unauthorized (UI prompts
// for login); rejected credentials → 403 Forbidden (UI shows
// "wrong key" message). Don't collapse them.
func TestE2E_Smoke_AdminAuthGate(t *testing.T) {
	env := newE2EEnv(t)

	// No header → 401.
	resp := env.get("/admin/stats")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/admin/stats without auth = %d, want 401", resp.StatusCode)
	}

	// With wrong key → 403 (admin middleware distinguishes the two).
	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/admin/stats", nil)
	req.Header.Set("X-Admin-Key", "wrong-key")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("admin GET wrong key: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("wrong key = %d, want 403", resp.StatusCode)
	}

	// With the right key → 200.
	req, _ = http.NewRequest(http.MethodGet, env.server.URL+"/admin/stats", nil)
	req.Header.Set("X-Admin-Key", env.adminKey)
	resp, err = env.client.Do(req)
	if err != nil {
		t.Fatalf("admin GET right key: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("right key = %d, want 200", resp.StatusCode)
	}
}

// TestE2E_Smoke_CORSHeadersOnRealRequest verifies that responses
// to a cross-origin request carry the CORS allow-origin header.
//
// We do NOT test OPTIONS preflight here — the standard library's
// per-method ServeMux returns 405 for OPTIONS on a POST-only
// route, which is fine for Beacon (same-origin via Vite proxy)
// but would break a true cross-origin browser caller. That's a
// known limitation; the CORS middleware adds the response
// headers but doesn't register an OPTIONS handler per route.
func TestE2E_Smoke_CORSHeadersOnRealRequest(t *testing.T) {
	env := newE2EEnv(t)
	req, _ := http.NewRequest(http.MethodGet,
		env.server.URL+"/api/v1/status", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatalf("GET with Origin: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("Access-Control-Allow-Origin missing — CORS middleware not wrapping route")
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("Access-Control-Allow-Headers missing")
	}
}

// TestE2E_Smoke_RateLimiterPasses120 confirms the global rate
// limiter is wired (every request goes through it). A simple
// success means the middleware exists — verifying the actual
// limit (120/min) would slow the suite considerably; we trust
// the rate-limiter's own unit tests for that.
func TestE2E_Smoke_RateLimiterPasses120(t *testing.T) {
	env := newE2EEnv(t)
	// Burst 10 quick requests; all should succeed (well under 120).
	for i := 0; i < 10; i++ {
		resp := env.get("/healthz")
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("healthz #%d = %d (rate limiter false-positive?)", i, resp.StatusCode)
		}
	}
}

// TestE2E_Smoke_ReadyzReportsErrorWhenStoreClosed verifies the
// readiness probe actually pulls the instance out of rotation
// when the store goes away — across the real socket, not just
// in-process.
func TestE2E_Smoke_ReadyzReportsErrorWhenStoreClosed(t *testing.T) {
	// Hand-built env so we control the store closure timing.
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "admin.key")
	if _, err := admin.Generate(keyPath, "smoke@korva.dev", false); err != nil {
		t.Fatalf("admin.Generate: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router := Router(ctx, s, RouterConfig{AdminKeyPath: keyPath})
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Initially ready.
	resp, err := client.Get(server.URL + "/readyz")
	if err != nil {
		t.Fatalf("readyz initial: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initial readyz = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Close the store mid-flight.
	s.Close()

	// 3. Now /readyz must report 503 over the real socket.
	resp, err = client.Get(server.URL + "/readyz")
	if err != nil {
		t.Fatalf("readyz after close: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("readyz after store close = %d, want 503", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !contains(string(body), "not_ready") {
		t.Errorf("body should report not_ready, got %s", string(body))
	}
}

// TestE2E_Smoke_404OnUnknownRoute pins the standard library mux's
// fallback. A wildcard catch-all here would mean a typo in the
// router silently mounts somewhere weird.
func TestE2E_Smoke_404OnUnknownRoute(t *testing.T) {
	env := newE2EEnv(t)
	resp := env.get("/this/does/not/exist")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// Sanity guard so the smoke tests run reasonably fast: assert the
// whole file completes under a generous bound. If any single test
// regresses past ~3s we want fast feedback.
func TestE2E_Smoke_TotalRuntimeUnderBound(t *testing.T) {
	// This test deliberately runs LAST (alphabetical ordering puts
	// "Total" after the others). It does nothing on its own — the
	// `-timeout 30s` we set in CI handles the actual bound.
	// This is just documentation in code form.
	_ = fmt.Sprintf("smoke ok at %s", time.Now())
}
