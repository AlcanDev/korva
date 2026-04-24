package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/vault/internal/store"
)

// teamTestEnv holds the fixtures needed to exercise /team/* endpoints.
type teamTestEnv struct {
	store        *store.Store
	handler      http.Handler
	teamID       string
	adminToken   string // plaintext session token for role=admin
	memberToken  string // plaintext session token for role=member
}

// newTeamTestEnv creates an in-memory store, one team, two members (admin + member),
// and session tokens for each. The router is built with a nil license so
// requireFeature always passes (no feature gate in test).
func newTeamTestEnv(t *testing.T) *teamTestEnv {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	db := s.DB()
	now := time.Now().UTC().Format(time.RFC3339)

	// Create team
	teamID := "team-test-001"
	db.Exec(`INSERT INTO teams(id, name, owner, created_at) VALUES(?,?,?,?)`,
		teamID, "Test Corp", "owner@corp.com", now)

	// Admin member
	adminMemberID := "member-admin-001"
	db.Exec(`INSERT INTO team_members(id, team_id, email, role, created_at) VALUES(?,?,?,?,?)`,
		adminMemberID, teamID, "admin@corp.com", "admin", now)

	// Regular member
	regularMemberID := "member-user-001"
	db.Exec(`INSERT INTO team_members(id, team_id, email, role, created_at) VALUES(?,?,?,?,?)`,
		regularMemberID, teamID, "dev@corp.com", "member", now)

	// Helper to create session tokens
	mkSession := func(memberID, email string) string {
		plain := fmt.Sprintf("tok-%s-%d", email, time.Now().UnixNano())
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(plain)))
		exp := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
		db.Exec(`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at)
			VALUES(?,?,?,?,?,?)`,
			newID(), teamID, memberID, email, hash, exp)
		return plain
	}

	adminToken := mkSession(adminMemberID, "admin@corp.com")
	memberToken := mkSession(regularMemberID, "dev@corp.com")

	// Router with a stub Teams license so withSession passes the license check (Rama 4).
	// The license is not verified via JWS in tests — we construct the struct directly.
	testLic := &license.License{
		LicenseID: "test-lic-001",
		Tier:      license.TierTeams,
		Features: []string{
			license.FeatureAdminSkills,
			license.FeaturePrivateScrolls,
			license.FeatureAuditLog,
		},
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour),
	}
	h := Router(s, RouterConfig{License: testLic})

	return &teamTestEnv{
		store:       s,
		handler:     h,
		teamID:      teamID,
		adminToken:  adminToken,
		memberToken: memberToken,
	}
}

// do executes an HTTP request against the team test router.
func (e *teamTestEnv) do(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var b *strings.Reader
	if body != "" {
		b = strings.NewReader(body)
	} else {
		b = strings.NewReader("")
	}
	r := httptest.NewRequest(method, path, b)
	if token != "" {
		r.Header.Set("X-Session-Token", token)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	return w
}

// ── /team/skills ─────────────────────────────────────────────────────────────

func TestTeamListSkills_NoToken(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/team/skills", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d — %s", w.Code, w.Body.String())
	}
}

func TestTeamListSkills_Empty(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/team/skills", e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["count"].(float64) != 0 {
		t.Errorf("want 0 skills, got %v", resp["count"])
	}
}

