package api

// team_scrolls.go — session-token authenticated CRUD for team private scrolls.
//
// Any team member can list and save scrolls scoped to their team.
// Only team admins can delete.
//
// Routes (all behind withSession middleware):
//   GET    /team/scrolls        — list team's private scrolls
//   POST   /team/scrolls        — create or update a scroll (upsert by name)
//   DELETE /team/scrolls/{id}   — delete a scroll (admin only)

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

type teamScrollRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// teamListScrolls returns all private scrolls owned by the authenticated member's team.
// GET /team/scrolls
func teamListScrolls(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, name, content, created_at, updated_at
			   FROM private_scrolls
			  WHERE team_id = ?
			  ORDER BY name ASC`, sess.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		scrolls := make([]teamScrollRow, 0)
		for rows.Next() {
			var sc teamScrollRow
			if err := rows.Scan(&sc.ID, &sc.Name, &sc.Content, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "reading scroll row: "+err.Error())
				return
			}
			scrolls = append(scrolls, sc)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"scrolls": scrolls, "count": len(scrolls)})
	}
}

// teamSaveScroll creates or updates a private scroll for the authenticated member's team.
// Any member can add/edit; only admins can delete (see teamDeleteScroll).
// POST /team/scrolls  — body: {"name":"...", "content":"..."}
func teamSaveScroll(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		var body struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		ctx := r.Context()
		now := time.Now().UTC().Format(time.RFC3339)

		// Check if a scroll with this name already exists for this team.
		var existingID string
		err := s.DB().QueryRowContext(ctx,
			`SELECT id FROM private_scrolls WHERE team_id = ? AND name = ?`,
			sess.teamID, body.Name).Scan(&existingID)

		switch {
		case err == nil:
			// Update existing scroll.
			if _, err := s.DB().ExecContext(ctx,
				`UPDATE private_scrolls SET content = ?, updated_at = ? WHERE id = ?`,
				body.Content, now, existingID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeAudit(s, sess.email, "team_update_scroll", existingID, hashStr(existingID), hashStr(body.Content))
			writeJSON(w, http.StatusOK, map[string]string{"id": existingID, "status": "updated"})

		case errors.Is(err, sql.ErrNoRows):
			// Insert new scroll.
			id := newID()
			if _, err := s.DB().ExecContext(ctx,
				`INSERT INTO private_scrolls(id, name, content, team_id, created_by, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				id, body.Name, body.Content, sess.teamID, sess.email, now, now); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeAudit(s, sess.email, "team_create_scroll", id, "", hashStr(body.Content))
			writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "created"})

		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
	}
}

// teamDeleteScroll removes a private scroll. Only team admins may delete.
// DELETE /team/scrolls/{id}
func teamDeleteScroll(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		if !requireAdmin(sess, w) {
			return
		}
		scrollID := r.PathValue("id")
		if scrollID == "" {
			writeError(w, http.StatusBadRequest, "scroll id is required")
			return
		}

		// Atomic verify + delete: WHERE id=? AND team_id=? prevents TOCTOU and
		// ensures team isolation in a single round-trip.
		res, err := s.DB().ExecContext(r.Context(),
			`DELETE FROM private_scrolls WHERE id = ? AND team_id = ?`, scrollID, sess.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			writeError(w, http.StatusNotFound, "scroll not found")
			return
		}
		writeAudit(s, sess.email, "team_delete_scroll", scrollID, "", "")
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
