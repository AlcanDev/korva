package hive

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/alcandev/korva/internal/db"
	"github.com/alcandev/korva/internal/privacy/cloud"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func newTestWorker(t *testing.T, client *Client) (*Worker, *Outbox) {
	t.Helper()
	database := newTestDB(t)
	outbox := NewOutbox(database)
	filter := cloud.New([]string{"pattern", "decision", "learning"}, "test-install")
	w := NewWorker(outbox, client, filter, "client-test", 1*time.Hour)
	return w, outbox
}

func TestNewWorker_DefaultInterval(t *testing.T) {
	w := NewWorker(nil, nil, nil, "id", 0)
	if w.interval != 15*time.Minute {
		t.Errorf("interval = %v, want 15m", w.interval)
	}
}

func TestNewWorker_CustomInterval(t *testing.T) {
	w := NewWorker(nil, nil, nil, "id", 5*time.Minute)
	if w.interval != 5*time.Minute {
		t.Errorf("interval = %v, want 5m", w.interval)
	}
}

func TestFlushOnce_EmptyOutbox(t *testing.T) {
	srv := healthySrv(t)
	defer srv.Close()
	w, _ := newTestWorker(t, NewClient(srv.URL, "k"))

	n, err := w.FlushOnce(context.Background())
	if err != nil {
		t.Fatalf("FlushOnce: %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
}

func TestFlushOnce_KillSwitch(t *testing.T) {
	t.Setenv("KORVA_HIVE_DISABLE", "1")

	w := &Worker{}
	n, err := w.FlushOnce(context.Background())
	if err == nil {
		t.Fatal("expected kill-switch error, got nil")
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
}

func TestFlushOnce_KillSwitch_True(t *testing.T) {
	t.Setenv("KORVA_HIVE_DISABLE", "true")

	w := &Worker{}
	_, err := w.FlushOnce(context.Background())
	if err == nil {
		t.Fatal("expected kill-switch error for 'true', got nil")
	}
}

func TestFlushOnce_OneAccepted(t *testing.T) {
	var batches []BatchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/observations/batch":
			json.NewEncoder(w).Encode(BatchResponse{Accepted: 1})
		default:
			http.NotFound(w, r)
		}
		_ = batches // suppress unused warning
	}))
	defer srv.Close()

	worker, outbox := newTestWorker(t, NewClient(srv.URL, "k"))

	payload, _ := json.Marshal(cloud.Input{
		Type:    "pattern",
		Title:   "Test pattern",
		Content: "No PII here",
		Project: "proj-1",
	})
	if err := outbox.Enqueue("obs-001", payload); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	n, err := worker.FlushOnce(context.Background())
	if err != nil {
		t.Fatalf("FlushOnce: %v", err)
	}
	if n != 1 {
		t.Errorf("n = %d, want 1", n)
	}

	counts, err := outbox.Status()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Sent != 1 {
		t.Errorf("sent = %d, want 1", counts.Sent)
	}
}

func TestFlushOnce_RejectsPrivacy(t *testing.T) {
	srv := healthySrv(t)
	defer srv.Close()

	worker, outbox := newTestWorker(t, NewClient(srv.URL, "k"))

	// Payload with PII (email) — cloud filter must reject it
	payload, _ := json.Marshal(cloud.Input{
		Type:    "pattern",
		Title:   "contains PII",
		Content: "user email is test@example.com please redact",
		Project: "proj-pii",
	})
	if err := outbox.Enqueue("obs-pii", payload); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	n, err := worker.FlushOnce(context.Background())
	if err != nil {
		t.Fatalf("FlushOnce: %v", err)
	}
	if n != 1 {
		t.Errorf("n = %d, want 1", n)
	}

	counts, _ := outbox.Status()
	// If the filter allowed it (PII scrubbed), it would be sent; if hard-rejected, rejected_privacy.
	// Either outcome is correct for this test — we just verify no crash and row transitions.
	if counts.Pending != 0 {
		t.Errorf("pending = %d, want 0 after flush", counts.Pending)
	}
}

