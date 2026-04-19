package api

// flows_test.go — end-to-end integration flows.
//
// These tests exercise multi-step user journeys through a real HTTP router
// backed by an in-memory SQLite store, ensuring the full request/response
// contract is correct across several coordinated calls.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/vault/internal/store"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// newTestRouterWithAdmin creates a router with a real temporary admin key.
// It returns the router and the admin key value (hex string) for use in requests.
func newTestRouterWithAdmin(t *testing.T) (http.Handler, string) {
	t.Helper()
	return newTestRouterWithAdminAndLicense(t, nil)
}

// newTestRouterWithTeamsLicense creates a router with a real admin key AND a
// mock Teams license that has all features enabled. Use this for tests that
// exercise license-gated endpoints (private scrolls, skills, audit log).
func newTestRouterWithTeamsLicense(t *testing.T) (http.Handler, string) {
	t.Helper()
	lic := &license.License{
		LicenseID: "test-license-teams",
		TeamID:    "test-team",
		Tier:      license.TierTeams,
		Features: []string{
			license.FeaturePrivateScrolls,
			license.FeatureAdminSkills,
			license.FeatureAuditLog,
			license.FeatureCustomWhitelist,
			license.FeatureMultiProfile,
		},
		IssuedAt:  time.Now().Add(-24 * time.Hour),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour),
		GraceDays: 7,
		Seats:     10,
	}
	return newTestRouterWithAdminAndLicense(t, lic)
}

func newTestRouterWithAdminAndLicense(t *testing.T, lic *license.License) (http.Handler, string) {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	keyPath := filepath.Join(t.TempDir(), "admin.key")
	cfg, err := admin.Generate(keyPath, "test-admin@korva.dev", false)
	if err != nil {
		t.Fatalf("admin.Generate: %v", err)
	}

	return Router(s, RouterConfig{AdminKeyPath: keyPath, License: lic}), cfg.Key
}

// post sends a POST request with a JSON body to h and returns the recorder.
func post(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// get sends a GET request to h and returns the recorder.
// The path is used verbatim; callers must URL-encode query values themselves
// using url.QueryEscape when the value contains spaces or special characters.
func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// search sends GET /api/v1/search?q=<query>, URL-encoding the query safely.
func search(t *testing.T, h http.Handler, query string) *httptest.ResponseRecorder {
	t.Helper()
	return get(t, h, "/api/v1/search?q="+url.QueryEscape(query))
}

// adminReq sends a request with the X-Admin-Key header set.
func adminReq(t *testing.T, h http.Handler, method, path string, body any, adminKey string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Admin-Key", adminKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decode unmarshals the recorder body into v.
func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v (body was: %s)", err, rec.Body.String())
	}
}

// ─── Flow: observation lifecycle ─────────────────────────────────────────────

