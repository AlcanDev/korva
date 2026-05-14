package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 18.C — multi-editor adoption telemetry.
//
// GET /admin/editor/adoption[?days=N] returns aggregated counts of
// interactions per X-Korva-Editor value, over the last N days
// (default 7, cap 90). Empty `editor` rows represent traffic that
// didn't opt in.
//
// Admin-only because the underlying interactions table is admin-
// only — same trust boundary as /admin/interactions.

const (
	adoptionDefaultDays = 7
	adoptionMaxDays     = 90
)

// editorAdoptionPayload is the wire shape the Beacon widget consumes.
// Total is the sum of `rows[].count`, included so the UI can show a
// "85 calls in last 7 days" headline without re-summing client-side.
type editorAdoptionPayload struct {
	WindowDays int                       `json:"window_days"`
	Total      int                       `json:"total"`
	Rows       []store.EditorAdoptionRow `json:"rows"`
}

func adminEditorAdoption(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days := adoptionDefaultDays
		if raw := r.URL.Query().Get("days"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				if n > adoptionMaxDays {
					n = adoptionMaxDays
				}
				days = n
			}
		}
		since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
		rows, err := s.EditorAdoption(since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		total := 0
		for _, r := range rows {
			total += r.Count
		}
		writeJSON(w, http.StatusOK, editorAdoptionPayload{
			WindowDays: days,
			Total:      total,
			Rows:       rows,
		})
	}
}
