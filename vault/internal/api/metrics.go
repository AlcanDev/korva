package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/alcandev/korva/internal/version"
	"github.com/alcandev/korva/vault/internal/store"
)

// serverStart records the instant the current process was launched.
// Used by the /api/v1/metrics endpoint to compute uptime_seconds.
var serverStart = time.Now()

// metricsHandler returns a lightweight JSON snapshot of vault health.
//
// GET /api/v1/metrics
//
//	{
//	  "version":             "1.0.0 (abc123) built 2026-01-01",
//	  "uptime_seconds":      3723.4,
//	  "goroutines":          14,
//	  "heap_alloc_mb":       2.3,
//	  "observations_total":  142,
//	  "sessions_total":      8,
//	  "prompts_total":       3,
//	  "by_type":             {"pattern": 80, "decision": 30, ...}
//	}
//
// No authentication required — the data is non-sensitive aggregate counts.
func metricsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		resp := map[string]any{
			"version":        version.String(),
			"uptime_seconds": time.Since(serverStart).Seconds(),
			"goroutines":     runtime.NumGoroutine(),
			"heap_alloc_mb":  float64(ms.HeapAlloc) / (1024 * 1024),
		}

		if vStats, err := s.Stats(); err == nil {
			resp["observations_total"] = vStats.TotalObservations
			resp["sessions_total"] = vStats.TotalSessions
			resp["prompts_total"] = vStats.TotalPrompts
			resp["by_type"] = vStats.ByType
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
