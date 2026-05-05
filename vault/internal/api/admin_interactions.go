package api

import (
	"net/http"
	"strconv"

	"github.com/alcandev/korva/vault/internal/store"
)

func adminListInteractions(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := store.CallFilters{
			Tool:    q.Get("tool"),
			Project: q.Get("project"),
			Author:  q.Get("author"),
			Status:  q.Get("status"),
		}
		if lim, _ := strconv.Atoi(q.Get("limit")); lim > 0 {
			f.Limit = lim
		}
		if off, _ := strconv.Atoi(q.Get("offset")); off > 0 {
			f.Offset = off
		}

		calls, err := s.ListCalls(f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		type row struct {
			ID        string `json:"id"`
			Tool      string `json:"tool"`
			Project   string `json:"project"`
			Author    string `json:"author"`
			Status    string `json:"status"`
			LatencyMs int64  `json:"latency_ms"`
			ErrorMsg  string `json:"error_msg,omitempty"`
			CreatedAt string `json:"created_at"`
		}
		out := make([]row, len(calls))
		for i, c := range calls {
			out[i] = row{
				ID:        c.ID,
				Tool:      c.Tool,
				Project:   c.Project,
				Author:    c.Author,
				Status:    c.Status,
				LatencyMs: c.LatencyMs,
				ErrorMsg:  c.ErrorMsg,
				CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z"),
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"calls": out, "count": len(out)})
	}
}

func adminInteractionStats(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.GetCallStats()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}