func TestTeamSaveSkill_Member(t *testing.T) {
	e := newTeamTestEnv(t)

	// A regular member can create a skill
	body := `{"name":"hex-boundaries","body":"Always respect hexagonal layer boundaries","tags":["architecture"]}`
	w := e.do(t, http.MethodPost, "/team/skills", e.memberToken, body)
	if w.Code != http.StatusOK {
		t.Fatalf("member save skill: want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "saved" {
		t.Errorf("want status=saved, got %v", resp["status"])
	}
	if resp["id"] == "" {
		t.Error("response must include an id")
	}

	// Skill must now appear in the list
	w2 := e.do(t, http.MethodGet, "/team/skills", e.memberToken, "")
	var listResp map[string]any
	json.NewDecoder(w2.Body).Decode(&listResp)
	if listResp["count"].(float64) != 1 {
		t.Errorf("want 1 skill after save, got %v", listResp["count"])
	}
}

func TestTeamSaveSkill_Upsert(t *testing.T) {
	e := newTeamTestEnv(t)

	save := func(body string) string {
		w := e.do(t, http.MethodPost, "/team/skills", e.adminToken, body)
		if w.Code != http.StatusOK {
			t.Fatalf("save: want 200, got %d", w.Code)
		}
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		return resp["id"].(string)
	}

	id1 := save(`{"name":"arch-rule","body":"v1"}`)
	id2 := save(`{"name":"arch-rule","body":"v2 — updated"}`)

	// Upsert should return the same ID
	if id1 != id2 {
		t.Errorf("upsert should keep same id: got %s vs %s", id1, id2)
	}

	// List should still have only 1 entry
	w := e.do(t, http.MethodGet, "/team/skills", e.adminToken, "")
	var listResp map[string]any
	json.NewDecoder(w.Body).Decode(&listResp)
	if listResp["count"].(float64) != 1 {
		t.Errorf("upsert must not duplicate: want 1, got %v", listResp["count"])
	}
}

func TestTeamDeleteSkill_MemberForbidden(t *testing.T) {
	e := newTeamTestEnv(t)

	// Create skill as admin
	w := e.do(t, http.MethodPost, "/team/skills", e.adminToken, `{"name":"s","body":"b"}`)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	id := resp["id"].(string)

	// Member attempts to delete → 403
	wDel := e.do(t, http.MethodDelete, "/team/skills/"+id, e.memberToken, "")
	if wDel.Code != http.StatusForbidden {
		t.Errorf("member delete: want 403, got %d — %s", wDel.Code, wDel.Body.String())
	}
}

func TestTeamDeleteSkill_Admin(t *testing.T) {
	e := newTeamTestEnv(t)

	// Create
	w := e.do(t, http.MethodPost, "/team/skills", e.adminToken, `{"name":"to-delete","body":"x"}`)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	id := resp["id"].(string)

	// Delete as admin
	wDel := e.do(t, http.MethodDelete, "/team/skills/"+id, e.adminToken, "")
	if wDel.Code != http.StatusOK {
		t.Errorf("admin delete: want 200, got %d — %s", wDel.Code, wDel.Body.String())
	}

	// List should be empty again
	wList := e.do(t, http.MethodGet, "/team/skills", e.adminToken, "")
	var listResp map[string]any
	json.NewDecoder(wList.Body).Decode(&listResp)
	if listResp["count"].(float64) != 0 {
		t.Errorf("want 0 after delete, got %v", listResp["count"])
	}
}

func TestTeamDeleteSkill_WrongTeam(t *testing.T) {
	e := newTeamTestEnv(t)

	// Create a skill for the test team
	w := e.do(t, http.MethodPost, "/team/skills", e.adminToken, `{"name":"mine","body":"b"}`)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	id := resp["id"].(string)

	// Create a second team + admin session
	db := e.store.DB()
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO teams(id, name, owner, created_at) VALUES(?,?,?,?)`, "team-other", "Other", "o@o.com", now)
	db.Exec(`INSERT INTO team_members(id, team_id, email, role, created_at) VALUES(?,?,?,?,?)`,
		"member-other", "team-other", "other@other.com", "admin", now)
	otherPlain := fmt.Sprintf("tok-other-%d", time.Now().UnixNano())
	otherHash := fmt.Sprintf("%x", sha256.Sum256([]byte(otherPlain)))
	exp := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at) VALUES(?,?,?,?,?,?)`,
		newID(), "team-other", "member-other", "other@other.com", otherHash, exp)

	// Other admin trying to delete a skill from team-test-001 → 404
	r := httptest.NewRequest(http.MethodDelete, "/team/skills/"+id, nil)
	r.Header.Set("X-Session-Token", otherPlain)
	wr := httptest.NewRecorder()
	e.handler.ServeHTTP(wr, r)
	if wr.Code != http.StatusNotFound {
		t.Errorf("cross-team delete: want 404, got %d — %s", wr.Code, wr.Body.String())
	}
}

// ── /team/scrolls ─────────────────────────────────────────────────────────────

func TestTeamListScrolls_Empty(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/team/scrolls", e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["count"].(float64) != 0 {
		t.Errorf("want 0 scrolls, got %v", resp["count"])
	}
}

