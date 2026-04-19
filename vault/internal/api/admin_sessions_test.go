package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// insertTestSession inserts a row into member_sessions for testing.
func insertTestSession(t *testing.T, s *store.Store, id, teamID, memberID, email, tokenHash, expiresAt string) {
	t.Helper()
	_, err := s.DB().ExecContext(context.Background(),
		`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at)
		 VALUES(?,?,?,?,?,?)`,
		id, teamID, memberID, email, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("insertTestSession: %v", err)
	}
}

func futureExpiry() string {
	return time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
}

func pastExpiry() string {
	return time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
}

func TestAdminListTeamSessions_Empty(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "TeamA")

	req := httptest.NewRequest(http.MethodGet, "/admin/teams/"+teamID+"/sessions", nil)
	req.SetPathValue("team_id", teamID)
	rec := httptest.NewRecorder()
	adminListTeamSessions(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	sessions := resp["sessions"].([]interface{})
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
	if int(resp["count"].(float64)) != 0 {
		t.Errorf("expected count=0, got %v", resp["count"])
	}
}

func TestAdminListTeamSessions_WithActive(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "TeamB")
	memberID := newID()
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		memberID, teamID, "alice@example.com", "member")

	sessionID := newID()
	insertTestSession(t, s, sessionID, teamID, memberID, "alice@example.com", "hash-active-1", futureExpiry())

	req := httptest.NewRequest(http.MethodGet, "/admin/teams/"+teamID+"/sessions", nil)
	req.SetPathValue("team_id", teamID)
	rec := httptest.NewRecorder()
	adminListTeamSessions(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	sessions := resp["sessions"].([]interface{})
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	sess := sessions[0].(map[string]any)
	if sess["status"] != "active" {
		t.Errorf("status = %q, want %q", sess["status"], "active")
	}
	if sess["id"] != sessionID {
		t.Errorf("id = %q, want %q", sess["id"], sessionID)
	}
}

func TestAdminListTeamSessions_WithExpired(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "TeamC")
	memberID := newID()
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		memberID, teamID, "bob@example.com", "member")

	sessionID := newID()
	insertTestSession(t, s, sessionID, teamID, memberID, "bob@example.com", "hash-expired-1", pastExpiry())

	req := httptest.NewRequest(http.MethodGet, "/admin/teams/"+teamID+"/sessions", nil)
	req.SetPathValue("team_id", teamID)
	rec := httptest.NewRecorder()
	adminListTeamSessions(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	sessions := resp["sessions"].([]interface{})
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	sess := sessions[0].(map[string]any)
	if sess["status"] != "expired" {
		t.Errorf("status = %q, want %q", sess["status"], "expired")
	}
}

func TestAdminListTeamSessions_OnlyOwnTeam(t *testing.T) {
	s := newTestStore(t)
	teamA := createTeamViaDB(t, s, "TeamD-A")
	teamB := createTeamViaDB(t, s, "TeamD-B")

	// Two distinct members in teamA so the UNIQUE(email, team_id) constraint is satisfied
	memberA1 := newID()
	memberA2 := newID()
	memberB := newID()
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		memberA1, teamA, "charlie@example.com", "member")
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		memberA2, teamA, "charlie2@example.com", "member")
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		memberB, teamB, "diana@example.com", "member")

	insertTestSession(t, s, newID(), teamA, memberA1, "charlie@example.com", "hash-a-1", futureExpiry())
	insertTestSession(t, s, newID(), teamA, memberA2, "charlie2@example.com", "hash-a-2", pastExpiry())
	insertTestSession(t, s, newID(), teamB, memberB, "diana@example.com", "hash-b-1", futureExpiry())

	req := httptest.NewRequest(http.MethodGet, "/admin/teams/"+teamA+"/sessions", nil)
	req.SetPathValue("team_id", teamA)
	rec := httptest.NewRecorder()
	adminListTeamSessions(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	sessions := resp["sessions"].([]interface{})
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions for teamA, got %d", len(sessions))
	}
	// Ensure all returned sessions belong to teamA
	for _, raw := range sessions {
		sess := raw.(map[string]any)
		// Sessions have member_id and email fields; verify neither belongs to teamB member
		if sess["email"] == "diana@example.com" {
			t.Errorf("got teamB session in teamA response: %v", sess)
		}
	}
}

func TestAdminRevokeSession_Success(t *testing.T) {
	s := newTestStore(t)
	teamID := createTeamViaDB(t, s, "TeamE")
	memberID := newID()
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		memberID, teamID, "eve@example.com", "member")

	sessionID := newID()
	insertTestSession(t, s, sessionID, teamID, memberID, "eve@example.com", "hash-revoke-1", futureExpiry())

	req := httptest.NewRequest(http.MethodDelete, "/admin/teams/"+teamID+"/sessions/"+sessionID, nil)
	req.SetPathValue("team_id", teamID)
	req.SetPathValue("session_id", sessionID)
	rec := httptest.NewRecorder()
	adminRevokeSession(s, "admin").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "revoked" {
		t.Errorf("status = %q, want %q", resp["status"], "revoked")
	}

	// Verify the session was actually deleted
	var count int
	s.DB().QueryRow(`SELECT COUNT(*) FROM member_sessions WHERE id=?`, sessionID).Scan(&count)
	if count != 0 {
		t.Errorf("session still exists after revoke")
	}
}

func TestAdminRevokeSession_NotFound(t *testing.T) {
	s := newTestStore(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin/teams/t1/sessions/nonexistent", nil)
	req.SetPathValue("team_id", "t1")
	req.SetPathValue("session_id", "nonexistent")
	rec := httptest.NewRecorder()
	adminRevokeSession(s, "admin").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAdminRevokeSession_WrongTeam(t *testing.T) {
	s := newTestStore(t)
	teamA := createTeamViaDB(t, s, "TeamF-A")
	teamB := createTeamViaDB(t, s, "TeamF-B")

	memberID := newID()
	s.DB().Exec(`INSERT INTO team_members(id, team_id, email, role) VALUES(?,?,?,?)`,
		memberID, teamA, "frank@example.com", "member")

	sessionID := newID()
	insertTestSession(t, s, sessionID, teamA, memberID, "frank@example.com", "hash-wrong-team-1", futureExpiry())

	// Try to delete teamA's session using teamB's ID — should 404
	req := httptest.NewRequest(http.MethodDelete, "/admin/teams/"+teamB+"/sessions/"+sessionID, nil)
	req.SetPathValue("team_id", teamB)
	req.SetPathValue("session_id", sessionID)
	rec := httptest.NewRecorder()
	adminRevokeSession(s, "admin").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}

	// Verify the session was NOT deleted
	var count int
	s.DB().QueryRow(`SELECT COUNT(*) FROM member_sessions WHERE id=?`, sessionID).Scan(&count)
	if count != 1 {
		t.Errorf("session was incorrectly deleted by wrong team request")
	}
}
