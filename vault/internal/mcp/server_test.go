package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

// newTestServer creates a Server with injected reader/writer for testing.
func newTestServer(t *testing.T, input string) (*Server, *bytes.Buffer) {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	var out bytes.Buffer
	srv := &Server{
		store:  s,
		reader: bufio.NewReader(strings.NewReader(input + "\n")),
		writer: &out,
		logger: log.New(bytes.NewBuffer(nil), "", 0), // discard logs
	}
	return srv, &out
}

// sendAndReceive sends a JSON-RPC request line and returns the parsed response.
func sendAndReceive(t *testing.T, input string) Response {
	t.Helper()
	srv, out := newTestServer(t, input)
	_ = srv.Run()

	var resp Response
	if err := json.NewDecoder(out).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v\noutput: %s", err, out.String())
	}
	return resp
}

// ─── initialize ──────────────────────────────────────────────────────────────

func TestInitialize(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("initialize error: %v", resp.Error)
	}

	// Result should contain protocolVersion
	raw, _ := json.Marshal(resp.Result)
	var init InitializeResult
	if err := json.Unmarshal(raw, &init); err != nil {
		t.Fatalf("unmarshal InitializeResult: %v", err)
	}
	if init.ProtocolVersion == "" {
		t.Error("initialize should return a protocolVersion")
	}
	if init.ServerInfo.Name != "korva-vault" {
		t.Errorf("server name = %q, want 'korva-vault'", init.ServerInfo.Name)
	}
}

// ─── tools/list ──────────────────────────────────────────────────────────────

func TestToolsList(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("tools/list error: %v", resp.Error)
	}

	raw, _ := json.Marshal(resp.Result)
	var result map[string]any
	json.Unmarshal(raw, &result)

	toolsRaw, ok := result["tools"]
	if !ok {
		t.Fatal("tools/list should return a 'tools' key")
	}
	toolsArr, ok := toolsRaw.([]any)
	if !ok {
		t.Fatal("tools should be an array")
	}
	const wantTools = 14 // vault_save, vault_search, vault_context, vault_timeline,
	// vault_get, vault_session_start, vault_session_end, vault_summary,
	// vault_save_prompt, vault_stats, vault_delete, vault_query, vault_bulk_save,
	// vault_team_context
	if len(toolsArr) != wantTools {
		t.Errorf("expected exactly %d tools, got %d", wantTools, len(toolsArr))
	}
}

// ─── ping ────────────────────────────────────────────────────────────────────

func TestPing(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":3,"method":"ping"}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("ping error: %v", resp.Error)
	}
}

// ─── unknown method ───────────────────────────────────────────────────────────

func TestUnknownMethod(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":4,"method":"no_such_method"}`
	resp := sendAndReceive(t, req)

	if resp.Error == nil {
		t.Error("unknown method should return an error")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601 (method not found), got %d", resp.Error.Code)
	}
}

// ─── parse error ─────────────────────────────────────────────────────────────

func TestParseError(t *testing.T) {
	srv, out := newTestServer(t, "not-valid-json")
	_ = srv.Run()

	var resp Response
	json.NewDecoder(out).Decode(&resp)
	if resp.Error == nil {
		t.Error("invalid JSON should produce a parse error response")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected code -32700 (parse error), got %d", resp.Error.Code)
	}
}

// ─── vault_save ──────────────────────────────────────────────────────────────

func TestToolCall_VaultSave(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"vault_save","arguments":{"project":"home-api","type":"decision","title":"Hexagonal","content":"Use ports and adapters"}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_save error: %v", resp.Error)
	}

	// Check result has content with an ID
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("vault_save should return content")
	}
	if !strings.Contains(result.Content[0].Text, "id") {
		t.Errorf("vault_save result should contain an id, got: %s", result.Content[0].Text)
	}
}

// ─── vault_stats ─────────────────────────────────────────────────────────────

func TestToolCall_VaultStats(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"vault_stats","arguments":{}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_stats error: %v", resp.Error)
	}

	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)

	if len(result.Content) == 0 {
		t.Error("vault_stats should return content")
	}
}

// ─── vault_search ─────────────────────────────────────────────────────────────