func TestTeamSaveScroll_Member(t *testing.T) {
	e := newTeamTestEnv(t)

	body := `{"name":"arch-guide","content":"# Architecture\nUse hexagonal architecture."}`
	w := e.do(t, http.MethodPost, "/team/scrolls", e.memberToken, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("member save scroll: want 201, got %d — %s", w.Code, w.Body.String())
	}

	// Should appear in list
	wList := e.do(t, http.MethodGet, "/team/scrolls", e.memberToken, "")
	var resp map[string]any
	json.NewDecoder(wList.Body).Decode(&resp)
	if resp["count"].(float64) != 1 {
		t.Errorf("want 1 scroll, got %v", resp["count"])
	}
}

func TestTeamSaveScroll_UpdateExisting(t *testing.T) {
	e := newTeamTestEnv(t)

	// Create
	w1 := e.do(t, http.MethodPost, "/team/scrolls", e.adminToken, `{"name":"deploy-guide","content":"v1"}`)
	if w1.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d", w1.Code)
	}

	// Update (same name → 200 updated)
	w2 := e.do(t, http.MethodPost, "/team/scrolls", e.adminToken, `{"name":"deploy-guide","content":"v2"}`)
	if w2.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d — %s", w2.Code, w2.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w2.Body).Decode(&resp)
	if resp["status"] != "updated" {
		t.Errorf("want status=updated, got %v", resp["status"])
	}

	// Only one scroll exists
	wList := e.do(t, http.MethodGet, "/team/scrolls", e.adminToken, "")
	var listResp map[string]any
	json.NewDecoder(wList.Body).Decode(&listResp)
	if listResp["count"].(float64) != 1 {
		t.Errorf("want 1 after upsert, got %v", listResp["count"])
	}
}

func TestTeamDeleteScroll_MemberForbidden(t *testing.T) {
	e := newTeamTestEnv(t)

	// Create
	w := e.do(t, http.MethodPost, "/team/scrolls", e.adminToken, `{"name":"guide","content":"x"}`)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	id := resp["id"].(string)

	// Member delete → 403
	wDel := e.do(t, http.MethodDelete, "/team/scrolls/"+id, e.memberToken, "")
	if wDel.Code != http.StatusForbidden {
		t.Errorf("member delete scroll: want 403, got %d", wDel.Code)
	}
}

func TestTeamDeleteScroll_Admin(t *testing.T) {
	e := newTeamTestEnv(t)

	w := e.do(t, http.MethodPost, "/team/scrolls", e.adminToken, `{"name":"bye","content":"bye"}`)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	id := resp["id"].(string)

	wDel := e.do(t, http.MethodDelete, "/team/scrolls/"+id, e.adminToken, "")
	if wDel.Code != http.StatusOK {
		t.Errorf("admin delete scroll: want 200, got %d — %s", wDel.Code, wDel.Body.String())
	}

	wList := e.do(t, http.MethodGet, "/team/scrolls", e.adminToken, "")
	var listResp map[string]any
	json.NewDecoder(wList.Body).Decode(&listResp)
	if listResp["count"].(float64) != 0 {
		t.Errorf("want 0 after delete, got %v", listResp["count"])
	}
}

// ── RBAC: expired session ─────────────────────────────────────────────────────

func TestTeamRoute_ExpiredSession(t *testing.T) {
	e := newTeamTestEnv(t)
	db := e.store.DB()

	// Insert an expired session
	expiredPlain := "expired-session-token"
	expiredHash := fmt.Sprintf("%x", sha256.Sum256([]byte(expiredPlain)))
	exp := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339) // already expired
	db.Exec(`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at)
		VALUES(?,?,?,?,?,?)`,
		newID(), e.teamID, "member-admin-001", "admin@corp.com", expiredHash, exp)

	w := e.do(t, http.MethodGet, "/team/skills", expiredPlain, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expired session: want 401, got %d — %s", w.Code, w.Body.String())
	}
}

// ── auth/me returns role ──────────────────────────────────────────────────────

func TestAuthMe_ReturnsRole(t *testing.T) {
	e := newTeamTestEnv(t)

	// Admin token should return role=admin
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	r.Header.Set("X-Session-Token", e.adminToken)
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("auth/me: want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["role"] != "admin" {
		t.Errorf("admin token: want role=admin, got %v", resp["role"])
	}

	// Member token should return role=member
	r2 := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	r2.Header.Set("X-Session-Token", e.memberToken)
	w2 := httptest.NewRecorder()
	e.handler.ServeHTTP(w2, r2)
	var resp2 map[string]any
	json.NewDecoder(w2.Body).Decode(&resp2)
	if resp2["role"] != "member" {
		t.Errorf("member token: want role=member, got %v", resp2["role"])
	}
}
