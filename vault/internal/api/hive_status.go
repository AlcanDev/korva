package api

import (
	"net/http"

	"github.com/alcandev/korva/internal/hive"
)

// hiveStatusHandler handles GET /api/v1/hive/status.
//
// Returns the live Hive worker status. When no worker is running (Hive
// disabled or community tier) it returns a static "disabled" response.
//
// Example response:
//
//	{
//	  "phase":              "healthy",
//	  "last_sync_at":       "2026-04-24T12:00:00Z",
//	  "consecutive_errors": 0,
//	  "pending_count":      3
//	}
func hiveStatusHandler(w *hive.Worker) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if w == nil {
			writeJSON(rw, http.StatusOK, map[string]any{
				"phase":         "disabled",
				"pending_count": 0,
			})
			return
		}
		writeJSON(rw, http.StatusOK, w.Status())
	}
}
