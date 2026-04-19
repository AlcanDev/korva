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
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
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
			`SELECT id, team_id, name, body, tags, created_at, updated_at
			   FROM skills WHERE team_id=? ORDER BY name`, teamID)
	} else {
		sqlRows, err = s.DB().QueryContext(r.Context(),
			`SELECT id, team_id, name, body, tags, created_at, updated_at
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
			&sk.Tags, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
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
			`SELECT id, team_id, name, body, tags, created_at, updated_at FROM skills WHERE id=?`, id).
			Scan(&sk.ID, &sk.TeamID, &sk.Name, &sk.Body, &sk.Tags, &sk.CreatedAt, &sk.UpdatedAt)
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
			TeamID string `json:"team_id"`
			Name   string `json:"name"`
			Body   string `json:"body"`
			Tags   string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, http.StatusBadRequest, "name and body are required")
			return
		}
		if body.Tags == "" {
			body.Tags = "[]"
		}
		id := newID()
		_, err := s.DB().ExecContext(r.Context(),
			`INSERT INTO skills(id, team_id, name, body, tags)
			   VALUES(?,?,?,?,?)
			   ON CONFLICT(team_id, name) DO UPDATE SET
			     body=excluded.body, tags=excluded.tags,
			     updated_at=datetime('now')`,
			id, body.TeamID, body.Name, body.Body, body.Tags)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeAudit(s, actor, "save_skill", body.Name, "", hashStr(body.Body))
		writeJSON(w, http.StatusCreated, map[string]string{"status": "saved", "id": id})
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