func TestToolCall_VaultSearch(t *testing.T) {
	// Save then search
	srv, out := newTestServer(t, "")
	srv.reader = bufio.NewReader(strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"vault_save","arguments":{"project":"p","type":"decision","title":"hexagonal architecture","content":"detail"}}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_search","arguments":{"query":"hexagonal","limit":10}}}` + "\n",
	))
	_ = srv.Run()

	// Read two responses
	dec := json.NewDecoder(out)
	var r1, r2 Response
	dec.Decode(&r1) // vault_save
	dec.Decode(&r2) // vault_search

	if r2.Error != nil {
		t.Fatalf("vault_search error: %v", r2.Error)
	}
	raw, _ := json.Marshal(r2.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if len(result.Content) == 0 {
		t.Error("vault_search should return content")
	}
}

// ─── vault_context ────────────────────────────────────────────────────────────

func TestToolCall_VaultContext(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"vault_context","arguments":{"project":"home-api","limit":5}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_context error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if len(result.Content) == 0 {
		t.Error("vault_context should return content")
	}
}

// ─── vault_timeline ───────────────────────────────────────────────────────────

func TestToolCall_VaultTimeline(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"vault_timeline","arguments":{"project":"home-api","from":"2026-01-01T00:00:00Z","to":"2027-01-01T00:00:00Z"}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_timeline error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if len(result.Content) == 0 {
		t.Error("vault_timeline should return content")
	}
}

// ─── vault_get ────────────────────────────────────────────────────────────────

func TestToolCall_VaultGet(t *testing.T) {
	// Get a nonexistent ID — should return gracefully
	req := `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"vault_get","arguments":{"id":"01ABC"}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_get error: %v", resp.Error)
	}
}

// ─── vault_session_start / end ────────────────────────────────────────────────

func TestToolCall_VaultSessionStartEnd(t *testing.T) {
	srv, out := newTestServer(t, "")
	srv.reader = bufio.NewReader(strings.NewReader(
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"vault_session_start","arguments":{"project":"p","team":"backend","country":"CL","agent":"copilot","goal":"implement feature"}}}` + "\n" +
			`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"vault_session_end","arguments":{"session_id":"PLACEHOLDER","summary":"done"}}}` + "\n",
	))
	_ = srv.Run()

	dec := json.NewDecoder(out)
	var r1 Response
	dec.Decode(&r1)

	if r1.Error != nil {
		t.Fatalf("vault_session_start error: %v", r1.Error)
	}
	raw, _ := json.Marshal(r1.Result)
	var startResult ToolCallResult
	json.Unmarshal(raw, &startResult)
	if len(startResult.Content) == 0 || !strings.Contains(startResult.Content[0].Text, "session_id") {
		t.Errorf("vault_session_start should return session_id, got: %v", startResult.Content)
	}
}

// ─── vault_summary ────────────────────────────────────────────────────────────

func TestToolCall_VaultSummary(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"vault_summary","arguments":{"project":"home-api"}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_summary error: %v", resp.Error)
	}
}

// ─── vault_save_prompt ────────────────────────────────────────────────────────

func TestToolCall_VaultSavePrompt(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"vault_save_prompt","arguments":{"name":"hex-review","content":"Review hexagonal boundaries carefully","tags":["hex","review"]}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_save_prompt error: %v", resp.Error)
	}
}

// ─── tools/call invalid params ────────────────────────────────────────────────

func TestToolCall_InvalidParams(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":15,"method":"tools/call","params":"not-an-object"}`
	resp := sendAndReceive(t, req)

	if resp.Error == nil {
		t.Error("tools/call with invalid params should return RPC error")
	}
}

// ─── empty line ignored ───────────────────────────────────────────────────────

func TestEmptyLineIgnored(t *testing.T) {
	// Empty lines should be skipped, not crash
	srv, out := newTestServer(t, "")
	srv.reader = bufio.NewReader(strings.NewReader(
		"\n" +
			"\n" +
			`{"jsonrpc":"2.0","id":16,"method":"ping"}` + "\n",
	))
	_ = srv.Run()

	var resp Response
	if err := json.NewDecoder(out).Decode(&resp); err != nil {
		t.Fatalf("expected a response after empty lines: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("ping after empty lines should succeed, got: %v", resp.Error)
	}
}

// ─── unknown tool ─────────────────────────────────────────────────────────────

func TestToolCall_UnknownTool(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"no_such_tool","arguments":{}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}

	// Tool errors come back as isError=true in the result, not as RPC errors
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if !result.IsError {
		t.Error("unknown tool should return isError=true in result")
	}
}
