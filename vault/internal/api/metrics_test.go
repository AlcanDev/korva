package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func TestMetricsHandler(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer s.Close()

	h := metricsHandler(s)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	w := httptest.NewRecorder()
	h(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding metrics response: %v", err)
	}

	required := []string{"version", "uptime_seconds", "goroutines", "heap_alloc_mb"}
	for _, key := range required {
		if _, ok := resp[key]; !ok {
			t.Errorf("metrics response missing key %q", key)
		}
	}

	// observations_total should be 0 on an empty store.
	if total, ok := resp["observations_total"]; ok {
		if total.(float64) != 0 {
			t.Errorf("expected observations_total=0, got %v", total)
		}
	} else {
		t.Error("metrics response missing key \"observations_total\"")
	}
}

func TestMetricsHandler_AfterSave(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer s.Close()

	// Save one observation and verify the counter bumps.
	if _, err := s.Save(store.Observation{
		Type:    "pattern",
		Title:   "test",
		Content: "hello world",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	h := metricsHandler(s)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	w := httptest.NewRecorder()
	h(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	total, ok := resp["observations_total"]
	if !ok || total.(float64) != 1 {
		t.Errorf("expected observations_total=1, got %v", total)
	}
}
