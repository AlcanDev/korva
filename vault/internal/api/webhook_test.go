package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 20.A — direct coverage for notifyWebhook. Async by design
// (fires in a goroutine so it doesn't block the save response), so
// the tests use sync.WaitGroup + channels to deterministically wait
// for the request without sleeping.

func TestNotifyWebhook_NoOpOnEmptyURL(t *testing.T) {
	// Should return immediately and not try to dial. We can't directly
	// assert "no goroutine spawned", but we CAN assert the call
	// returns quickly and doesn't panic.
	done := make(chan struct{})
	go func() {
		notifyWebhook("", store.Observation{Project: "p", Title: "t"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("notifyWebhook with empty URL should return immediately")
	}
}

func TestNotifyWebhook_PostsObservationPayload(t *testing.T) {
	var (
		gotBody []byte
		gotMethod string
		gotCT     string
		gotUA     string
		mu        sync.Mutex
		wg        sync.WaitGroup
	)
	wg.Add(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		gotUA = r.Header.Get("User-Agent")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifyWebhook(srv.URL, store.Observation{
		ID: "obs-1", Project: "korva", Title: "test", Content: "x",
		Type: store.TypeLearning,
	})

	// Wait for the goroutine to fire — generous timeout for CI.
	waitWithTimeout(t, &wg, 2*time.Second)

	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if gotUA == "" || !contains(gotUA, "korva-vault") {
		t.Errorf("User-Agent should identify korva-vault, got %q", gotUA)
	}

	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, string(gotBody))
	}
	if payload["event"] != "observation.created" {
		t.Errorf("event = %v, want observation.created", payload["event"])
	}
	if payload["ts"] == nil || payload["ts"] == "" {
		t.Errorf("ts missing: %v", payload["ts"])
	}
	obs, _ := payload["observation"].(map[string]any)
	if obs == nil {
		t.Fatal("observation field missing")
	}
	if obs["id"] != "obs-1" || obs["project"] != "korva" || obs["title"] != "test" {
		t.Errorf("observation payload wrong: %+v", obs)
	}
}

func TestNotifyWebhook_SwallowsServerErrors(t *testing.T) {
	// If the webhook receiver returns 500, notifyWebhook must NOT
	// panic / propagate — webhooks are best-effort. We just verify
	// the goroutine completes.
	var wg sync.WaitGroup
	wg.Add(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		defer wg.Done()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	notifyWebhook(srv.URL, store.Observation{Project: "p", Title: "t"})
	waitWithTimeout(t, &wg, 2*time.Second)
	// If we got here without panic, the test passes — the goroutine
	// caught the >=400 status and logged it.
}

func TestNotifyWebhook_HandlesUnreachableURL(t *testing.T) {
	// A URL that never resolves / connects must NOT panic and must
	// NOT block the caller. We can't easily assert on goroutine
	// completion here (the goroutine waits for the http.Client's
	// own dial timeout which can be longer than our patience), but
	// we CAN assert that the OUTER call returns immediately.
	done := make(chan struct{})
	go func() {
		notifyWebhook("http://127.0.0.1:1/never-listens", store.Observation{Project: "p", Title: "t"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("notifyWebhook should return immediately even when the URL is unreachable (goroutine handles the dial)")
	}
}

// waitWithTimeout fails the test when wg doesn't return within d.
// Used by every webhook test so a regression that loses the
// goroutine fail-fast instead of hanging the suite.
func waitWithTimeout(t *testing.T, wg *sync.WaitGroup, d time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("timed out after %s waiting for webhook goroutine", d)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
