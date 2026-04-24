package api

// team_skills.go — session-token authenticated CRUD for team skills.
//
// Any team member (role="member" or "admin") can list and save skills.
// Only team admins (role="admin") can delete.
//
// Routes (all behind withSession middleware):
//   GET    /team/skills        — list team's skills
//   POST   /team/skills        — create or update a skill (upsert by name)
//   DELETE /team/skills/{id}   — delete a skill (admin only)

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

type teamSkillRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Body      string `json:"body"`
	Tags      string `json:"tags"` // JSON array string
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// teamListSkills returns all skills owned by the authenticated member's team.
// GET /team/skills
func teamListSkills(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, name, body, tags, created_at, updated_at
			   FROM skills
			  WHERE team_id = ?
			  ORDER BY name ASC`, sess.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		skills := make([]teamSkillRow, 0)
		for rows.Next() {
			var sk teamSkillRow
			if err := rows.Scan(&sk.ID, &sk.Name, &sk.Body, &sk.Tags, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "reading skill row: "+err.Error())
				return
			}
			skills = append(skills, sk)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"skills": skills, "count": len(skills)})
	}
}

// teamSaveSkill creates or updates a skill for the authenticated member's team.
// Any member can add/edit; only admins can delete (see teamDeleteSkill).
// POST /team/skills  — body: {"name":"...", "body":"...", "tags":[...]}
func teamSaveSkill(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		var body struct {
			Name string   `json:"name"`
			Body string   `json:"body"`
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		tagsJSON := "[]"
		if body.Tags != nil {
			if b, err := json.Marshal(body.Tags); err == nil {
				tagsJSON = string(b)
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)

		// Upsert by (team_id, name) — ON CONFLICT updates body, tags, updated_at.
		newID := newID()
		if _, err := s.DB().ExecContext(r.Context(),
			`INSERT INTO skills(id, team_id, name, body, tags, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(team_id, name)
			 DO UPDATE SET body = excluded.body,
			               tags = excluded.tags,
			               updated_at = excluded.updated_at`,
			newID, sess.teamID, body.Name, body.Body, tagsJSON, now, now,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Resolve the actual ID (may differ when an existing row was updated by ON CONFLICT).
		var actualID string
		if err := s.DB().QueryRowContext(r.Context(),
			`SELECT id FROM skills WHERE team_id = ? AND name = ?`,
			sess.teamID, body.Name).Scan(&actualID); err != nil {
			writeError(w, http.StatusInternalServerError, "resolving skill id: "+err.Error())
			return
		}

		writeAudit(s, sess.email, "team_save_skill", actualID, "", hashStr(body.Body))
		writeJSON(w, http.StatusOK, map[string]string{"id": actualID, "status": "saved"})
	}
}

// teamDeleteSkill removes a skill. Only team admins may delete.
// DELETE /team/skills/{id}
func teamDeleteSkill(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		if !requireAdmin(sess, w) {
			return
		}
		skillID := r.PathValue("id")
		if skillID == "" {
			writeError(w, http.StatusBadRequest, "skill id is required")
			return
		}

		// Atomic verify + delete: WHERE id=? AND team_id=? prevents TOCTOU.
		res, err := s.DB().ExecContext(r.Context(),
			`DELETE FROM skills WHERE id = ? AND team_id = ?`, skillID, sess.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}
		writeAudit(s, sess.email, "team_delete_skill", skillID, "", "")
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
