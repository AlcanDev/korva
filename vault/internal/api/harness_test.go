package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

// Phase 14.2 — REST endpoint tests. Layered security mandate, so
// coverage spans:
//
//   - Auth: 401 without session token; 401 with invalid token.
//   - Multi-tenant isolation: cross-team reads return 404 (NOT 403)
//     indistinguishable from "doesn't exist".
//   - Input validation: empty / over-long / control-char inputs rejected.
//   - Limit clamping: ?limit= echoed and bounded to [1, 1000].
//   - Audit: every read writes to audit_logs.
//
// All tests use the shared teamTestEnv from team_test.go, which spins
// up a router with two members (admin + member) under the same team.

// seedHarnessRow inserts a snapshot + transition for the given team
// directly into the test store, bypassing MCP. Used to set up state
// for read-endpoint assertions.
func seedHarnessRow(t *testing.T, e *teamTestEnv, team, project, root, status string) {
	t.Helper()
	if err := e.store.SaveHarnessSnapshot(team, project, root,
		`{"project":"`+project+`","features":[{"id":1,"name":"f","status":"`+status+`"}]}`); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	if err := e.store.RecordHarnessTransition(store.HarnessTransition{
		TeamID:     team,
		Project:    project,
		Root:       root,
		FeatureID:  1,
		FromStatus: "pending",
		ToStatus:   status,
		Owner:      "alice",
	}); err != nil {
		t.Fatalf("seed transition: %v", err)
	}
}

// auditCount returns how many audit_logs rows match the given action.
func auditCount(t *testing.T, e *teamTestEnv, action string) int {
	t.Helper()
	var n int
	if err := e.store.DB().QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE action = ?`, action).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	return n
}

// ── auth ────────────────────────────────────────────────────────────────────

func TestHarnessListProjects_NoTokenReturns401(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/harness/projects", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d — %s", w.Code, w.Body.String())
	}
}

func TestHarnessListProjects_InvalidTokenReturns401(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/harness/projects", "definitely-not-a-real-token", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHarnessGetProject_NoTokenReturns401(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/harness/projects/p?root=/r", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHarnessListTransitions_NoTokenReturns401(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/harness/transitions", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// ── happy path ──────────────────────────────────────────────────────────────

func TestHarnessListProjects_ScopedToCallerTeam(t *testing.T) {
	e := newTeamTestEnv(t)
	seedHarnessRow(t, e, e.teamID, "p1", "/r1", "in_progress")
	seedHarnessRow(t, e, "other-team", "p2", "/r2", "in_progress")

	w := e.do(t, http.MethodGet, "/api/v1/harness/projects", e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp struct {
		Projects []store.HarnessProjectSummary `json:"projects"`
		Count    int                           `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 1 || len(resp.Projects) != 1 {
		t.Fatalf("expected 1 row scoped to caller team, got %d (%+v)", resp.Count, resp.Projects)
	}
	if resp.Projects[0].Project != "p1" {
		t.Errorf("returned other team's project: %s", resp.Projects[0].Project)
	}
}