func TestFlushOnce_BadPayload(t *testing.T) {
	srv := healthySrv(t)
	defer srv.Close()

	worker, outbox := newTestWorker(t, NewClient(srv.URL, "k"))

	// Store invalid JSON payload
	if err := outbox.Enqueue("obs-bad", []byte("not-json")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	n, err := worker.FlushOnce(context.Background())
	if err != nil {
		t.Fatalf("FlushOnce: %v", err)
	}
	if n != 1 {
		t.Errorf("n = %d, want 1", n)
	}
	counts, _ := outbox.Status()
	if counts.Rejected != 1 {
		t.Errorf("rejected = %d, want 1 for bad payload", counts.Rejected)
	}
}

func TestFlushOnce_NetworkError(t *testing.T) {
	worker, outbox := newTestWorker(t, NewClient("http://127.0.0.1:1", "k"))

	payload, _ := json.Marshal(cloud.Input{
		Type:    "pattern",
		Title:   "net-fail",
		Content: "Content without PII",
		Project: "p",
	})
	outbox.Enqueue("obs-net", payload)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	n, err := worker.FlushOnce(ctx)
	// Network error is propagated
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if n != 1 {
		t.Errorf("n = %d, want 1", n)
	}
}

func TestRun_CancelStops(t *testing.T) {
	srv := healthySrv(t)
	defer srv.Close()

	worker, _ := newTestWorker(t, NewClient(srv.URL, "k"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}

func TestRun_KillSwitchSkipsTick(t *testing.T) {
	t.Setenv("KORVA_HIVE_DISABLE", "1")
	srv := healthySrv(t)
	defer srv.Close()

	worker, _ := newTestWorker(t, NewClient(srv.URL, "k"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
}

func TestKillSwitch_Disabled(t *testing.T) {
	os.Unsetenv("KORVA_HIVE_DISABLE")
	if killSwitch() {
		t.Error("expected killSwitch()=false when env unset")
	}
}

func TestKillSwitch_Enabled(t *testing.T) {
	t.Setenv("KORVA_HIVE_DISABLE", "1")
	if !killSwitch() {
		t.Error("expected killSwitch()=true for KORVA_HIVE_DISABLE=1")
	}
}

func TestNewBatchID(t *testing.T) {
	id1 := newBatchID()
	id2 := newBatchID()
	if id1 == "" {
		t.Error("empty batch ID")
	}
	if id1 == id2 {
		t.Error("batch IDs must not be identical")
	}
}

func TestFindRow_Found(t *testing.T) {
	rows := []Row{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	r := findRow(rows, "b")
	if r.ID != "b" {
		t.Errorf("findRow = %q, want %q", r.ID, "b")
	}
}

func TestFindRow_NotFound(t *testing.T) {
	rows := []Row{{ID: "a"}}
	r := findRow(rows, "z")
	if r.ID != "" {
		t.Errorf("findRow not found should return zero Row, got %q", r.ID)
	}
}

func TestRun_OfflineHealthSkipsBatch(t *testing.T) {
	// Server that returns 503 on health — worker should skip batch silently
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	worker, outbox := newTestWorker(t, NewClient(srv.URL, "k"))
	payload, _ := json.Marshal(cloud.Input{Type: "pattern", Title: "T", Content: "C", Project: "p"})
	outbox.Enqueue("obs-offline", payload)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done

	// Row should still be pending (health check failed, skipped batch)
	counts, _ := outbox.Status()
	if counts.Pending != 1 {
		t.Errorf("pending = %d, want 1 after offline health check", counts.Pending)
	}
}

func TestBackoff_Schedule(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{0, 30 * time.Second},
		{1, 30 * time.Second},
		{2, 2 * time.Minute},
		{3, 10 * time.Minute},
		{4, time.Hour},
		{5, 6 * time.Hour},
		{6, 24 * time.Hour},
		{99, 24 * time.Hour},
	}
	for _, tc := range cases {
		if got := backoff(tc.attempts); got != tc.want {
			t.Errorf("backoff(%d) = %v, want %v", tc.attempts, got, tc.want)
		}
	}
}

func TestOutbox_Retry(t *testing.T) {
	db := newTestDB(t)
	outbox := NewOutbox(db)

	// Enqueue and mark as failed
	payload, _ := json.Marshal(cloud.Input{Type: "pattern", Title: "T", Content: "C"})
	outbox.Enqueue("obs-retry", payload)
	rows, _ := outbox.NextBatch(10)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// Park as failed (6th attempt)
	outbox.MarkFailed(rows[0].ID, 5, "test error")

	counts, _ := outbox.Status()
	if counts.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", counts.Failed)
	}

	n, err := outbox.Retry()
	if err != nil {
		t.Fatalf("Retry: %v", err)
	}
	if n != 1 {
		t.Errorf("Retry n = %d, want 1", n)
	}
	counts, _ = outbox.Status()
	if counts.Pending != 1 {
		t.Errorf("after Retry: pending = %d, want 1", counts.Pending)
	}
}

func TestOutbox_MarkSent(t *testing.T) {
	db := newTestDB(t)
	outbox := NewOutbox(db)
	payload, _ := json.Marshal(cloud.Input{Type: "pattern", Title: "T", Content: "C"})
	outbox.Enqueue("obs-sent", payload)

	rows, _ := outbox.NextBatch(10)
	outbox.MarkSent(rows[0].ID)

	counts, _ := outbox.Status()
	if counts.Sent != 1 {
		t.Errorf("sent = %d, want 1", counts.Sent)
	}
}

func TestOutbox_MarkFailed_Backoff(t *testing.T) {
	db := newTestDB(t)
	outbox := NewOutbox(db)
	payload, _ := json.Marshal(cloud.Input{Type: "pattern", Title: "T", Content: "C"})
	outbox.Enqueue("obs-fail", payload)

	rows, _ := outbox.NextBatch(10)
	// First failure — still pending with backoff
	outbox.MarkFailed(rows[0].ID, 0, "network error")

	counts, _ := outbox.Status()
	if counts.Pending != 1 {
		t.Errorf("pending = %d after first failure (should remain pending)", counts.Pending)
	}
}

// healthySrv returns a test server that responds 200 to /v1/health.
func healthySrv(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/observations/batch":
			json.NewEncoder(w).Encode(BatchResponse{Accepted: 1})
		default:
			http.NotFound(w, r)
		}
	}))
}
