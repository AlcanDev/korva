package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func insertSkill(t *testing.T, s *store.Store, teamID, name, body string) string {
	t.Helper()
	id := newID()
	if _, err := s.DB().Exec(
		`INSERT INTO skills(id, team_id, name, body, tags) VALUES(?,?,?,?,?)`,
		id, teamID, name, body, "[]"); err != nil {
		t.Fatalf("insert skill: %v", err)
	}
	return id
}

func TestAdminListSkills_Empty(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminListSkills(s), http.MethodGet, "/admin/skills", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	skills := resp["skills"].([]interface{})
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestAdminSaveSkill_Success(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", map[string]string{
		"name": "NestJS Module",
		"body": "Always use @Module decorator with providers array",
		"tags": `["nestjs","backend"]`,
	})
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("expected non-empty id")
	}
}

func TestAdminSaveSkill_MissingName(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", map[string]string{
		"body": "some body",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminSaveSkill_Upsert(t *testing.T) {
	s := newTestStore(t)
	payload := map[string]string{
		"team_id": "team-1",
		"name":    "MySkill",
		"body":    "v1",
	}
	adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", payload)
	// Update same name/team
	payload["body"] = "v2"
	rec := adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", payload)
	if rec.Code != http.StatusCreated {
		t.Errorf("upsert status = %d, want 201", rec.Code)
	}
	// Verify only 1 row with updated body
	var count int
	s.DB().QueryRow(`SELECT COUNT(*) FROM skills WHERE name='MySkill' AND team_id='team-1'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 skill after upsert, got %d", count)
	}
}

func TestAdminGetSkill_Success(t *testing.T) {
	s := newTestStore(t)
	// Insert directly
	id := newID()
	s.DB().Exec(`INSERT INTO skills(id, team_id, name, body, tags) VALUES(?,?,?,?,?)`,
		id, "team-1", "Go Patterns", "Use table-driven tests", "[]")

	req := httptest.NewRequest(http.MethodGet, "/admin/skills/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	adminGetSkill(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var sk skillRow
	json.NewDecoder(rec.Body).Decode(&sk)
	if sk.Name != "Go Patterns" {
		t.Errorf("name = %q, want 'Go Patterns'", sk.Name)
	}
}

func TestAdminGetSkill_NotFound(t *testing.T) {
	s := newTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/skills/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()
	adminGetSkill(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAdminListSkills_WithTeamFilter(t *testing.T) {
	s := newTestStore(t)
	s.DB().Exec(`INSERT INTO skills(id, team_id, name, body, tags) VALUES(?,?,?,?,?)`,
		newID(), "team-A", "SkillA", "bodyA", "[]")
	s.DB().Exec(`INSERT INTO skills(id, team_id, name, body, tags) VALUES(?,?,?,?,?)`,
		newID(), "team-B", "SkillB", "bodyB", "[]")

	req := httptest.NewRequest(http.MethodGet, "/admin/skills?team_id=team-A", nil)
	rec := httptest.NewRecorder()
	adminListSkills(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	skills := resp["skills"].([]interface{})
	if len(skills) != 1 {
		t.Errorf("expected 1 skill for team-A, got %d", len(skills))
	}
}

func TestAdminDeleteSkill_Success(t *testing.T) {
	s := newTestStore(t)
	id := newID()
	s.DB().Exec(`INSERT INTO skills(id, team_id, name, body, tags) VALUES(?,?,?,?,?)`,
		id, "team-1", "ToDelete", "body", "[]")

	req := httptest.NewRequest(http.MethodDelete, "/admin/skills/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	adminDeleteSkill(s, "admin").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "deleted" {
		t.Errorf("status = %q, want 'deleted'", resp["status"])
	}
}

func TestAdminDeleteSkill_NotFound(t *testing.T) {
	s := newTestStore(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin/skills/nope", nil)
	req.SetPathValue("id", "nope")
	rec := httptest.NewRecorder()
	adminDeleteSkill(s, "admin").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAdminSaveSkill_DefaultTags(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", map[string]string{
		"name": "NoTagSkill",
		"body": "just a body",
		// tags omitted — should default to "[]"
	})
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}
