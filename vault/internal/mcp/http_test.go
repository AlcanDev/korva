package mcp

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// seedSessionToken inserts a team + member + active session row into the given
// store and returns the plaintext bearer token that resolveSession() will
// accept. Used by auth-gate tests that need a real, valid token.
func seedSessionToken(t *testing.T, s *store.Store, email, teamID string) string {
	t.Helper()
	db := s.DB()
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := db.Exec(`INSERT INTO teams(id, name, owner, created_at) VALUES(?,?,?,?)`,
		teamID, teamID+" name", email, now); err != nil {
		t.Fatalf("insert team: %v", err)
	}

	memberID := teamID + "-member"
	if _, err := db.Exec(`INSERT INTO team_members(id, team_id, email, role, created_at) VALUES(?,?,?,?,?)`,
		memberID, teamID, email, "member", now); err != nil {
		t.Fatalf("insert team_member: %v", err)
	}

	plain := fmt.Sprintf("mcp-http-token-%d", time.Now().UnixNano())
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(plain)))
	exp := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO member_sessions(id, team_id, member_id, email, token_hash, expires_at)
		VALUES(?,?,?,?,?,?)`,
		teamID+"-session", teamID, memberID, email, hash, exp); err != nil {
		t.Fatalf("insert member_session: %v", err)
	}
	return plain
}

// newHTTPTestServer builds a Server backed by an in-memory store and exposes
// it through the Streamable HTTP transport. The returned httptest.Server runs
// for the lifetime of the test. Anonymous access is enabled so tests focused
// on transport mechanics don't have to mint session tokens; the dedicated
// auth-gate tests flip this env var off to exercise the default-deny path.
func newHTTPTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	t.Setenv("KORVA_MCP_ALLOW_ANONYMOUS", "true")

	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	srv := &Server{
		store:        s,
		logger:       log.New(io.Discard, "", 0),
		profile:      ProfileAdmin,
		contextCache: make(map[string]*contextCacheEntry),
	}
	ts := httptest.NewServer(srv.HTTPHandler())
	t.Cleanup(ts.Close)
	return ts
}

func postJSONRPC(t *testing.T, baseURL, body string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, data
}

func TestHTTPInitialize(t *testing.T) {
	ts := newHTTPTestServer(t)

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"claude-code","version":"1.0"}}}`
	resp, body := postJSONRPC(t, ts.URL, req, nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal: %v\nbody=%s", err, body)
	}
	if r.Error != nil {
		t.Fatalf("response error: %+v", r.Error)
	}

	raw, _ := json.Marshal(r.Result)
	var init InitializeResult
	if err := json.Unmarshal(raw, &init); err != nil {
		t.Fatalf("unmarshal InitializeResult: %v", err)
	}
	if init.ServerInfo.Name != "korva-vault" {
		t.Errorf("server name = %q, want korva-vault", init.ServerInfo.Name)
	}
	if init.ProtocolVersion == "" {
		t.Error("protocolVersion is empty")
	}
}

func TestHTTPToolsList(t *testing.T) {
	ts := newHTTPTestServer(t)

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	resp, body := postJSONRPC(t, ts.URL, req, nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body=%s", resp.StatusCode, body)
	}

	var r struct {
		Result struct {
			Tools []Tool `json:"tools"`
		} `json:"result"`
		Error *RPCError `json:"error"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Error != nil {
		t.Fatalf("error: %+v", r.Error)
	}
	if len(r.Result.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
}

func TestHTTPToolsCallVaultStats(t *testing.T) {
	ts := newHTTPTestServer(t)

	req := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"vault_stats","arguments":{}}}`
	resp, body := postJSONRPC(t, ts.URL, req, nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body=%s", resp.StatusCode, body)
	}

	var r struct {
		Result ToolCallResult `json:"result"`
		Error  *RPCError      `json:"error"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Error != nil {
		t.Fatalf("error: %+v", r.Error)
	}
	if r.Result.IsError {
		t.Errorf("vault_stats reported isError; content=%+v", r.Result.Content)
	}
	if len(r.Result.Content) == 0 {
		t.Error("vault_stats returned no content blocks")
	}
}

func TestHTTPNotificationReturns202(t *testing.T) {
	ts := newHTTPTestServer(t)

	// JSON-RPC notification: no id field. Spec forbids a response body.
	req := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`
	resp, body := postJSONRPC(t, ts.URL, req, nil)

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", resp.StatusCode, body)
	}
	if len(body) != 0 {
		t.Errorf("notification response body should be empty, got %q", body)
	}
}

func TestHTTPParseError(t *testing.T) {
	ts := newHTTPTestServer(t)

	resp, body := postJSONRPC(t, ts.URL, `{not json`, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", resp.StatusCode, body)
	}

	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Error == nil || r.Error.Code != -32700 {
		t.Errorf("expected parse error -32700, got %+v", r.Error)
	}
}

func TestHTTPMethodNotAllowed(t *testing.T) {
	ts := newHTTPTestServer(t)

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		req, _ := http.NewRequest(method, ts.URL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", method, resp.StatusCode)
		}
		if allow := resp.Header.Get("Allow"); !strings.Contains(allow, "POST") {
			t.Errorf("%s: Allow header = %q, want POST", method, allow)
		}
	}
}

