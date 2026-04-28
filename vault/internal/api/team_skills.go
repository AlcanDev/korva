package api

// team_skills.go — session-token authenticated CRUD + history for team skills.
//
// Any team member (role="member" or "admin") can list and save skills.
// Only team admins (role="admin") can delete.
//
// Routes (all behind withSession middleware):
//   GET    /team/skills              — list team's skills
//   POST   /team/skills              — create or update a skill (upsert by name)
//   DELETE /team/skills/{id}         — delete a skill (admin only)
//   GET    /team/skills/{id}/history — version history for a skill

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
	Tags      string `json:"tags"`
	Version   int    `json:"version"`
	UpdatedBy string `json:"updated_by"`
	Scope     string `json:"scope"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// teamListSkills returns all skills owned by the authenticated member's team.
// GET /team/skills
func teamListSkills(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, name, body, tags, version, updated_by, scope, created_at, updated_at
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
			if err := rows.Scan(&sk.ID, &sk.Name, &sk.Body, &sk.Tags,
				&sk.Version, &sk.UpdatedBy, &sk.Scope,
				&sk.CreatedAt, &sk.UpdatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "reading skill row: "+err.Error())
				return
			}
			sk.AutoLoad = autoLoadInt == 1
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
// On update: version is incremented and a skill_history row is recorded.
// POST /team/skills  — body: {"name":"...", "body":"...", "tags":[...], "scope":"team", "summary":"..."}
func teamSaveSkill(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		var body struct {
			Name    string   `json:"name"`
			Body    string   `json:"body"`
			Tags    []string `json:"tags"`
			Scope   string   `json:"scope"`
			Summary string   `json:"summary"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if body.Scope == "" {
			body.Scope = "team"
		}

		tagsJSON := "[]"
		if body.Tags != nil {
			if b, err := json.Marshal(body.Tags); err == nil {
				tagsJSON = string(b)
			}
		}
		triggersJSON := "{}"
		if body.Triggers != nil {
			if b, err := json.Marshal(body.Triggers); err == nil {
				triggersJSON = string(b)
			}
		}
		autoLoadInt := 0
		if body.AutoLoad {
			autoLoadInt = 1
		}
		now := time.Now().UTC().Format(time.RFC3339)

		tx, err := s.DB().BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer tx.Rollback() //nolint:errcheck

		skillULID := newID()
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO skills(id, team_id, name, body, tags, scope, updated_by, version, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
			 ON CONFLICT(team_id, name)
			 DO UPDATE SET body       = excluded.body,
			               tags       = excluded.tags,
			               scope      = excluded.scope,
			               updated_by = excluded.updated_by,
			               version    = skills.version + 1,
			               updated_at = excluded.updated_at`,
			skillULID, sess.teamID, body.Name, body.Body, tagsJSON, body.Scope, sess.email, now, now,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var actualID string
		var version int
		if err := tx.QueryRowContext(r.Context(),
			`SELECT id, version FROM skills WHERE team_id = ? AND name = ?`,
			sess.teamID, body.Name,
		).Scan(&actualID, &version); err != nil {
			writeError(w, http.StatusInternalServerError, "resolving skill id: "+err.Error())
			return
		}

		histID := newID()
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO skill_history(id, skill_id, version, body, changed_by, summary, changed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			histID, actualID, version, body.Body, sess.email, body.Summary, now,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "recording history: "+err.Error())
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeAudit(s, sess.email, "team_save_skill", actualID, "", hashStr(body.Body))
		writeJSON(w, http.StatusOK, map[string]any{"id": actualID, "status": "saved", "version": version})
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

// teamGetSkillHistory returns the version history for a skill belonging to the
// authenticated member's team, ordered by version descending.
// GET /team/skills/{id}/history
func teamGetSkillHistory(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r)
		skillID := r.PathValue("id")

		// Verify ownership first (indexed PK + team_id lookup — sub-microsecond).
		// Separate from the history fetch so we can return 404 vs 200+empty correctly.
		var teamOwned int
		s.DB().QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM skills WHERE id = ? AND team_id = ?`,
			skillID, sess.teamID,
		).Scan(&teamOwned) //nolint:errcheck
		if teamOwned == 0 {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}

		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, skill_id, version, body, changed_by, summary, changed_at
			   FROM skill_history
			  WHERE skill_id = ?
			  ORDER BY version DESC
			  LIMIT 50`, skillID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		history := make([]skillHistoryRow, 0)
		for rows.Next() {
			var h skillHistoryRow
			if err := rows.Scan(&h.ID, &h.SkillID, &h.Version, &h.Body,
				&h.ChangedBy, &h.Summary, &h.ChangedAt); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			history = append(history, h)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"history":  history,
			"skill_id": skillID,
			"count":    len(history),
		})
	}
}