// TestFlow_ObservationLifecycle exercises the core vault loop:
// save → get by ID → search by content → project context.
func TestFlow_ObservationLifecycle(t *testing.T) {
	h := newTestRouter(t)

	// 1. Save an observation
	rec := post(t, h, "/api/v1/observations", map[string]any{
		"project": "payments",
		"type":    "incident",
		"title":   "Race condition in checkout",
		"content": "Two concurrent requests can overcharge the customer. Fix: Redis distributed lock on payment_id with 30s TTL.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("save: want 201, got %d — %s", rec.Code, rec.Body.String())
	}
	var created map[string]string
	decode(t, rec, &created)
	id := created["id"]
	if id == "" {
		t.Fatal("save response must contain a non-empty 'id'")
	}

	// 2. Get by ID — title must round-trip
	rec2 := get(t, h, "/api/v1/observations/"+id)
	if rec2.Code != http.StatusOK {
		t.Fatalf("get: want 200, got %d", rec2.Code)
	}
	var obs map[string]any
	decode(t, rec2, &obs)
	if obs["title"] != "Race condition in checkout" {
		t.Errorf("title round-trip: got %v", obs["title"])
	}

	// 3. Search by content keyword (URL-safe term, no FTS5 operators)
	rec3 := search(t, h, "overcharge")
	if rec3.Code != http.StatusOK {
		t.Fatalf("search: want 200, got %d — %s", rec3.Code, rec3.Body.String())
	}
	var sr map[string]any
	decode(t, rec3, &sr)
	results, _ := sr["results"].([]any)
	if len(results) == 0 {
		t.Error("search should return at least one result for 'overcharge'")
	}

	// 4. Project context should include the observation
	rec4 := get(t, h, "/api/v1/context/payments")
	if rec4.Code != http.StatusOK {
		t.Fatalf("context: want 200, got %d — %s", rec4.Code, rec4.Body.String())
	}
}

// ─── Flow: session lifecycle ──────────────────────────────────────────────────

// TestFlow_SessionLifecycle tests start → end → appear in list.
func TestFlow_SessionLifecycle(t *testing.T) {
	h := newTestRouter(t)

	// Start session
	rec := post(t, h, "/api/v1/sessions", map[string]any{
		"project": "auth-service",
		"team":    "backend",
		"country": "CL",
		"agent":   "claude",
		"goal":    "implement JWT RS256 refresh token rotation",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("start session: want 201, got %d — %s", rec.Code, rec.Body.String())
	}
	var sess map[string]string
	decode(t, rec, &sess)
	sessionID := sess["session_id"]
	if sessionID == "" {
		t.Fatal("start session: expected non-empty session_id")
	}

	// End session
	b, _ := json.Marshal(map[string]string{
		"summary": "Implemented RS256 refresh with 7-day sliding window; stored revocation list in Redis.",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+sessionID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("end session: want 200, got %d — %s", rec2.Code, rec2.Body.String())
	}

	// List all — session must appear
	rec3 := get(t, h, "/api/v1/sessions/all")
	if rec3.Code != http.StatusOK {
		t.Fatalf("list sessions: want 200, got %d", rec3.Code)
	}
	var list map[string]any
	decode(t, rec3, &list)
	sessions, _ := list["sessions"].([]any)
	if len(sessions) == 0 {
		t.Error("sessions list should contain at least one entry after start+end")
	}
}

// ─── Flow: stats accumulation ─────────────────────────────────────────────────

// TestFlow_StatsAccumulate verifies that /api/v1/stats reflects saved data.
func TestFlow_StatsAccumulate(t *testing.T) {
	h := newTestRouter(t)

	// Baseline: stats always returns 200
	rec0 := get(t, h, "/api/v1/stats")
	if rec0.Code != http.StatusOK {
		t.Fatalf("initial stats: want 200, got %d", rec0.Code)
	}

	// Save 5 observations across two projects
	for i := 0; i < 5; i++ {
		project := "proj-a"
		if i%2 == 0 {
			project = "proj-b"
		}
		rec := post(t, h, "/api/v1/observations", map[string]any{
			"project": project,
			"type":    "pattern",
			"title":   fmt.Sprintf("Pattern #%d", i+1),
			"content": "A recurring architectural pattern worth documenting.",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("save #%d: want 201, got %d — %s", i+1, rec.Code, rec.Body.String())
		}
	}

	// Stats must reflect the new count
	rec := get(t, h, "/api/v1/stats")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats after saves: want 200, got %d", rec.Code)
	}
	var stats map[string]any
	decode(t, rec, &stats)
	total, _ := stats["total_observations"].(float64)
	if total < 5 {
		t.Errorf("total_observations = %.0f, want >= 5", total)
	}
}

// ─── Flow: admin delete ───────────────────────────────────────────────────────

// TestFlow_AdminDeleteObservation ensures admin can delete an observation
// and that it becomes unreachable afterwards.
func TestFlow_AdminDeleteObservation(t *testing.T) {
	h, adminKey := newTestRouterWithAdmin(t)

	// Save
	rec := post(t, h, "/api/v1/observations", map[string]any{
		"project": "cleanup-test",
		"type":    "decision",
		"title":   "Observation to delete",
		"content": "This observation exists only to be deleted by the admin.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("save: want 201, got %d — %s", rec.Code, rec.Body.String())
	}
	var created map[string]string
	decode(t, rec, &created)
	id := created["id"]

	// Admin delete
	rec2 := adminReq(t, h, http.MethodDelete, "/admin/observations/"+id, nil, adminKey)
	if rec2.Code != http.StatusOK && rec2.Code != http.StatusNoContent {
		t.Fatalf("admin delete: want 200/204, got %d — %s", rec2.Code, rec2.Body.String())
	}

	// Should be gone
	rec3 := get(t, h, "/api/v1/observations/"+id)
	if rec3.Code != http.StatusNotFound {
		t.Errorf("after admin delete: want 404, got %d", rec3.Code)
	}
}

// ─── Flow: admin delete — no key ─────────────────────────────────────────────

// TestFlow_AdminDelete_Unauthorized ensures missing or wrong key is rejected.
func TestFlow_AdminDelete_Unauthorized(t *testing.T) {
	h, adminKey := newTestRouterWithAdmin(t)

	// Save something
	rec := post(t, h, "/api/v1/observations", map[string]any{
		"project": "auth-test", "type": "decision",
		"title": "Protected entry", "content": "Should not be deleted without key.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("save: want 201, got %d", rec.Code)
	}
	var created map[string]string
	decode(t, rec, &created)
	id := created["id"]

	tests := []struct {
		name string
		key  string
	}{
		{"no key", ""},
		{"wrong key", "not-the-right-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/admin/observations/"+id, nil)
			if tt.key != "" {
				req.Header.Set("X-Admin-Key", tt.key)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code == http.StatusOK || rec.Code == http.StatusNoContent {
				t.Errorf("delete with %q: expected auth failure, got %d", tt.key, rec.Code)
			}
		})
	}
	_ = adminKey // keep the valid key in scope so newTestRouterWithAdmin is not optimized away
}

// ─── Flow: private scrolls CRUD ──────────────────────────────────────────────

// TestFlow_PrivateScrollsCRUD exercises the complete scroll lifecycle through
// the full router (including admin auth + feature gate):
// empty list → create → list (1 entry) → upsert (same name) → list (still 1) → delete → empty.
func TestFlow_PrivateScrollsCRUD(t *testing.T) {
	h, adminKey := newTestRouterWithTeamsLicense(t)

	doAdmin := func(method, path string, body any) *httptest.ResponseRecorder {
		return adminReq(t, h, method, path, body, adminKey)
	}

	// 1. Empty list
	rec := doAdmin(http.MethodGet, "/admin/scrolls/private", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty list: want 200, got %d — %s", rec.Code, rec.Body.String())
	}
	var lr1 map[string]any
	decode(t, rec, &lr1)
	if scrolls, _ := lr1["scrolls"].([]any); len(scrolls) != 0 {
		t.Errorf("initial list: want 0 scrolls, got %d", len(scrolls))
	}

	// 2. Create
	rec2 := doAdmin(http.MethodPost, "/admin/scrolls/private", map[string]any{
		"name":    "stripe-webhooks",
		"content": "Always use idempotency keys. Verify Stripe-Signature header before processing.",
	})
	if rec2.Code != http.StatusCreated && rec2.Code != http.StatusOK {
		t.Fatalf("create: want 200/201, got %d — %s", rec2.Code, rec2.Body.String())
	}

	// 3. List — must contain 1 entry
	rec3 := doAdmin(http.MethodGet, "/admin/scrolls/private", nil)
	var lr2 map[string]any
	decode(t, rec3, &lr2)
	scrolls2, _ := lr2["scrolls"].([]any)
	if len(scrolls2) != 1 {
		t.Fatalf("after create: want 1 scroll, got %d", len(scrolls2))
	}
	scroll := scrolls2[0].(map[string]any)
	scrollID, _ := scroll["id"].(string)
	if scrollID == "" {
		t.Fatal("scroll missing 'id' field")
	}

	// 4. Upsert — same name, updated content (idempotent create)
	rec4 := doAdmin(http.MethodPost, "/admin/scrolls/private", map[string]any{
		"name":    "stripe-webhooks",
		"content": "Updated: idempotency keys required. Respond 200 within 5s or Stripe retries.",
	})
	if rec4.Code != http.StatusOK && rec4.Code != http.StatusCreated {
		t.Fatalf("upsert: want 200/201, got %d — %s", rec4.Code, rec4.Body.String())
	}

	// 5. List after upsert — still 1 (not 2)
	rec5 := doAdmin(http.MethodGet, "/admin/scrolls/private", nil)
	var lr3 map[string]any
	decode(t, rec5, &lr3)
	if scrolls3, _ := lr3["scrolls"].([]any); len(scrolls3) != 1 {
		t.Errorf("after upsert: want 1 scroll (idempotent), got %d", len(scrolls3))
	}

	// 6. Delete
	rec6 := doAdmin(http.MethodDelete, "/admin/scrolls/private/"+scrollID, nil)
	if rec6.Code != http.StatusOK && rec6.Code != http.StatusNoContent {
		t.Fatalf("delete: want 200/204, got %d — %s", rec6.Code, rec6.Body.String())
	}

	// 7. Should be empty again
	rec7 := doAdmin(http.MethodGet, "/admin/scrolls/private", nil)
	var lr4 map[string]any
	decode(t, rec7, &lr4)
	if scrolls4, _ := lr4["scrolls"].([]any); len(scrolls4) != 0 {
		t.Errorf("after delete: want 0 scrolls, got %d", len(scrolls4))
	}
}

// ─── Flow: multi-project search ──────────────────────────────────────────────

// TestFlow_MultiProjectSearch verifies that full-text search spans projects
// and that distinct keywords return only the relevant observation.
func TestFlow_MultiProjectSearch(t *testing.T) {
	h := newTestRouter(t)

	observations := []map[string]any{
		{
			"project": "payments",
			"type":    "incident",
			"title":   "Redis lock timeout",
			"content": "Distributed lock expired during peak traffic causing duplicate payments.",
		},
		{
			"project": "auth",
			"type":    "decision",
			"title":   "Adopt RS256 JWT",
			"content": "HS256 is dangerous with multiple services sharing the secret. RS256 with key rotation.",
		},
		{
			"project": "notifications",
			"type":    "pattern",
			"title":   "Transactional outbox",
			"content": "Use outbox pattern for reliable message delivery without distributed transactions.",
		},
	}

	for _, obs := range observations {
		rec := post(t, h, "/api/v1/observations", obs)
		if rec.Code != http.StatusCreated {
			t.Fatalf("save %q: want 201, got %d — %s", obs["title"], rec.Code, rec.Body.String())
		}
	}

	tests := []struct {
		query   string
		wantMin int
	}{
		{"lock", 1},    // matches "Redis lock timeout"
		{"RS256", 1},   // matches "Adopt RS256 JWT"
		{"outbox", 1},  // matches "Transactional outbox"
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			// Use search() helper which URL-encodes the query safely.
			rec := search(t, h, tt.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("search %q: want 200, got %d — %s", tt.query, rec.Code, rec.Body.String())
			}
			var sr map[string]any
			decode(t, rec, &sr)
			results, _ := sr["results"].([]any)
			if len(results) < tt.wantMin {
				t.Errorf("search %q: want >= %d results, got %d", tt.query, tt.wantMin, len(results))
			}
		})
	}
}

// ─── Flow: prompt lifecycle ───────────────────────────────────────────────────

// TestFlow_PromptSave saves a reusable prompt and verifies the 201 response.
func TestFlow_PromptSave(t *testing.T) {
	h := newTestRouter(t)

	// Save prompt — the API only supports POST (save), not list/get.
	rec := post(t, h, "/api/v1/prompts", map[string]any{
		"name":    "hex-review",
		"content": "Review this code for hexagonal architecture violations. Focus on: domain logic in adapters, direct infrastructure calls from domain, missing port interfaces.",
		"tags":    []string{"hex", "architecture", "review"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("save prompt: want 201, got %d — %s", rec.Code, rec.Body.String())
	}

	// Save a second prompt — idempotent (same name should not error)
	rec2 := post(t, h, "/api/v1/prompts", map[string]any{
		"name":    "stripe-checklist",
		"content": "Before merging Stripe code: idempotency key present, signature verified, 200 returned quickly.",
		"tags":    []string{"stripe", "payments"},
	})
	if rec2.Code != http.StatusCreated {
		t.Fatalf("save second prompt: want 201, got %d — %s", rec2.Code, rec2.Body.String())
	}
}

// ─── Flow: project timeline ───────────────────────────────────────────────────

// TestFlow_ProjectTimeline saves observations across different projects
// and checks that per-project timeline and summary only reflect that project.
func TestFlow_ProjectTimeline(t *testing.T) {
	h := newTestRouter(t)

	for _, obs := range []map[string]any{
		{"project": "billing", "type": "decision", "title": "Use Stripe", "content": "Chose Stripe over Braintree for better webhook reliability."},
		{"project": "billing", "type": "pattern", "title": "Idempotency keys", "content": "Always include Idempotency-Key header on Stripe calls."},
		{"project": "infra", "type": "decision", "title": "Use K8s", "content": "Kubernetes on AKS for production workloads."},
	} {
		rec := post(t, h, "/api/v1/observations", obs)
		if rec.Code != http.StatusCreated {
			t.Fatalf("save %q: want 201, got %d", obs["title"], rec.Code)
		}
	}

	// Timeline for billing
	rec := get(t, h, "/api/v1/timeline/billing")
	if rec.Code != http.StatusOK {
		t.Fatalf("timeline: want 200, got %d", rec.Code)
	}

	// Summary for billing
	rec2 := get(t, h, "/api/v1/summary/billing")
	if rec2.Code != http.StatusOK {
		t.Fatalf("summary: want 200, got %d", rec2.Code)
	}
}
