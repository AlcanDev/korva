package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// listObservationsSince handles GET /v1/observations?since=<RFC3339>&limit=<n>&project=<p>
//
// This endpoint allows the Korva Hive (and peer vaults) to pull observations
// from this vault. It is intentionally unauthenticated for the same reason
// POST /v1/observations/batch is unauthenticated — both sides of the sync are
// covered by the Hive API key at the network level, not the HTTP layer.
//
// Responses are paginated by `since` cursor (createdAt). Callers advance the
// cursor by passing the created_at of the last received observation.
func listObservationsSince(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		var since time.Time
		if raw := q.Get("since"); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid since: must be RFC3339")
				return
			}
			since = t
		}

		limit := 200
		if lim, err := strconv.Atoi(q.Get("limit")); err == nil && lim > 0 && lim <= 500 {
			limit = lim
		}

		obs, err := s.Search("", store.SearchFilters{
			Project: q.Get("project"),
			Since:   since,
			Limit:   limit,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Determine next cursor from the oldest record in this page
		// (observations are returned newest-first, so the last item is oldest)
		var nextSince string
		if len(obs) > 0 {
			nextSince = obs[len(obs)-1].CreatedAt.UTC().Format(time.RFC3339)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"observations": obs,
			"count":        len(obs),
			"next_since":   nextSince,
		})
	}
}
