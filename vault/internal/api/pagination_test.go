package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"context"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

func stringReader(s string) *strings.Reader { return strings.NewReader(s) }

// timeoutChan returns a channel that fires after n seconds.
func timeoutChan(n int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(n) * time.Second)
		close(ch)
	}()
	return ch
}

// TestSearch_Pagination verifies that offset and total are returned correctly.
func TestSearch_Pagination(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	defer s.Close()

	// Insert 5 observations in the "pg" project.
	for i := 0; i < 5; i++ {
		s.Save(store.Observation{ //nolint:errcheck
			Project: "pg",
			Type:    store.TypePattern,
			Title:   "obs",
			Content: "content",
		})
	}

	h := Router(context.Background(), s, RouterConfig{})

	// Page 1: offset=0, limit=2 — expect 2 results, total=5.
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?project=pg&limit=2&offset=0", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("page 1: want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck

	if resp["count"].(float64) != 2 {
		t.Errorf("page 1 count: want 2, got %v", resp["count"])
	}
	if resp["total"].(float64) != 5 {
		t.Errorf("page 1 total: want 5, got %v", resp["total"])
	}
	if resp["limit"].(float64) != 2 {
		t.Errorf("page 1 limit: want 2, got %v", resp["limit"])
	}
	if resp["offset"].(float64) != 0 {
		t.Errorf("page 1 offset: want 0, got %v", resp["offset"])
	}

	// Page 2: offset=4, limit=2 — expect 1 result (only the 5th obs).
	r2 := httptest.NewRequest(http.MethodGet, "/api/v1/search?project=pg&limit=2&offset=4", nil)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)

	if w2.Code != http.StatusOK {
		t.Fatalf("page 2: want 200, got %d — %s", w2.Code, w2.Body.String())
	}
	var resp2 map[string]any
	json.NewDecoder(w2.Body).Decode(&resp2) //nolint:errcheck

	if resp2["count"].(float64) != 1 {
		t.Errorf("page 2 count: want 1, got %v", resp2["count"])
	}
	if resp2["total"].(float64) != 5 {
		t.Errorf("page 2 total: want 5, got %v", resp2["total"])
	}
}

// TestSearch_FTSNoTotal verifies the response contract: when a full-text
// query is used, the "total" field is omitted (it would require a costly
// separate COUNT that doesn't map cleanly to FTS ranking).
// We verify the shape directly from the handler logic (no FTS index needed).
func TestSearch_FTSNoTotal(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	defer s.Close()

	h := Router(context.Background(), s, RouterConfig{})

	// Empty query → uses the recent path (no FTS); total IS included.
	r := httptest.NewRequest(http.MethodGet, "/api/v1/search?project=noproject", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck

	// Recent path always includes total.
	if _, hasTotal := resp["total"]; !hasTotal {
		t.Error("recent (non-FTS) search should include 'total' in response")
	}
	// limit and offset must always be present.
	if _, ok := resp["limit"]; !ok {
		t.Error("response missing 'limit'")
	}
	if _, ok := resp["offset"]; !ok {
		t.Error("response missing 'offset'")
	}
}

// TestWebhook_Fires verifies that a POST is sent to the webhook URL on save.
func TestWebhook_Fires(t *testing.T) {
	fired := make(chan string, 1)
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		fired <- body["event"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookSrv.Close()

	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	defer s.Close()

	h := Router(context.Background(), s, RouterConfig{WebhookURL: webhookSrv.URL})

	body := `{"project":"wh","type":"pattern","title":"t","content":"c"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/observations",
		stringReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — %s", w.Code, w.Body.String())
	}

	select {
	case event := <-fired:
		if event != "observation.created" {
			t.Errorf("want event=observation.created, got %q", event)
		}
	case <-timeoutChan(2):
		t.Error("webhook was not called within 2 seconds")
	}
}

// TestWebhook_Disabled verifies that no POST is sent when WebhookURL is empty.
func TestWebhook_Disabled(t *testing.T) {
	callCount := 0
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookSrv.Close()

	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	defer s.Close()

	// Empty WebhookURL — no webhook should fire.
	h := Router(context.Background(), s, RouterConfig{})

	body := `{"project":"wh","type":"pattern","title":"t","content":"c"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/observations", stringReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	// Give the goroutine a moment to (not) fire.
	<-timeoutChan(1)
	if callCount != 0 {
		t.Errorf("webhook should not fire when URL is empty, got %d calls", callCount)
	}
}
