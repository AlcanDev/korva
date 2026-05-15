package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alcandev/korva/internal/license"
)

// Phase 20.B — verify the liveness + readiness contracts.
//
// /healthz:
//   - returns 200 always when the process responds
//   - body keeps the legacy `"status":"ok"` field (the docker-
//     compose healthcheck greps for it; breaking this without a
//     coordinated config rollout takes the cluster down)
//   - includes the build version so operators can correlate with
//     their deployed image
//
// /readyz:
//   - returns 200 with status="ready" when the DB ping succeeds
//   - returns 503 with status="not_ready" when DB is unreachable
//   - DB check has a 2s timeout (we don't directly test the bound,
//     but we verify it short-circuits when the store is closed)
//   - includes license tier when configured
//   - never fails on missing license (community tier is valid)

func TestHealthz_PreservesLegacyContract(t *testing.T) {
	w := httptest.NewRecorder()
	healthz(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("body not JSON: %v\n%s", err, w.Body.String())
	}
	// CRITICAL: docker-compose.yml healthcheck greps for `"status":"ok"`.
	// Removing or renaming this field breaks every existing
	// deployment that pulled the image without updating
	// docker-compose. Pin the contract.
	if payload["status"] != "ok" {
		t.Errorf("status = %v, want \"ok\" — breaks docker-compose healthcheck", payload["status"])
	}
	if payload["service"] != "korva-vault" {
		t.Errorf("service = %v", payload["service"])
	}
	if _, ok := payload["version"]; !ok {
		t.Error("version field missing")
	}
}

func TestHealthz_DoesNotTouchTheDB(t *testing.T) {
	// /healthz must remain dependency-free so a flaky DB doesn't
	// cause restart storms. We verify by closing the store before
	// the call — /healthz must still return 200.
	s := newAPITestStore(t)
	s.Close()
	w := httptest.NewRecorder()
	healthz(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Errorf("healthz hit DB after close — status %d", w.Code)
	}
}

func TestReadyz_HappyPath(t *testing.T) {
	s := newAPITestStore(t)
	w := httptest.NewRecorder()
	readyz(s, nil)(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var payload readyzPayload
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != "ready" {
		t.Errorf("status = %q, want ready", payload.Status)
	}
	if payload.Checks["db"] != "ok" {
		t.Errorf("db check = %q, want ok", payload.Checks["db"])
	}
	if payload.Service != "korva-vault" {
		t.Errorf("service = %q", payload.Service)
	}
	// No license configured → field omitted via omitempty.
	if payload.License != "" {
		t.Errorf("license should be empty, got %q", payload.License)
	}
}

func TestReadyz_503WhenDBClosed(t *testing.T) {
	s := newAPITestStore(t)
	s.Close()

	w := httptest.NewRecorder()
	readyz(s, nil)(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	var payload readyzPayload
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != "not_ready" {
		t.Errorf("status = %q, want not_ready", payload.Status)
	}
	if payload.Checks["db"] == "ok" {
		t.Errorf("db check should report failure, got %q", payload.Checks["db"])
	}
}

func TestReadyz_IncludesLicenseTierWhenSet(t *testing.T) {
	s := newAPITestStore(t)
	lic := &license.License{Tier: license.Tier("teams")}

	w := httptest.NewRecorder()
	readyz(s, lic)(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var payload readyzPayload
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload.License != "teams" {
		t.Errorf("license = %q, want teams", payload.License)
	}
}

func TestReadyz_HonorsRequestContextDeadline(t *testing.T) {
	// The handler ANDs its 2s timeout with the request context. We
	// can't easily simulate a slow PingContext, but we can verify
	// that an already-canceled request context propagates into
	// the check.
	s := newAPITestStore(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	ctx, cancel := newDeadlineContext()
	cancel() // already canceled
	r = r.WithContext(ctx)

	readyz(s, nil)(w, r)
	// Behavior on canceled context: PingContext returns the
	// cancellation error → check fails → 503.
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("canceled context should fail readiness, got %d", w.Code)
	}
}

// newDeadlineContext returns an already-cancelable context the
// caller can choose to cancel immediately to simulate "request
// gave up before we got to ping the DB".
func newDeadlineContext() (ctx context.Context, cancel context.CancelFunc) {
	return context.WithDeadline(context.Background(), time.Now().Add(50*time.Millisecond))
}
