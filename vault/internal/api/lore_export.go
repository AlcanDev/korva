package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// loreExportHandler handles GET /team/lore/export (requires X-Session-Token).
//
// Exports the authenticated team's private scrolls as structured notes with
// hash-based change detection. Supports incremental export via ?since=<RFC3339>.
//
// The team_id is always derived from the validated session — callers cannot
// enumerate another team's scrolls by guessing an ID.
//
// Query parameters:
//
//	since  — RFC3339 timestamp; return only scrolls updated after this time
//	limit  — max results (default 100, max 500)
//	offset — skip N results for pagination
//
// Example:
//
//	GET /team/lore/export?since=2026-01-01T00:00:00Z
//	X-Session-Token: <token>
func loreExportHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Session is already validated and injected by sessMW.
		sess := sessionFromCtx(r)

		opts := store.ExportScrollsOptions{
			TeamID: sess.teamID, // always scoped to the authenticated team
		}

		if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
			if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
				opts.Since = t
			} else {
				writeError(w, http.StatusBadRequest, "invalid since parameter — use RFC3339 format")
				return
			}
		}

		opts.Limit, opts.Offset = parsePagination(r, 100, 500)

		notes, total, err := s.ExportScrolls(opts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to export scrolls")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"notes":       notes,
			"count":       len(notes),
			"total":       total,
			"exported_at": time.Now().UTC().Format(time.RFC3339),
			"incremental": !opts.Since.IsZero(),
			"team_id":     sess.teamID,
			"pagination": map[string]any{
				"limit":    opts.Limit,
				"offset":   opts.Offset,
				"has_more": opts.Offset+len(notes) < total,
			},
		})
	}
}

// adminLoreExportHandler handles GET /admin/lore/export (requires X-Admin-Key).
//
// Admin-only variant that allows exporting scrolls for any team. Used for
// backups, compliance exports, and cross-team admin operations.
//
// Query parameters:
//
//	team_id — required; the team whose scrolls to export
//	since   — RFC3339 timestamp for incremental sync
//	limit   — max results (default 100, max 1000)
//	offset  — pagination offset
func adminLoreExportHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.URL.Query().Get("team_id")
		// team_id is optional for admin — omitting returns all teams (bulk backup)

		opts := store.ExportScrollsOptions{
			TeamID: teamID,
		}

		if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
			if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
				opts.Since = t
			} else {
				writeError(w, http.StatusBadRequest, "invalid since parameter — use RFC3339 format")
				return
			}
		}

		opts.Limit, opts.Offset = parsePagination(r, 100, 1000)

		notes, total, err := s.ExportScrolls(opts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to export scrolls")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"notes":       notes,
			"count":       len(notes),
			"total":       total,
			"exported_at": time.Now().UTC().Format(time.RFC3339),
			"incremental": !opts.Since.IsZero(),
			"team_id":     teamID,
			"pagination": map[string]any{
				"limit":    opts.Limit,
				"offset":   opts.Offset,
				"has_more": opts.Offset+len(notes) < total,
			},
		})
	}
}

// parsePagination reads limit/offset query params with sensible defaults and caps.
func parsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}
