package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 2 — Deferred-apply endpoints. Operators inspect / replay / discard
// pulled mutations that failed to apply locally.
//
//   GET    /admin/cloud/deferred                   list rows (default: deferred)
//   POST   /admin/cloud/deferred/{sync_id}/retry   nudge retry_count + last_error
//   POST   /admin/cloud/deferred/{sync_id}/applied mark as applied (manual)
//   DELETE /admin/cloud/deferred/{sync_id}         drop a 'dead' or 'applied' row

// adminListDeferred handles GET /admin/cloud/deferred?status=…&limit=….
func adminListDeferred(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		status := q.Get("status")
		limit := 100
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		rows, err := s.ListDeferred(status, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"deferred": rows,
			"count":    len(rows),
			"status":   status,
		})
	}
}

// retryDeferredRequest is the wire shape for /retry — exactly one field.
type retryDeferredRequest struct {
	LastError string `json:"last_error,omitempty"`
}

// adminRetryDeferred handles POST /admin/cloud/deferred/{sync_id}/retry. It
// records that another retry attempt happened — incrementing retry_count and
// stamping last_error / last_attempted_at. Once retry_count exceeds the
// internal ceiling the row flips to 'dead'. This endpoint does NOT itself
// re-execute the apply; the background replay worker (or the operator's own
// script) is responsible for that.
func adminRetryDeferred(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		syncID := r.PathValue("sync_id")
		if syncID == "" {
			writeError(w, http.StatusBadRequest, "missing sync_id")
			return
		}
		var body retryDeferredRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := s.IncrementDeferredRetry(syncID, body.LastError); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		got, _ := s.GetDeferred(syncID)
		writeJSON(w, http.StatusOK, got)
	}
}

// adminMarkDeferredApplied handles POST /admin/cloud/deferred/{sync_id}/applied.
// Operators reach this endpoint after they have verified — by hand or via a
// migration script — that the dependent state is now present.
func adminMarkDeferredApplied(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		syncID := r.PathValue("sync_id")
		if err := s.MarkDeferredApplied(syncID); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "applied", "sync_id": syncID})
	}
}

// adminDeleteDeferred handles DELETE /admin/cloud/deferred/{sync_id} — final
// cleanup for rows we no longer want around (typically 'dead' entries the
// operator has accepted will never apply).
func adminDeleteDeferred(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		syncID := r.PathValue("sync_id")
		deleted, err := s.DeleteDeferred(syncID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !deleted {
			writeError(w, http.StatusNotFound, "deferred row not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "sync_id": syncID})
	}
}
