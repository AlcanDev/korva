package hive

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Phase 4.3 — exhaustive coverage for the reason-code classifier and the
// distinct PhasePulling phase. These tests pin the public contract of
// WorkerStatus.Reason so the Beacon dashboard + `korva status` can dispatch
// on stable labels.

func TestClassifyError_NilIsNone(t *testing.T) {
	if got := ClassifyError(nil); got != ReasonNone {
		t.Errorf("nil → %q, want %q", got, ReasonNone)
	}
}

func TestClassifyError_HTTPStatusCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ReasonCode
	}{
		{"401 unauthorized", errors.New("hive returned 401: unauthorized"), ReasonAuthRequired},
		{"403 forbidden", errors.New("hive returned 403: forbidden"), ReasonAuthRequired},
		{"410 gone", errors.New("hive returned 410: schema gone"), ReasonServerUnsupported},
		{"426 upgrade required", errors.New("hive returned 426: upgrade required"), ReasonServerUnsupported},
		{"501 not implemented", errors.New("hive returned 501: not implemented"), ReasonServerUnsupported},
		{"500 internal", errors.New("hive returned 500: internal server error"), ReasonInternalError},
		{"502 bad gateway", errors.New("hive returned 502: bad gateway"), ReasonInternalError},
		{"503 unavailable", errors.New("hive returned 503: service unavailable"), ReasonInternalError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyError(tc.err); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyError_TransportShapes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ReasonCode
	}{
		{"no such host", errors.New("dial tcp: lookup hive.example: no such host"), ReasonTransportFailed},
		{"connection refused", errors.New("dial tcp 127.0.0.1:9: connection refused"), ReasonTransportFailed},
		{"timeout",
			errors.New("Post \"http://hive\": net/http: request canceled (Client.Timeout exceeded)"),
			ReasonTransportFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyError(tc.err); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyError_PolicyForbidden(t *testing.T) {
	err := fmt.Errorf("filter rejected_privacy: forbidden token")
	if got := ClassifyError(err); got != ReasonPolicyForbidden {
		t.Errorf("got %q, want %q", got, ReasonPolicyForbidden)
	}
}

// ── PhasePulling wiring ─────────────────────────────────────────────────────

// A saver that records nothing — used to drive pullTick without involving the
// real store. Implements ObservationSaver.
type recordingSaver struct {
	saved int
}

func (r *recordingSaver) ExistsObservation(string) (bool, error) { return false, nil }
func (r *recordingSaver) SavePulled(_, _, _, _, _, _ string, _ []string) error {
	r.saved++
	return nil
}

// pullSrv returns a test server that serves a single fake observation on
// /v1/observations/pull and 200 on /v1/health.
func pullSrv(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/observations":
			w.Header().Set("Content-Type", "application/json")
			// Mirrors PullResponse: count, next_since, observations[].
			_, _ = w.Write([]byte(`{
				"count": 1,
				"next_since": "2026-05-01T00:00:00Z",
				"observations": [
					{"id":"obs-pull-1","project":"p","type":"context","title":"t","content":"c","author":"a","tags":[]}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestPullTick_RecordsPullCountAndCursor(t *testing.T) {
	srv := pullSrv(t)
	defer srv.Close()

	w, _ := newTestWorker(t, NewClient(srv.URL, "k"))
	saver := &recordingSaver{}
	w.saver = saver

	if err := w.pullTick(context.Background()); err != nil {
		t.Fatalf("pullTick: %v", err)
	}
	if saver.saved != 1 {
		t.Errorf("saver.saved = %d, want 1", saver.saved)
	}

	s := w.Status()
	if s.PullCount != 1 {
		t.Errorf("PullCount = %d, want 1", s.PullCount)
	}
	if s.LastPullAt == nil {
		t.Error("LastPullAt should be set after a successful pull")
	}
}

func TestPullTick_ClassifiesErrorWithoutChangingPhase(t *testing.T) {
	// Point the client at a port that refuses connections.
	w, _ := newTestWorker(t, NewClient("http://127.0.0.1:1", "k"))
	w.saver = &recordingSaver{}

	// Pretend we just finished a healthy push.
	w.setPhase(PhaseHealthy)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	err := w.pullTick(ctx)
	if err == nil {
		t.Fatal("expected error from pullTick against dead endpoint")
	}

	s := w.Status()
	if s.Reason != ReasonTransportFailed {
		t.Errorf("reason = %q, want %q", s.Reason, ReasonTransportFailed)
	}
	// Pull errors must NOT poison the overall phase — push side is the
	// source of truth for healthy/backoff.
	if s.Phase != PhaseHealthy {
		t.Errorf("phase = %q, want %q (pull errors are non-fatal)", s.Phase, PhaseHealthy)
	}
}

func TestRecordFailure_SetsReasonCode(t *testing.T) {
	w, _ := newTestWorker(t, NewClient("http://127.0.0.1:1", "k"))
	w.recordFailure(errors.New("hive returned 401: unauthorized"))
	s := w.Status()
	if s.Reason != ReasonAuthRequired {
		t.Errorf("reason after 401 = %q, want %q", s.Reason, ReasonAuthRequired)
	}
	if s.Phase != PhaseBackoff {
		t.Errorf("phase = %q, want %q", s.Phase, PhaseBackoff)
	}
}

func TestRecordSuccess_ClearsReasonCode(t *testing.T) {
	w, _ := newTestWorker(t, NewClient("http://127.0.0.1:1", "k"))
	w.recordFailure(errors.New("hive returned 500"))
	if r := w.Status().Reason; r != ReasonInternalError {
		t.Fatalf("setup: reason = %q, want %q", r, ReasonInternalError)
	}
	w.recordSuccess(time.Now().UTC())
	if r := w.Status().Reason; r != ReasonNone {
		t.Errorf("after recordSuccess, reason = %q, want empty", r)
	}
}

func TestKillSwitch_SetsSyncPausedReason(t *testing.T) {
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
	if s.Reason != ReasonSyncPaused {
		t.Errorf("reason = %q, want %q", s.Reason, ReasonSyncPaused)
	}
}