func TestHarnessGetProject_ReturnsSnapshot(t *testing.T) {
	e := newTeamTestEnv(t)
	seedHarnessRow(t, e, e.teamID, "auth_layer", "/repo", "spec_ready")

	w := e.do(t, http.MethodGet, "/api/v1/harness/projects/auth_layer?root=/repo", e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var snap store.HarnessSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if snap.Project != "auth_layer" {
		t.Errorf("project = %q", snap.Project)
	}
	if !strings.Contains(snap.Payload, "spec_ready") {
		t.Errorf("payload missing status, got %s", snap.Payload)
	}
}

func TestHarnessListTransitions_ScopedToCallerTeam(t *testing.T) {
	e := newTeamTestEnv(t)
	seedHarnessRow(t, e, e.teamID, "p1", "/r", "in_progress")
	seedHarnessRow(t, e, "other-team", "p2", "/r", "in_progress")

	w := e.do(t, http.MethodGet, "/api/v1/harness/transitions", e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp struct {
		Transitions []store.HarnessTransition `json:"transitions"`
		Count       int                       `json:"count"`
		Limit       int                       `json:"limit"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Count != 1 {
		t.Errorf("count = %d, want 1", resp.Count)
	}
	if resp.Transitions[0].TeamID != e.teamID {
		t.Errorf("returned other team's transition")
	}
	if resp.Limit != 100 {
		t.Errorf("default limit = %d, want 100", resp.Limit)
	}
}

func TestHarnessListTransitions_ProjectFilter(t *testing.T) {
	e := newTeamTestEnv(t)
	seedHarnessRow(t, e, e.teamID, "alpha", "/r", "in_progress")
	seedHarnessRow(t, e, e.teamID, "beta", "/r", "in_progress")

	w := e.do(t, http.MethodGet, "/api/v1/harness/transitions?project=alpha", e.memberToken, "")
	var resp struct {
		Transitions []store.HarnessTransition `json:"transitions"`
		Count       int                       `json:"count"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Count != 1 || resp.Transitions[0].Project != "alpha" {
		t.Errorf("project filter broken: %+v", resp)
	}
}

// ── multi-tenant isolation (anti-enumeration) ──────────────────────────────

func TestHarnessGetProject_CrossTeamReturns404(t *testing.T) {
	// other-team has the snapshot; e.memberToken is in e.teamID. The
	// response must look identical to "doesn't exist" so an attacker
	// can't enumerate other teams' project names.
	e := newTeamTestEnv(t)
	seedHarnessRow(t, e, "other-team", "secret", "/r", "in_progress")

	w := e.do(t, http.MethodGet, "/api/v1/harness/projects/secret?root=/r", e.memberToken, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-team must return 404, got %d (%s)", w.Code, w.Body.String())
	}
	// Body must equal the "truly missing" body so client can't diff.
	wMissing := e.do(t, http.MethodGet, "/api/v1/harness/projects/never?root=/r", e.memberToken, "")
	if wMissing.Code != http.StatusNotFound {
		t.Errorf("missing project should also be 404, got %d", wMissing.Code)
	}
	if w.Body.String() != wMissing.Body.String() {
		t.Errorf("404 bodies must match (anti-enum):\n  cross-team: %s\n  missing:    %s",
			w.Body.String(), wMissing.Body.String())
	}
}

func TestHarnessListProjects_EmptyForTeamWithoutData(t *testing.T) {
	e := newTeamTestEnv(t)
	// Seed only `other-team` data — caller's team should see empty.
	seedHarnessRow(t, e, "other-team", "p", "/r", "in_progress")

	w := e.do(t, http.MethodGet, "/api/v1/harness/projects", e.memberToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp struct {
		Projects []store.HarnessProjectSummary `json:"projects"`
		Count    int                           `json:"count"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Count != 0 {
		t.Errorf("expected empty list for team without data, got %d", resp.Count)
	}
	// Response must be a JSON array, not null — Beacon expects []
	if !strings.Contains(w.Body.String(), `"projects"`) {
		t.Errorf("missing projects key: %s", w.Body.String())
	}
}

// ── input validation ───────────────────────────────────────────────────────

func TestHarnessGetProject_RejectsMissingRoot(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/harness/projects/p", e.memberToken, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHarnessGetProject_RejectsOverLongIdentifier(t *testing.T) {
	e := newTeamTestEnv(t)
	long := strings.Repeat("x", harnessIdentifierMax+1)
	w := e.do(t, http.MethodGet, "/api/v1/harness/projects/p?root="+long, e.memberToken, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for over-long root, got %d", w.Code)
	}
	w = e.do(t, http.MethodGet, "/api/v1/harness/projects/"+long+"?root=/r", e.memberToken, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for over-long project, got %d", w.Code)
	}
}

func TestHarnessGetProject_RejectsControlChars(t *testing.T) {
	e := newTeamTestEnv(t)
	// %00 (NUL) and %0A (LF) must be rejected — defense against log
	// injection + malformed clients.
	w := e.do(t, http.MethodGet, "/api/v1/harness/projects/p?root=%00bad", e.memberToken, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("NUL byte should be rejected, got %d", w.Code)
	}
	w = e.do(t, http.MethodGet, "/api/v1/harness/projects/p?root=line%0Abad", e.memberToken, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("LF byte should be rejected, got %d", w.Code)
	}
}

func TestHarnessListTransitions_RejectsBadProjectFilter(t *testing.T) {
	e := newTeamTestEnv(t)
	long := strings.Repeat("x", harnessIdentifierMax+1)
	w := e.do(t, http.MethodGet, "/api/v1/harness/transitions?project="+long, e.memberToken, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── limit clamping ─────────────────────────────────────────────────────────

func TestParseHarnessLimit_TableDriven(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"", 100},
		{"50", 50},
		{"0", 100},
		{"-5", 100},
		{"abc", 100},
		{"999999", 1000},
		{"  20  ", 20},
		{"1000", 1000},
		{"1001", 1000},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			if got := parseHarnessLimit(tc.raw); got != tc.want {
				t.Errorf("parseHarnessLimit(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

func TestHarnessListTransitions_LimitEchoedAndClamped(t *testing.T) {
	e := newTeamTestEnv(t)
	seedHarnessRow(t, e, e.teamID, "p", "/r", "in_progress")

	w := e.do(t, http.MethodGet, "/api/v1/harness/transitions?limit=99999", e.memberToken, "")
	var resp struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Limit != 1000 {
		t.Errorf("limit should clamp to 1000, got %d", resp.Limit)
	}

	w = e.do(t, http.MethodGet, "/api/v1/harness/transitions?limit=25", e.memberToken, "")
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Limit != 25 {
		t.Errorf("limit echo broken: %d", resp.Limit)
	}
}

// ── audit logging ──────────────────────────────────────────────────────────

func TestHarnessListProjects_AuditsOnSuccess(t *testing.T) {
	e := newTeamTestEnv(t)
	before := auditCount(t, e, "harness_list_projects")
	if w := e.do(t, http.MethodGet, "/api/v1/harness/projects", e.memberToken, ""); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	after := auditCount(t, e, "harness_list_projects")
	if after != before+1 {
		t.Errorf("audit not written: before=%d after=%d", before, after)
	}
}

func TestHarnessGetProject_AuditsOnSuccess(t *testing.T) {
	e := newTeamTestEnv(t)
	seedHarnessRow(t, e, e.teamID, "p", "/r", "in_progress")
	before := auditCount(t, e, "harness_get_project")
	if w := e.do(t, http.MethodGet, "/api/v1/harness/projects/p?root=/r", e.memberToken, ""); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	after := auditCount(t, e, "harness_get_project")
	if after != before+1 {
		t.Errorf("audit not written: before=%d after=%d", before, after)
	}
}

func TestHarnessListTransitions_AuditsOnSuccess(t *testing.T) {
	e := newTeamTestEnv(t)
	before := auditCount(t, e, "harness_list_transitions")
	if w := e.do(t, http.MethodGet, "/api/v1/harness/transitions", e.memberToken, ""); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	after := auditCount(t, e, "harness_list_transitions")
	if after != before+1 {
		t.Errorf("audit not written: before=%d after=%d", before, after)
	}
}

// ── content-type / response shape ──────────────────────────────────────────

func TestHarnessListProjects_ResponseShapeJSON(t *testing.T) {
	e := newTeamTestEnv(t)
	w := e.do(t, http.MethodGet, "/api/v1/harness/projects", e.memberToken, "")
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ── validHarnessIdentifier ─────────────────────────────────────────────────

func TestValidHarnessIdentifier_TableDriven(t *testing.T) {
	cases := []struct {
		s     string
		valid bool
	}{
		{"normal", true},
		{"path/with/slash", true},
		{"with-hyphen.dot_underscore", true},
		{"acentos-áéíóú", true},
		{"", false},
		{"\x00with-nul", false},
		{"line\nfeed", false},
		{"\ttab", false},
		{"del\x7f", false},
		{strings.Repeat("a", harnessIdentifierMax+1), false},
		{strings.Repeat("a", harnessIdentifierMax), true},
	}
	for _, tc := range cases {
		tc := tc
		name := tc.s
		if name == "" {
			name = "empty"
		} else if len(name) > 30 {
			name = name[:10] + "..."
		}
		t.Run(name, func(t *testing.T) {
			if got := validHarnessIdentifier(tc.s); got != tc.valid {
				t.Errorf("validHarnessIdentifier(%q) = %v, want %v", tc.s, got, tc.valid)
			}
		})
	}
}
