package mcp

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// setupSessionStore creates an in-memory store with a team, member, and a
// valid session token. Returns the store and the plaintext session token.
func setupSessionStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	db := s.DB()
	now := time.Now().UTC().Format(time.RFC3339)

	teamID := "sess-team-01"
	db.Exec(`INSERT INTO teams(id, name, owner, created_at) VALUES(?,?,?,?)`,
		teamID, "Session Corp", "boss@corp.com", now)

	memberID := "sess-member-01"
	db.Exec(`INSERT INTO team_members(id, team_id, email, role, created_at) VALUES(?,?,?,?,?)`,
		memberID, teamID, "dev@corp.com", "member", now)

	plain := fmt.Sprintf("mcp-test-token-%d", time.Now().UnixNano())
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(plain)))
	exp := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at)
		VALUES(?,?,?,?,?,?)`,
		"sess-session-01", teamID, memberID, "dev@corp.com", hash, exp)

	// Seed a team skill
	db.Exec(`INSERT INTO skills(id, team_id, name, body, tags, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?)`,
		"skill-01", teamID, "hex-arch",
		"Always maintain hexagonal layer separation — domain never imports infra.",
		"[]", now, now)

	// Seed a private scroll
	db.Exec(`INSERT INTO private_scrolls(id, name, content, team_id, created_by, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?)`,
		"scroll-01", "deploy-guide",
		"# Deploy Guide\nUse blue/green with k8s AKS.",
		teamID, "boss@corp.com", now, now)

	return s, plain
}

// newSessionServer builds an MCP server that receives sessionToken in the
// initialize params followed by the given tool call lines.
func newSessionServer(t *testing.T, s *store.Store, sessionToken string, toolLines ...string) *strings.Builder {
	t.Helper()

	initLine := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","session_token":%q}}`,
		sessionToken,
	)
	lines := []string{initLine}
	lines = append(lines, toolLines...)
	input := strings.Join(lines, "\n") + "\n"

	var out strings.Builder
	srv := &Server{
		store:  s,
		reader: bufio.NewReader(strings.NewReader(input)),
		writer: &out,
		logger: log.New(bytes.NewBuffer(nil), "", 0), // discard logs
	}
	srv.Run() //nolint:errcheck
	return &out
}

// decodeAll reads every JSON-RPC Response from the builder output.
func decodeAll(t *testing.T, out *strings.Builder) []Response {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(out.String()))
	var responses []Response
	for dec.More() {
		var r Response
		if err := dec.Decode(&r); err != nil {
			t.Fatalf("decoding response: %v\noutput: %s", err, out.String())
		}
		responses = append(responses, r)
	}
	return responses
}

// ── MCP session: initialize resolves session token ───────────────────────────

