package hive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alcandev/korva/internal/privacy/cloud"

	_ "modernc.org/sqlite"
)

func TestWorkerStatus_InitialPhaseIsIdle(t *testing.T) {
	srv := healthySrv(t)
	defer srv.Close()
	w, _ := newTestWorker(t, NewClient(srv.URL, "k"))

	s := w.Status()
	if s.Phase != PhaseIdle {
		t.Errorf("initial phase = %q, want %q", s.Phase, PhaseIdle)
	}
	if s.ConsecutiveErrors != 0 {
		t.Errorf("initial consecutive_errors = %d, want 0", s.ConsecutiveErrors)
	}
}

func TestWorkerStatus_AfterSuccessfulFlush(t *testing.T) {
	srv := batchAcceptSrv(t)
	defer srv.Close()

	w, outbox := newTestWorker(t, NewClient(srv.URL, "k"))
	outbox.Enqueue("obs-status-ok", testCloudPayload(t)) //nolint:errcheck

	_, err := w.FlushOnce(context.Background())
	if err != nil {
		t.Fatalf("FlushOnce: %v", err)
	}

	s := w.Status()
	if s.Phase != PhaseHealthy {
		t.Errorf("phase = %q, want %q", s.Phase, PhaseHealthy)
	}
	if s.LastSyncAt == nil {
		t.Error("last_sync_at should be set after successful flush")
	}
	if s.ConsecutiveErrors != 0 {
		t.Errorf("consecutive_errors = %d, want 0", s.ConsecutiveErrors)
	}
	if s.LastError != "" {
		t.Errorf("last_error should be empty after success, got %q", s.LastError)
	}
}

func TestWorkerStatus_AfterNetworkFailure(t *testing.T) {
	w, outbox := newTestWorker(t, NewClient("http://127.0.0.1:1", "k"))
	outbox.Enqueue("obs-fail-status", testCloudPayload(t)) //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, _ = w.FlushOnce(ctx)

	s := w.Status()
	if s.Phase != PhaseBackoff {
		t.Errorf("phase = %q, want %q", s.Phase, PhaseBackoff)
	}
	if s.ConsecutiveErrors != 1 {
		t.Errorf("consecutive_errors = %d, want 1", s.ConsecutiveErrors)
	}
	if s.BackoffUntil == nil {
		t.Error("backoff_until should be set after failure")
	}
	if s.LastError == "" {
		t.Error("last_error should be non-empty after failure")
	}
}

func TestWorkerStatus_DisabledWhenKillSwitch(t *testing.T) {
	t.Setenv("KORVA_HIVE_DISABLE", "1")
	srv := healthySrv(t)
	defer srv.Close()
	w, _ := newTestWorker(t, NewClient(srv.URL, "k"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	s := w.Status()
	if s.Phase != PhaseDisabled {
		t.Errorf("phase = %q, want %q", s.Phase, PhaseDisabled)
	}
}

func TestJitterBackoff_IsPositive(t *testing.T) {
	for errors := 0; errors <= 6; errors++ {
		d := jitterBackoff(errors)
		if d <= 0 {
			t.Errorf("jitterBackoff(%d) = %v, want > 0", errors, d)
		}
	}
}

func TestJitterBackoff_BasesIncrease(t *testing.T) {
	bases := []time.Duration{
		30 * time.Second,
		30 * time.Second, // errors=1 maps to same base as 0
		2 * time.Minute,
		10 * time.Minute,
		time.Hour,
		6 * time.Hour,
	}
	for i, base := range bases {
		// Minimum possible value (base - 25% jitter)
		minExpected := base - base/4
		if got := jitterBackoff(i); got < minExpected/2 {
			t.Errorf("jitterBackoff(%d) = %v too far below base %v", i, got, base)
		}
	}
}

// --- helpers ---

func batchAcceptSrv(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/observations/batch":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"accepted":1}`)) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
}

func testCloudPayload(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(cloud.Input{
		Type:    "pattern",
		Title:   "test pattern",
		Content: "content without PII",
		Project: "proj",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}
