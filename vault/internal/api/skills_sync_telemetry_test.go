package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTeamReportSkillSync(t *testing.T) {
	env := newTeamTestEnv(t)

	rr := env.do(t, "POST", "/team/skills/sync/report", env.memberToken,
		`{"skills_count":5,"target":"/home/dev/.claude"}`)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck
	if resp["status"] != "reported" {
		t.Errorf("expected status=reported, got %q", resp["status"])
	}
}

func TestAdminSkillsSyncStatus_Empty(t *testing.T) {
	s := newTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/skills/sync-status", nil)
	rr := httptest.NewRecorder()
	adminSkillsSyncStatus(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Count int `json:"count"`
	}
	json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck
	if resp.Count != 0 {
		t.Errorf("expected count=0, got %d", resp.Count)
	}
}

func TestAdminSkillsSyncStatus_ShowsEntries(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "SyncTeam")

	s.DB().Exec(`INSERT INTO skill_sync_log (id, team_id, user_email, synced_at, skills_count, target)
		VALUES ('sl-001', ?, 'alice@corp.com', '2026-04-01T10:00:00Z', 3, '/home/alice/.claude')`, teamID) //nolint:errcheck
	s.DB().Exec(`INSERT INTO skill_sync_log (id, team_id, user_email, synced_at, skills_count, target)
		VALUES ('sl-002', ?, 'bob@corp.com', '2026-03-15T08:00:00Z', 3, '/home/bob/.claude')`, teamID) //nolint:errcheck

	req := httptest.NewRequest(http.MethodGet, "/admin/skills/sync-status?team_id="+teamID, nil)
	rr := httptest.NewRecorder()
	adminSkillsSyncStatus(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Count int `json:"count"`
	}
	json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck
	if resp.Count != 2 {
		t.Errorf("expected 2 entries, got %d", resp.Count)
	}
}

func TestAdminSkillsSyncStatus_BehindDetected(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "BehindTeam")

	// Sync happened before the skill was updated.
	s.DB().Exec(`INSERT INTO skill_sync_log (id, team_id, user_email, synced_at, skills_count, target)
		VALUES ('sl-old', ?, 'dev@corp.com', '2026-01-01T00:00:00Z', 2, '/home/dev/.claude')`, teamID) //nolint:errcheck
	s.DB().Exec(`INSERT INTO skills (id, team_id, name, body, updated_at)
		VALUES ('sk-new', ?, 'new-skill', 'body', '2026-04-01T00:00:00Z')`, teamID) //nolint:errcheck

	req := httptest.NewRequest(http.MethodGet, "/admin/skills/sync-status?team_id="+teamID, nil)
	rr := httptest.NewRecorder()
	adminSkillsSyncStatus(s)(rr, req)

	var resp struct {
		Entries []struct {
			IsUpToDate bool `json:"is_up_to_date"`
		} `json:"entries"`
	}
	json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck
	if len(resp.Entries) == 0 {
		t.Fatal("expected 1 entry")
	}
	if resp.Entries[0].IsUpToDate {
		t.Error("expected is_up_to_date=false when sync is older than latest skill")
	}
}

func TestAdminSkillsSyncStatus_UpToDateDetected(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "UpToDateTeam")

	// Skill updated, then developer synced after.
	s.DB().Exec(`INSERT INTO skills (id, team_id, name, body, updated_at)
		VALUES ('sk-a', ?, 'skill-a', 'body', '2026-03-01T00:00:00Z')`, teamID) //nolint:errcheck
	s.DB().Exec(`INSERT INTO skill_sync_log (id, team_id, user_email, synced_at, skills_count, target)
		VALUES ('sl-new', ?, 'dev@corp.com', '2026-04-01T00:00:00Z', 1, '/home/dev/.claude')`, teamID) //nolint:errcheck

	req := httptest.NewRequest(http.MethodGet, "/admin/skills/sync-status?team_id="+teamID, nil)
	rr := httptest.NewRecorder()
	adminSkillsSyncStatus(s)(rr, req)

	var resp struct {
		Entries []struct {
			IsUpToDate bool `json:"is_up_to_date"`
		} `json:"entries"`
	}
	json.NewDecoder(rr.Body).Decode(&resp) //nolint:errcheck
	if len(resp.Entries) == 0 {
		t.Fatal("expected 1 entry")
	}
	if !resp.Entries[0].IsUpToDate {
		t.Error("expected is_up_to_date=true when sync is newer than latest skill")
	}
}