func TestMCP_SessionResolved(t *testing.T) {
	s, token := setupSessionStore(t)
	out := newSessionServer(t, s, token) // only initialize
	responses := decodeAll(t, out)

	if len(responses) != 1 {
		t.Fatalf("want 1 response (initialize), got %d", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("initialize error: %v", responses[0].Error)
	}
}

func TestMCP_SessionInvalid_GracefulDegradation(t *testing.T) {
	s, _ := setupSessionStore(t)
	// Pass a totally bogus token — server should still respond to initialize
	out := newSessionServer(t, s, "not-a-valid-token")
	responses := decodeAll(t, out)

	if len(responses) != 1 {
		t.Fatalf("want 1 response even with bad token, got %d", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("invalid token must NOT error initialize: %v", responses[0].Error)
	}
}

// ── vault_save: auto-fills team from session ─────────────────────────────────

func TestMCP_VaultSave_TeamAutoFill(t *testing.T) {
	s, token := setupSessionStore(t)

	saveLine := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_save","arguments":{"title":"Auto team","content":"test","type":"pattern"}}}`
	out := newSessionServer(t, s, token, saveLine)
	responses := decodeAll(t, out)

	if len(responses) != 2 {
		t.Fatalf("want 2 responses (init + save), got %d — %s", len(responses), out.String())
	}
	saveResp := responses[1]
	if saveResp.Error != nil {
		t.Fatalf("vault_save error: %v", saveResp.Error)
	}

	result := parseToolResult(t, saveResp)
	if !strings.Contains(result.Content[0].Text, `"status": "saved"`) {
		t.Errorf("unexpected result: %s", result.Content[0].Text)
	}

	// The saved observation should have team_id from the session
	obs, _ := s.Search("Auto team", store.SearchFilters{Team: "sess-team-01", Limit: 5})
	if len(obs) == 0 {
		t.Error("vault_save with session: observation should be tagged with session team_id")
	}
}

// ── vault_context: includes team skills and scrolls ──────────────────────────

func TestMCP_VaultContext_TeamEnrichment(t *testing.T) {
	s, token := setupSessionStore(t)

	ctxLine := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_context","arguments":{"project":"home-api"}}}`
	out := newSessionServer(t, s, token, ctxLine)
	responses := decodeAll(t, out)

	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	ctxResp := responses[1]
	if ctxResp.Error != nil {
		t.Fatalf("vault_context error: %v", ctxResp.Error)
	}

	result := parseToolResult(t, ctxResp)
	text := result.Content[0].Text

	// Response must include team_skills and team_scrolls from the seeded fixtures
	if !strings.Contains(text, "hex-arch") {
		t.Errorf("vault_context: expected team skill 'hex-arch' in response:\n%s", text)
	}
	if !strings.Contains(text, "deploy-guide") {
		t.Errorf("vault_context: expected team scroll 'deploy-guide' in response:\n%s", text)
	}
	if !strings.Contains(text, "sess-team-01") {
		t.Errorf("vault_context: expected team_id in response:\n%s", text)
	}
}

// ── vault_context: no enrichment without session ─────────────────────────────

func TestMCP_VaultContext_NoSession(t *testing.T) {
	s, _ := setupSessionStore(t)

	// Build server WITHOUT a session token in initialize
	initLine := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	ctxLine := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_context","arguments":{"project":"home-api"}}}`
	input := initLine + "\n" + ctxLine + "\n"

	var outBuf strings.Builder
	srv := &Server{
		store:  s,
		reader: bufio.NewReader(strings.NewReader(input)),
		writer: &outBuf,
		logger: log.New(bytes.NewBuffer(nil), "", 0),
	}
	srv.Run() //nolint:errcheck

	responses := decodeAll(t, &outBuf)
	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	text := parseToolResult(t, responses[1]).Content[0].Text

	// Without session: team_skills and team_scrolls must NOT appear
	if strings.Contains(text, "hex-arch") {
		t.Error("vault_context without session must not include team_skills")
	}
	if strings.Contains(text, "team_id") {
		t.Error("vault_context without session must not include team_id")
	}
}

// ── vault_team_context ────────────────────────────────────────────────────────

func TestMCP_VaultTeamContext_WithSession(t *testing.T) {
	s, token := setupSessionStore(t)

	tcLine := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_team_context","arguments":{}}}`
	out := newSessionServer(t, s, token, tcLine)
	responses := decodeAll(t, out)

	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	tcResp := responses[1]
	if tcResp.Error != nil {
		t.Fatalf("vault_team_context error: %v", tcResp.Error)
	}

	result := parseToolResult(t, tcResp)
	text := result.Content[0].Text

	if !strings.Contains(text, "sess-team-01") {
		t.Errorf("vault_team_context: expected team_id:\n%s", text)
	}
	if !strings.Contains(text, "hex-arch") {
		t.Errorf("vault_team_context: expected skill:\n%s", text)
	}
	if !strings.Contains(text, "deploy-guide") {
		t.Errorf("vault_team_context: expected scroll:\n%s", text)
	}
}

func TestMCP_VaultTeamContext_NoSession(t *testing.T) {
	s, _ := setupSessionStore(t)

	initLine := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	tcLine := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_team_context","arguments":{}}}`
	input := initLine + "\n" + tcLine + "\n"

	var outBuf strings.Builder
	srv := &Server{
		store:  s,
		reader: bufio.NewReader(strings.NewReader(input)),
		writer: &outBuf,
		logger: log.New(bytes.NewBuffer(nil), "", 0),
	}
	srv.Run() //nolint:errcheck

	responses := decodeAll(t, &outBuf)
	result := parseToolResult(t, responses[1])
	text := result.Content[0].Text

	if !strings.Contains(text, "no active session") {
		t.Errorf("vault_team_context without session should include hint:\n%s", text)
	}
}

// parseToolResult is defined in bulk_save_test.go (same package).
