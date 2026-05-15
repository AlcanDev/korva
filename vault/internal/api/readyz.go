package api

import (
	"context"
	"net/http"
	"time"

	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/version"
	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 20.B — readiness probe.
//
// Distinct from /healthz (liveness — "the process is up"), /readyz
// answers the deeper "is this instance ready to serve traffic?"
// question. K8s, GCP load balancers, ECS target groups all respect
// the convention: liveness restarts the pod when the process hangs;
// readiness only pulls it out of rotation when its dependencies are
// not OK. Mixing them causes restart storms when the DB blips.
//
// What we check:
//   - DB ping with a 2s timeout. SQLite is local-disk so a slow
//     ping usually means the file is locked by a long-running write
//     transaction or the disk is exhausted.
//   - License tier (informational, never fails the probe — a vault
//     missing a license still serves the community surface).
//
// We deliberately do NOT check the Hive worker or external IdP
// here: those are best-effort dependencies. If we did, every IdP
// hiccup would pull the vault out of rotation while it can still
// serve admin + observation traffic.

const readinessTimeout = 2 * time.Second

// readyzPayload is the documented wire shape. Status is "ready" /
// "not_ready"; checks names a per-dependency string (either "ok" or
// the error message). Stable across releases — operators script
// against it.
type readyzPayload struct {
	Status  string            `json:"status"`
	Service string            `json:"service"`
	Version string            `json:"version"`
	Checks  map[string]string `json:"checks"`
	License string            `json:"license,omitempty"` // tier name when present
}

func readyz(s *store.Store, lic *license.License) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		checks := map[string]string{}
		ready := true

		if err := s.DB().PingContext(ctx); err != nil {
			checks["db"] = err.Error()
			ready = false
		} else {
			checks["db"] = "ok"
		}

		payload := readyzPayload{
			Status:  "ready",
			Service: "korva-vault",
			Version: version.Version,
			Checks:  checks,
		}
		if lic != nil {
			payload.License = string(lic.Tier)
		}
		status := http.StatusOK
		if !ready {
			payload.Status = "not_ready"
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, payload)
	}
}
