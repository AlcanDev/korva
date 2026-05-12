package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// adminListActivity handles GET /admin/activity — the Observatory timeline.
// Filters: project, model, agent, status, q (FTS5), from, to (RFC3339).
// Pagination: limit (default 50, max 500), offset.
func adminListActivity(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		f := store.InteractionFilters{
			Project: q.Get("project"),
			Model:   q.Get("model"),
			Agent:   q.Get("agent"),
			Status:  q.Get("status"),
			Query:   q.Get("q"),
		}
		if from, ok := parseTime(q.Get("from")); ok {
			f.Since = from
		}
		if to, ok := parseTime(q.Get("to")); ok {
			f.Until = to
		}
		if lim, _ := strconv.Atoi(q.Get("limit")); lim > 0 {
			f.Limit = lim
		}
		if off, _ := strconv.Atoi(q.Get("offset")); off > 0 {
			f.Offset = off
		}

		rows, err := s.ListInteractions(f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		total, _ := s.CountInteractions(f) // -1 for FTS

		out := make([]map[string]any, len(rows))
		for i, in := range rows {
			out[i] = activityRowJSON(in)
		}

		resp := map[string]any{
			"interactions": out,
			"limit":        f.Limit,
			"offset":       f.Offset,
		}
		if total >= 0 {
			resp["total"] = total
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// adminGetActivity handles GET /admin/activity/{id} — full interaction detail.
func adminGetActivity(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing interaction id")
			return
		}
		got, err := s.GetInteraction(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if got == nil {
			writeError(w, http.StatusNotFound, "interaction not found")
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}

// activityRowJSON formats an interaction for the timeline list (compact shape
// without the full prompt/response — those are returned only by the detail
// endpoint).
func activityRowJSON(in store.Interaction) map[string]any {
	return map[string]any{
		"id":             in.ID,
		"ts":             in.CreatedAt.UTC().Format(time.RFC3339),
		"project":        in.Project,
		"team":           in.Team,
		"agent":          in.Agent,
		"model":          in.Model,
		"duration_ms":    in.DurationMs,
		"input_tokens":   in.InputTokens,
		"output_tokens":  in.OutputTokens,
		"cache_read":     in.CacheRead,
		"cache_creation": in.CacheCreation,
		"prompt_excerpt": in.PromptExcerpt,
		"status":         in.Status,
		"estimated":      in.Estimated,
	}
}

// parseTime parses an RFC3339 timestamp, returning ok=false for empty or invalid input.
func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
