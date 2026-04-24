package api

// team_skills_test.go — versioning, history, and metadata tests for /team/skills.
// Basic CRUD and RBAC tests live in team_test.go.

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestTeamSaveSkill_VersionStartsAtOne ensures the first save gets version=1.
func TestTeamSaveSkill_VersionStartsAtOne(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodPost, "/team/skills", e.memberToken,
		`{"name":"v-skill","body":"initial body"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if v, ok := resp["version"].(float64); !ok || v != 1 {
		t.Errorf("version = %v, want 1", resp["version"])
	}
}

// TestTeamSaveSkill_VersionIncrements verifies version goes 1 → 2 on upsert.
func TestTeamSaveSkill_VersionIncrements(t *testing.T) {
	e := newTeamTestEnv(t)
	save := func(body string) map[string]any {
		w := e.do(t, http.MethodPost, "/team/skills", e.adminToken, body)
		if w.Code != http.StatusOK {
			t.Fatalf("save failed: %d — %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		return resp
	}

	r1 := save(`{"name":"versioned","body":"first"}`)
	if r1["version"].(float64) != 1 {
		t.Errorf("first save version = %v, want 1", r1["version"])
	}

	r2 := save(`{"name":"versioned","body":"second"}`)
	if r2["version"].(float64) != 2 {
		t.Errorf("second save version = %v, want 2", r2["version"])
	}
	if r1["id"] != r2["id"] {
		t.Errorf("upsert must keep same id: %v vs %v", r1["id"], r2["id"])
	}
}

// TestTeamSkillHistory_RecordsEachVersion checks that skill_history captures
// every save with the correct body and changedBy fields.
func TestTeamSkillHistory_RecordsEachVersion(t *testing.T) {
	e := newTeamTestEnv(t)

	save := func(body string) {
		w := e.do(t, http.MethodPost, "/team/skills", e.adminToken, body)
		if w.Code != http.StatusOK {
			t.Fatalf("save: %d — %s", w.Code, w.Body.String())
		}
	}
	save(`{"name":"hist-skill","body":"v1 body","summary":"initial"}`)
	save(`{"name":"hist-skill","body":"v2 body","summary":"fix typo"}`)
	save(`{"name":"hist-skill","body":"v3 body","summary":"add detail"}`)

	// Get skill ID from the list.
	wList := e.do(t, http.MethodGet, "/team/skills", e.adminToken, "")
	var listResp map[string]any
	json.NewDecoder(wList.Body).Decode(&listResp)
	skills := listResp["skills"].([]interface{})
	skillID := skills[0].(map[string]any)["id"].(string)

	// Fetch history.
	wHist := e.do(t, http.MethodGet, "/team/skills/"+skillID+"/history", e.adminToken, "")
	if wHist.Code != http.StatusOK {
		t.Fatalf("history: want 200, got %d — %s", wHist.Code, wHist.Body.String())
	}
	var histResp map[string]any
	json.NewDecoder(wHist.Body).Decode(&histResp)

	count := int(histResp["count"].(float64))
	if count != 3 {
		t.Errorf("history count = %d, want 3", count)
	}

	// Returned in DESC order — first entry is latest version.
	history := histResp["history"].([]interface{})
	latest := history[0].(map[string]any)
	if latest["version"].(float64) != 3 {
		t.Errorf("latest version = %v, want 3", latest["version"])
	}
	if latest["body"] != "v3 body" {
		t.Errorf("latest body = %q, want 'v3 body'", latest["body"])
	}
	if latest["changed_by"] != "admin@corp.com" {
		t.Errorf("changed_by = %q, want 'admin@corp.com'", latest["changed_by"])
	}
	if latest["summary"] != "add detail" {
		t.Errorf("summary = %q, want 'add detail'", latest["summary"])
	}
}

// TestTeamSkillHistory_WrongTeam returns 404 when the skill doesn't belong to
// the session's team — prevents cross-team history leaks.
func TestTeamSkillHistory_WrongTeam(t *testing.T) {
	e := newTeamTestEnv(t)
	// Insert a skill that belongs to a different team directly.
	otherSkillID := newID()
	e.store.DB().Exec(
		`INSERT INTO skills(id, team_id, name, body, tags) VALUES(?,?,?,?,?)`,
		otherSkillID, "team-other-9999", "foreign-skill", "body", "[]",
	)

	wHist := e.do(t, http.MethodGet, "/team/skills/"+otherSkillID+"/history", e.memberToken, "")
	if wHist.Code != http.StatusNotFound {
		t.Errorf("cross-team history: want 404, got %d — %s", wHist.Code, wHist.Body.String())
	}
}

// TestTeamSkillList_IncludesVersionMetadata verifies the list endpoint exposes
// version, updated_by, and scope on each row.
func TestTeamSkillList_IncludesVersionMetadata(t *testing.T) {
	e := newTeamTestEnv(t)
	e.do(t, http.MethodPost, "/team/skills", e.adminToken,
		`{"name":"meta-skill","body":"body","scope":"org"}`)

	wList := e.do(t, http.MethodGet, "/team/skills", e.adminToken, "")
	var resp map[string]any
	json.NewDecoder(wList.Body).Decode(&resp)
	skills := resp["skills"].([]interface{})
	if len(skills) == 0 {
		t.Fatal("expected at least 1 skill")
	}
	sk := skills[0].(map[string]any)
	if sk["version"] == nil {
		t.Error("version field missing from list response")
	}
	if sk["updated_by"] == nil {
		t.Error("updated_by field missing from list response")
	}
	if sk["scope"] != "org" {
		t.Errorf("scope = %v, want 'org'", sk["scope"])
	}
	if sk["updated_by"] != "admin@corp.com" {
		t.Errorf("updated_by = %v, want 'admin@corp.com'", sk["updated_by"])
	}
}

// TestTeamSkillHistory_EmptyForNewSkill verifies a new skill has exactly 1 history row.
func TestTeamSkillHistory_EmptyForNewSkill(t *testing.T) {
	e := newTeamTestEnv(t)
	wSave := e.do(t, http.MethodPost, "/team/skills", e.memberToken,
		`{"name":"fresh-skill","body":"brand new"}`)
	var saveResp map[string]any
	json.NewDecoder(wSave.Body).Decode(&saveResp)
	skillID := saveResp["id"].(string)

	wHist := e.do(t, http.MethodGet, "/team/skills/"+skillID+"/history", e.memberToken, "")
	if wHist.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", wHist.Code)
	}
	var resp map[string]any
	json.NewDecoder(wHist.Body).Decode(&resp)
	if resp["count"].(float64) != 1 {
		t.Errorf("new skill should have 1 history row, got %v", resp["count"])
	}
}

// TestTeamDeleteSkill_CascadesHistory verifies that deleting a skill also
// removes its history (ON DELETE CASCADE).
func TestTeamDeleteSkill_CascadesHistory(t *testing.T) {
	e := newTeamTestEnv(t)

	// Create + update (2 history rows).
	wSave := e.do(t, http.MethodPost, "/team/skills", e.adminToken,
		`{"name":"cascade-skill","body":"v1"}`)
	var saveResp map[string]any
	json.NewDecoder(wSave.Body).Decode(&saveResp)
	skillID := saveResp["id"].(string)
	e.do(t, http.MethodPost, "/team/skills", e.adminToken,
		`{"name":"cascade-skill","body":"v2"}`)

	// Delete the skill.
	e.do(t, http.MethodDelete, "/team/skills/"+skillID, e.adminToken, "")

	// History rows should be gone.
	var histCount int
	e.store.DB().QueryRow(
		`SELECT COUNT(*) FROM skill_history WHERE skill_id = ?`, skillID,
	).Scan(&histCount)
	if histCount != 0 {
		t.Errorf("expected 0 history rows after delete, got %d", histCount)
	}
}
