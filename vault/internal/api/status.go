package api

import (
	"net/http"
	"time"

	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/version"
	"github.com/alcandev/korva/vault/internal/store"
)

// statusHandler handles GET /api/v1/status.
//
// Returns a lightweight, unauthenticated snapshot for external monitoring
// (load balancers, uptime tools, team dashboards).
//
//	{
//	  "status":             "ok",
//	  "version":            "1.0.0",
//	  "uptime_seconds":     3723,
//	  "license_tier":       "teams",
//	  "observations_total": 142,
//	  "sessions_total":     8
//	}
//
// Intentionally omits sensitive data (hive state, admin key path, etc.).
func statusHandler(s *store.Store, lic *license.License) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"status":         "ok",
			"version":        version.Version,
			"uptime_seconds": int(time.Since(serverStart).Seconds()),
		}

		if lic == nil {
			resp["license_tier"] = "community"
		} else {
			resp["license_tier"] = string(lic.Tier)
		}

		if stats, err := s.Stats(); err == nil {
			resp["observations_total"] = stats.TotalObservations
			resp["sessions_total"] = stats.TotalSessions
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
