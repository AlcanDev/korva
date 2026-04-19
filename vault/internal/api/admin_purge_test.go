package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/vault/internal/store"
)

// newStoreWithAdmin creates an in-memory store + router with a temp admin key.
// Returns (store, handler, adminKeyHex) so tests can pre-populate data and verify state.
func newStoreWithAdmin(t *testing.T) (*store.Store, http.Handler, string) {
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

	return s, Router(s, RouterConfig{AdminKeyPath: keyPath}), cfg.Key
}

func purgeReq(t *testing.T, h http.Handler, body map[string]any, adminKey string) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/admin/purge", bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Admin-Key", adminKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAdminPurge_RequiresAuth(t *testing.T) {
	_, h, _ := newStoreWithAdmin(t)
	r := httptest.NewRequest(http.MethodPost, "/admin/purge", bytes.NewBufferString(`{}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAdminPurge_RequiresFilter(t *testing.T) {
	_, h, key := newStoreWithAdmin(t)
	w := purgeReq(t, h, map[string]any{}, key)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d — body: %s", w.Code, w.Body.String())
	}
}

func TestAdminPurge_DryRun(t *testing.T) {
	s, h, key := newStoreWithAdmin(t)

	s.Save(store.Observation{Project: "demo", Type: "pattern", Title: "t", Content: "c"})   //nolint:errcheck
	s.Save(store.Observation{Project: "demo", Type: "pattern", Title: "t2", Content: "c2"}) //nolint:errcheck

	w := purgeReq(t, h, map[string]any{
		"project": "demo",
		"dry_run": true,
	}, key)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if deleted := resp["deleted"].(float64); deleted != 2 {
		t.Fatalf("dry_run count: want 2, got %v", deleted)
	}
	if dryRun := resp["dry_run"].(bool); !dryRun {
		t.Error("want dry_run=true in response")
	}

	// Database must be untouched.
	results, _ := s.Search("", store.SearchFilters{Project: "demo", Limit: 10})
	if len(results) != 2 {
		t.Fatalf("dry-run must not delete: want 2, got %d", len(results))
	}
}

func TestAdminPurge_Delete(t *testing.T) {
	s, h, key := newStoreWithAdmin(t)

	s.Save(store.Observation{Project: "target", Type: "pattern", Title: "x", Content: "x"}) //nolint:errcheck
	s.Save(store.Observation{Project: "safe", Type: "pattern", Title: "y", Content: "y"})   //nolint:errcheck

	w := purgeReq(t, h, map[string]any{"project": "target"}, key)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if deleted := resp["deleted"].(float64); deleted != 1 {
		t.Fatalf("want deleted=1, got %v", deleted)
	}

	// Safe project untouched.
	safe, _ := s.Search("", store.SearchFilters{Project: "safe", Limit: 10})
	if len(safe) != 1 {
		t.Fatalf("safe project: want 1, got %d", len(safe))
	}
}

func TestAdminExport_RequiresAuth(t *testing.T) {
	_, h, _ := newStoreWithAdmin(t)
	r := httptest.NewRequest(http.MethodGet, "/admin/export", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAdminExport_JSONL(t *testing.T) {
	s, h, key := newStoreWithAdmin(t)

	s.Save(store.Observation{Project: "p", Type: "pattern", Title: "first", Content: "hello"})   //nolint:errcheck
	s.Save(store.Observation{Project: "p", Type: "decision", Title: "second", Content: "world"}) //nolint:errcheck

	r := httptest.NewRequest(http.MethodGet, "/admin/export?project=p", nil)
	r.Header.Set("X-Admin-Key", key)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("want Content-Type application/x-ndjson, got %s", ct)
	}
	if w.Header().Get("Content-Disposition") == "" {
		t.Error("missing Content-Disposition header")
	}

	// Body should be valid JSONL — two lines.
	lines := bytes.Split(bytes.TrimSpace(w.Body.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("want 2 JSONL lines, got %d", len(lines))
	}
	for i, line := range lines {
		var obs map[string]any
		if err := json.Unmarshal(line, &obs); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestStatusHandler(t *testing.T) {
	_, h, _ := newStoreWithAdmin(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	for _, k := range []string{"status", "version", "uptime_seconds", "license_tier", "observations_total"} {
		if _, ok := resp[k]; !ok {
			t.Errorf("status response missing key %q", k)
		}
	}
	if resp["status"] != "ok" {
		t.Errorf("want status=ok, got %v", resp["status"])
	}
	if resp["license_tier"] != "community" {
		t.Errorf("want license_tier=community (nil license), got %v", resp["license_tier"])
	}
}
