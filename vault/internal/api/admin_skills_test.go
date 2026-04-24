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
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("expected non-empty id")
	}
	if v, ok := resp["version"].(float64); !ok || v != 1 {
		t.Errorf("version = %v, want 1", resp["version"])
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

// TestAdminSaveSkill_VersionIncrement verifies that saving the same skill twice
// bumps the version from 1 → 2 and records two history rows.
func TestAdminSaveSkill_VersionIncrement(t *testing.T) {
	s := newTestStore(t)
	payload := map[string]string{"team_id": "team-x", "name": "VerSkill", "body": "first"}
	adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", payload)

	payload["body"] = "second"
	rec := adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("second save: %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if v, ok := resp["version"].(float64); !ok || v != 2 {
		t.Errorf("version = %v, want 2", resp["version"])
	}

	// Should have 2 history rows.
	var histCount int
	s.DB().QueryRow(
		`SELECT COUNT(*) FROM skill_history sh
		   JOIN skills sk ON sk.id = sh.skill_id
		  WHERE sk.name='VerSkill' AND sk.team_id='team-x'`,
	).Scan(&histCount)
	if histCount != 2 {
		t.Errorf("history rows = %d, want 2", histCount)
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
	if sk.Version != 1 {
		t.Errorf("version = %d, want 1 (default)", sk.Version)
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

// TestAdminListSkillHistory verifies the history endpoint returns all versions.
func TestAdminListSkillHistory(t *testing.T) {
	s := newTestStore(t)
	payload := map[string]string{"team_id": "team-h", "name": "HistSkill", "body": "body-v1", "summary": "initial"}
	adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", payload)
	payload["body"] = "body-v2"
	payload["summary"] = "second edit"
	adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", payload)

	var skillID string
	s.DB().QueryRow(`SELECT id FROM skills WHERE name='HistSkill' AND team_id='team-h'`).Scan(&skillID)

	req := httptest.NewRequest(http.MethodGet, "/admin/skills/"+skillID+"/history", nil)
	req.SetPathValue("id", skillID)
	rec := httptest.NewRecorder()
	adminListSkillHistory(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	count := int(resp["count"].(float64))
	if count != 2 {
		t.Errorf("history count = %d, want 2", count)
	}
	// Latest version first (DESC order)
	history := resp["history"].([]interface{})
	first := history[0].(map[string]any)
	if first["version"].(float64) != 2 {
		t.Errorf("first history version = %v, want 2", first["version"])
	}
}

func TestAdminSaveSkill_ScopeDefault(t *testing.T) {
	s := newTestStore(t)
	adminDo(t, adminSaveSkill(s, "admin"), http.MethodPost, "/admin/skills", map[string]string{
		"name": "ScopedSkill",
		"body": "body",
	})
	var scope string
	s.DB().QueryRow(`SELECT scope FROM skills WHERE name='ScopedSkill'`).Scan(&scope)
	if scope != "team" {
		t.Errorf("scope = %q, want 'team'", scope)
	}
}

func TestAdminSaveSkill_UpdatedByTracked(t *testing.T) {
	s := newTestStore(t)
	adminDo(t, adminSaveSkill(s, "alice"), http.MethodPost, "/admin/skills", map[string]string{
		"name": "AuthorSkill",
		"body": "some body",
	})
	var updatedBy string
	s.DB().QueryRow(`SELECT updated_by FROM skills WHERE name='AuthorSkill'`).Scan(&updatedBy)
	if updatedBy != "alice" {
		t.Errorf("updated_by = %q, want 'alice'", updatedBy)
	}
}
