package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// adminDo performs an authenticated request against the given handler.
func adminDo(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// createTeamViaDB inserts a team directly and returns its ID.
func createTeamViaDB(t *testing.T, s *store.Store, name string) string {
	t.Helper()
	id := newID()
	_, err := s.DB().Exec(`INSERT INTO teams(id, name, owner, license_id) VALUES(?,?,?,?)`, id, name, "owner@test.com", "lic-1")
	if err != nil {
		t.Fatalf("insert team: %v", err)
	}
	return id
}

func TestAdminListTeams_Empty(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminListTeams(s), http.MethodGet, "/admin/teams", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	teams := resp["teams"].([]interface{})
	if len(teams) != 0 {
		t.Errorf("expected empty teams, got %d", len(teams))
	}
}

func TestAdminListTeams_WithData(t *testing.T) {
	s := newTestStore(t)
	createTeamViaDB(t, s, "Alpha")
	createTeamViaDB(t, s, "Beta")

	rec := adminDo(t, adminListTeams(s), http.MethodGet, "/admin/teams", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	teams := resp["teams"].([]interface{})
	if len(teams) != 2 {
		t.Errorf("expected 2 teams, got %d", len(teams))
	}
}

func TestAdminCreateTeam_Success(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminCreateTeam(s, "admin"), http.MethodPost, "/admin/teams", map[string]string{
		"name":  "Test Team",
		"owner": "owner@test.com",
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

func TestAdminCreateTeam_MissingName(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminCreateTeam(s, "admin"), http.MethodPost, "/admin/teams", map[string]string{
		"owner": "owner@test.com",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminListMembers_Empty(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "MyTeam")

	req := httptest.NewRequest(http.MethodGet, "/admin/teams/"+teamID+"/members", nil)
	req.SetPathValue("team_id", teamID)
	rec := httptest.NewRecorder()
	adminListMembers(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	members := resp["members"].([]interface{})
	if len(members) != 0 {
		t.Errorf("expected 0 members, got %d", len(members))
	}
}

func TestAdminAddMember_Success(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "MyTeam")

	req := httptest.NewRequest(http.MethodPost, "/admin/teams/"+teamID+"/members",
		bytes.NewReader(mustJSON(map[string]string{"email": "alice@example.com", "role": "member"})))
	req.SetPathValue("team_id", teamID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	adminAddMember(s, "admin", nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminAddMember_MissingEmail(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "MyTeam")

	req := httptest.NewRequest(http.MethodPost, "/admin/teams/"+teamID+"/members",
		bytes.NewReader(mustJSON(map[string]string{"role": "admin"})))
	req.SetPathValue("team_id", teamID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	adminAddMember(s, "admin", nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAdminAddMember_DefaultRole(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "MyTeam")

	req := httptest.NewRequest(http.MethodPost, "/admin/teams/"+teamID+"/members",
		bytes.NewReader(mustJSON(map[string]string{"email": "bob@example.com"})))
	req.SetPathValue("team_id", teamID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	adminAddMember(s, "admin", nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

func TestAdminRemoveMember_Success(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "MyTeam")
	memberID := newID()
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		memberID, teamID, "carol@example.com", "member")

	req := httptest.NewRequest(http.MethodDelete, "/admin/teams/"+teamID+"/members/"+memberID, nil)
	req.SetPathValue("team_id", teamID)
	req.SetPathValue("member_id", memberID)
	rec := httptest.NewRecorder()
	adminRemoveMember(s, "admin").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRemoveMember_NotFound(t *testing.T) {
	s := newTestStore(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin/teams/t1/members/nonexistent", nil)
	req.SetPathValue("team_id", "t1")
	req.SetPathValue("member_id", "nonexistent")
	rec := httptest.NewRecorder()
	adminRemoveMember(s, "admin").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAdminActiveProfile_NoTeam(t *testing.T) {
	s := newTestStore(t)
	rec := adminDo(t, adminActiveProfile(s), http.MethodGet, "/admin/teams/profile/active", nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestAdminActiveProfile_WithTeam(t *testing.T) {
	s := newTestStore(t)
	createTeamViaDB(t, s, "ProdTeam")

	rec := adminDo(t, adminActiveProfile(s), http.MethodGet, "/admin/teams/profile/active", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["team"] == nil {
		t.Error("expected team key in response")
	}
}

func TestNewID_Unique(t *testing.T) {
	ids := map[string]bool{}
	for i := 0; i < 20; i++ {
		id := newID()
		if len(id) != 16 {
			t.Errorf("id length = %d, want 16", len(id))
		}
		if ids[id] {
			t.Errorf("duplicate id: %s", id)
		}
		ids[id] = true
	}
}

func TestAdminListMembers_WithMembers(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "MyTeam")
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		newID(), teamID, "alice@example.com", "admin")
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		newID(), teamID, "bob@example.com", "member")

	req := httptest.NewRequest(http.MethodGet, "/admin/teams/"+teamID+"/members", nil)
	req.SetPathValue("team_id", teamID)
	rec := httptest.NewRecorder()
	adminListMembers(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	members := resp["members"].([]interface{})
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestAdminActiveProfile_WithMembers(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "FullTeam")
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		newID(), teamID, "lead@corp.com", "admin")

	rec := adminDo(t, adminActiveProfile(s), http.MethodGet, "/admin/teams/profile/active", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	members, _ := resp["members"].([]interface{})
	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
