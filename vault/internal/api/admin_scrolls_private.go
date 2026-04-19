package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// privateScrollRow is the wire type returned to the admin UI.
type privateScrollRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// adminListPrivateScrolls returns all private scrolls ordered by name.
// GET /admin/scrolls/private
func adminListPrivateScrolls(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, name, content, created_at, updated_at
			   FROM private_scrolls
			  ORDER BY name ASC`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		scrolls := make([]privateScrollRow, 0)
		for rows.Next() {
			var sc privateScrollRow
			if err := rows.Scan(&sc.ID, &sc.Name, &sc.Content, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
				continue
			}
			scrolls = append(scrolls, sc)
		}
		writeJSON(w, http.StatusOK, map[string]any{"scrolls": scrolls, "count": len(scrolls)})
	}
}

// adminSavePrivateScroll creates a new scroll or updates an existing one by name.
// POST /admin/scrolls/private — body: {"name": "...", "content": "..."}
//
// Using name as the upsert key keeps the frontend simple: it calls POST for both
// create and save, and the backend decides whether to insert or update.
func adminSavePrivateScroll(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		// Check for an existing scroll with this name.
		var existingID string
		err := s.DB().QueryRowContext(ctx,
			`SELECT id FROM private_scrolls WHERE name = ?`, body.Name).
			Scan(&existingID)

		switch {
		case err == nil:
			// Update existing scroll.
			if _, err := s.DB().ExecContext(ctx,
				`UPDATE private_scrolls SET content = ?, updated_at = ? WHERE id = ?`,
				body.Content, now, existingID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeAudit(s, actor, "update_private_scroll", existingID, hashStr(existingID), hashStr(body.Content))
			writeJSON(w, http.StatusOK, map[string]string{"id": existingID, "status": "updated"})

		case errors.Is(err, sql.ErrNoRows):
			// Insert new scroll.
			id := newID()
			if _, err := s.DB().ExecContext(ctx,
				`INSERT INTO private_scrolls(id, name, content, created_by, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				id, body.Name, body.Content, actor, now, now); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeAudit(s, actor, "create_private_scroll", id, "", hashStr(body.Content))
			writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "created"})

		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
	}
}

// adminDeletePrivateScroll removes a private scroll by ID.
// DELETE /admin/scrolls/private/{scroll_id}
func adminDeletePrivateScroll(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scrollID := r.PathValue("scroll_id")

		res, err := s.DB().ExecContext(r.Context(),
			`DELETE FROM private_scrolls WHERE id = ?`, scrollID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "scroll not found")
			return
		}
		writeAudit(s, actor, "delete_private_scroll", scrollID, "", "")
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
