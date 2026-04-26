package api

// team_skills_sync_test.go — tests for GET /team/skills/sync

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestTeamSyncSkills_FullSync returns all skills when ?since is absent.
func TestTeamSyncSkills_FullSync(t *testing.T) {
	e := newTeamTestEnv(t)
	e.do(t, http.MethodPost, "/team/skills", e.adminToken, `{"name":"skill-a","body":"body-a"}`)
	e.do(t, http.MethodPost, "/team/skills", e.adminToken, `{"name":"skill-b","body":"body-b"}`)

	w := e.do(t, http.MethodGet, "/team/skills/sync", e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	count := int(resp["count"].(float64))
	if count != 2 {
		t.Errorf("full sync count = %d, want 2", count)
	}
	if resp["synced_at"] == "" {
		t.Error("synced_at must be non-empty")
	}
}

// TestTeamSyncSkills_Differential returns only skills changed after ?since.
// Uses direct DB inserts with explicit timestamps to avoid second-precision races.
func TestTeamSyncSkills_Differential(t *testing.T) {
	e := newTeamTestEnv(t)

	oldTS := "2026-01-01T00:00:00Z"
	newTS := "2026-04-24T12:00:00Z"
	since := "2026-03-01T00:00:00Z" // between old and new

	// Insert skills directly with controlled timestamps.
	oldID, newID2 := newID(), newID()
	e.store.DB().Exec(
		`INSERT INTO skills(id, team_id, name, body, tags, updated_at, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		oldID, e.teamID, "old-skill", "old body", "[]", oldTS, oldTS)
	e.store.DB().Exec(
		`INSERT INTO skills(id, team_id, name, body, tags, updated_at, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		newID2, e.teamID, "new-skill", "new body", "[]", newTS, newTS)

	w := e.do(t, http.MethodGet, "/team/skills/sync?since="+since, e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	count := int(resp["count"].(float64))
	if count != 1 {
		t.Errorf("differential sync count = %d, want 1", count)
	}
	skills := resp["skills"].([]interface{})
	name := skills[0].(map[string]any)["name"].(string)
	if name != "new-skill" {
		t.Errorf("synced skill = %q, want 'new-skill'", name)
	}
}

// TestTeamSyncSkills_IncludesVersionAndMetadata verifies the payload has all fields.
func TestTeamSyncSkills_IncludesVersionAndMetadata(t *testing.T) {
	e := newTeamTestEnv(t)
	e.do(t, http.MethodPost, "/team/skills", e.adminToken,
		`{"name":"meta-skill","body":"body","scope":"org"}`)
	// Update once to bump version.
	e.do(t, http.MethodPost, "/team/skills", e.adminToken,
		`{"name":"meta-skill","body":"body v2"}`)

	w := e.do(t, http.MethodGet, "/team/skills/sync", e.memberToken, "")
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	skills := resp["skills"].([]interface{})
	if len(skills) == 0 {
		t.Fatal("expected skills in response")
	}
	sk := skills[0].(map[string]any)
	if sk["version"].(float64) != 2 {
		t.Errorf("version = %v, want 2", sk["version"])
	}
	if sk["updated_by"] != "admin@corp.com" {
		t.Errorf("updated_by = %v, want 'admin@corp.com'", sk["updated_by"])
	}
	if sk["deleted"] != false {
		t.Errorf("deleted should be false for active skill, got %v", sk["deleted"])
	}
	if sk["body"] == nil {
		t.Error("body must be present in sync payload")
	}
}

// TestTeamSyncSkills_DeletedSkillsAppearsAsStub verifies that after a skill is
// deleted, a differential sync returns a stub row with deleted=true.
// Uses a past timestamp as ?since to avoid second-precision races.
func TestTeamSyncSkills_DeletedSkillsAppearsAsStub(t *testing.T) {
	e := newTeamTestEnv(t)

	// Create skill via API (records audit log entry on delete).
	wSave := e.do(t, http.MethodPost, "/team/skills", e.adminToken,
		`{"name":"soon-deleted","body":"x"}`)
	var saveResp map[string]any
	json.NewDecoder(wSave.Body).Decode(&saveResp)
	skillID := saveResp["id"].(string)

	// Use a ?since well in the past so the delete audit entry is included.
	since := "2020-01-01T00:00:00Z"

	// Delete the skill.
	e.do(t, http.MethodDelete, "/team/skills/"+skillID, e.adminToken, "")

	// Differential sync should include a deleted stub.
	w := e.do(t, http.MethodGet, "/team/skills/sync?since="+since, e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	skills := resp["skills"].([]interface{})
	var foundDeleted bool
	for _, s := range skills {
		row := s.(map[string]any)
		if row["id"] == skillID && row["deleted"] == true {
			foundDeleted = true
			break
		}
	}
	if !foundDeleted {
		t.Errorf("expected deleted stub for skill %s in sync response", skillID)
	}
}

// TestTeamSyncSkills_InvalidSince treats unparseable ?since as full sync.
func TestTeamSyncSkills_InvalidSince(t *testing.T) {
	e := newTeamTestEnv(t)
	e.do(t, http.MethodPost, "/team/skills", e.adminToken, `{"name":"s1","body":"b1"}`)

	w := e.do(t, http.MethodGet, "/team/skills/sync?since=not-a-date", e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	// Invalid since = full sync → should return all skills.
	if resp["count"].(float64) < 1 {
		t.Errorf("invalid since should fall back to full sync, got count=%v", resp["count"])
	}
}

// TestTeamSyncSkills_NoToken returns 401.
func TestTeamSyncSkills_NoToken(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/team/skills/sync", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}