func TestHTTPOptionsCORS(t *testing.T) {
	ts := newHTTPTestServer(t)

	req, _ := http.NewRequest(http.MethodOptions, ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", resp.StatusCode)
	}
	if h := resp.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(h, "Authorization") {
		t.Errorf("Allow-Headers = %q, missing Authorization", h)
	}
}

func TestHTTPBearerTokenExtraction(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", ""},
		{"Bearer abc123", "abc123"},
		{"bearer xyz", "xyz"},          // case-insensitive scheme
		{"Bearer  spaced  ", "spaced"}, // trim
		{"Basic creds", ""},            // wrong scheme
		{"Bearer", ""},                 // no token
	}
	for _, tc := range cases {
		r, _ := http.NewRequest(http.MethodPost, "/", nil)
		if tc.header != "" {
			r.Header.Set("Authorization", tc.header)
		}
		if got := bearerToken(r); got != tc.want {
			t.Errorf("bearerToken(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestHTTPAuthGateRejectsAnonymous(t *testing.T) {
	// Default-deny: KORVA_MCP_ALLOW_ANONYMOUS unset → any request without a
	// valid bearer is rejected with 401.
	t.Setenv("KORVA_MCP_ALLOW_ANONYMOUS", "")

	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	srv := &Server{
		store:        s,
		logger:       log.New(io.Discard, "", 0),
		profile:      ProfileAdmin,
		contextCache: make(map[string]*contextCacheEntry),
	}
	ts := httptest.NewServer(srv.HTTPHandler())
	t.Cleanup(ts.Close)

	// No Authorization header — must 401.
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	resp, body := postJSONRPC(t, ts.URL, req, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want 401; body=%s", resp.StatusCode, body)
	}
	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Error == nil || r.Error.Code != -32001 {
		t.Errorf("expected JSON-RPC error -32001, got %+v", r.Error)
	}

	// Bogus bearer token — resolveSession silently fails, session stays nil → 401.
	resp, body = postJSONRPC(t, ts.URL, req, map[string]string{"Authorization": "Bearer not-a-real-token"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bogus bearer status = %d, want 401; body=%s", resp.StatusCode, body)
	}
}

func TestHTTPAuthGateAcceptsValidToken(t *testing.T) {
	t.Setenv("KORVA_MCP_ALLOW_ANONYMOUS", "")

	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	// Mint a real session token directly through the store so we exercise the
	// same path resolveSession() reads from.
	token := seedSessionToken(t, s, "ops@example.com", "team-test")

	srv := &Server{
		store:        s,
		logger:       log.New(io.Discard, "", 0),
		profile:      ProfileAdmin,
		contextCache: make(map[string]*contextCacheEntry),
	}
	ts := httptest.NewServer(srv.HTTPHandler())
	t.Cleanup(ts.Close)

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	resp, body := postJSONRPC(t, ts.URL, req, map[string]string{"Authorization": "Bearer " + token})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid bearer status = %d, want 200; body=%s", resp.StatusCode, body)
	}
}

func TestHTTPConcurrentRequestsDoNotShareSession(t *testing.T) {
	// Two requests in parallel must not race on the shared Server's session
	// field. Even with no bearer token, the forRequest() clone path is
	// exercised — the race detector catches violations.
	ts := newHTTPTestServer(t)

	done := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
			httpResp, err := http.Post(ts.URL, "application/json", bytes.NewBufferString(req))
			if err != nil {
				t.Errorf("post: %v", err)
				return
			}
			httpResp.Body.Close()
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
