package api

import (
	"encoding/json"
	"net/http"

	"github.com/alcandev/korva/vault/internal/store"
)

type skillRow struct {
	ID        string `json:"id"`
	TeamID    string `json:"team_id"`
	Name      string `json:"name"`
	Body      string `json:"body"`
	Tags      string `json:"tags"`
	Version   int    `json:"version"`
	UpdatedBy string `json:"updated_by"`
	Scope     string `json:"scope"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type skillHistoryRow struct {
	ID        string `json:"id"`
	SkillID   string `json:"skill_id"`
	Version   int    `json:"version"`
	Body      string `json:"body"`
	ChangedBy string `json:"changed_by"`
	Summary   string `json:"summary"`
	ChangedAt string `json:"changed_at"`
}

func adminListSkills(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSkillList(w, r, s, r.URL.Query().Get("team_id"))
	}
}

func writeSkillList(w http.ResponseWriter, r *http.Request, s *store.Store, teamID string) {
	var (
		sqlRows interface {
			Next() bool
			Scan(...any) error
			Close() error
		}
		err error
	)
	if teamID != "" {
		sqlRows, err = s.DB().QueryContext(r.Context(),
			`SELECT id, team_id, name, body, tags, version, updated_by, scope, created_at, updated_at
			   FROM skills WHERE team_id=? ORDER BY name`, teamID)
	} else {
		sqlRows, err = s.DB().QueryContext(r.Context(),
			`SELECT id, team_id, name, body, tags, version, updated_by, scope, created_at, updated_at
			   FROM skills ORDER BY team_id, name`)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer sqlRows.Close()
	var skills []skillRow
	for sqlRows.Next() {
		var sk skillRow
		if err := sqlRows.Scan(&sk.ID, &sk.TeamID, &sk.Name, &sk.Body,
			&sk.Tags, &sk.Version, &sk.UpdatedBy, &sk.Scope,
			&sk.CreatedAt, &sk.UpdatedAt); err != nil {
			continue
		}
		skills = append(skills, sk)
	}
	if skills == nil {
		skills = []skillRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills, "count": len(skills)})
}

func adminGetSkill(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var sk skillRow
		err := s.DB().QueryRowContext(r.Context(),
			`SELECT id, team_id, name, body, tags, version, updated_by, scope, created_at, updated_at
			   FROM skills WHERE id=?`, id).
			Scan(&sk.ID, &sk.TeamID, &sk.Name, &sk.Body, &sk.Tags,
				&sk.Version, &sk.UpdatedBy, &sk.Scope, &sk.CreatedAt, &sk.UpdatedAt)
		if err != nil {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}
		writeJSON(w, http.StatusOK, sk)
	}
}

func adminSaveSkill(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			TeamID  string `json:"team_id"`
			Name    string `json:"name"`
			Body    string `json:"body"`
			Tags    string `json:"tags"`
			Scope   string `json:"scope"`
			Summary string `json:"summary"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, http.StatusBadRequest, "name and body are required")
			return
		}
		if body.Tags == "" {
			body.Tags = "[]"
		}
		if body.Scope == "" {
			body.Scope = "team"
		}

		tx, err := s.DB().BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer tx.Rollback() //nolint:errcheck

		id := newID()
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO skills(id, team_id, name, body, tags, scope, updated_by, version)
			   VALUES(?,?,?,?,?,?,?,1)
			   ON CONFLICT(team_id, name) DO UPDATE SET
			     body       = excluded.body,
			     tags       = excluded.tags,
			     scope      = excluded.scope,
			     updated_by = excluded.updated_by,
			     version    = skills.version + 1,
			     updated_at = datetime('now')`,
			id, body.TeamID, body.Name, body.Body, body.Tags, body.Scope, actor,
		); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var actualID string
		var version int
		if err := tx.QueryRowContext(r.Context(),
			`SELECT id, version FROM skills WHERE team_id=? AND name=?`,
			body.TeamID, body.Name,
		).Scan(&actualID, &version); err != nil {
			writeError(w, http.StatusInternalServerError, "resolving skill: "+err.Error())
			return
		}

		histID := newID()
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO skill_history(id, skill_id, version, body, changed_by, summary)
			   VALUES(?,?,?,?,?,?)`,
			histID, actualID, version, body.Body, actor, body.Summary,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "recording history: "+err.Error())
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeAudit(s, actor, "save_skill", actualID, "", hashStr(body.Body))
		writeJSON(w, http.StatusCreated, map[string]any{"status": "saved", "id": actualID, "version": version})
	}
}

func adminDeleteSkill(s *store.Store, actor string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var name string
		_ = s.DB().QueryRowContext(r.Context(), `SELECT name FROM skills WHERE id=?`, id).Scan(&name)
		res, err := s.DB().ExecContext(r.Context(), `DELETE FROM skills WHERE id=?`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}
		writeAudit(s, actor, "delete_skill", id, hashStr(name), "")
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// adminListSkillHistory returns the version history for a skill by ID.
// GET /admin/skills/{id}/history
func adminListSkillHistory(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		skillID := r.PathValue("id")
		rows, err := s.DB().QueryContext(r.Context(),
			`SELECT id, skill_id, version, body, changed_by, summary, changed_at
			   FROM skill_history
			  WHERE skill_id=?
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
