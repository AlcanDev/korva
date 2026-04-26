package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		store:   s,
		reader:  bufio.NewReader(strings.NewReader(input + "\n")),
		writer:  &out,
		logger:  log.New(bytes.NewBuffer(nil), "", 0), // discard logs
		profile: ProfileAdmin,                         // tests get full tool access
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
	const wantTools = 23 // 21 prior + vault_skill_match + vault_compress
	// vault_save, vault_search, vault_context, vault_timeline, vault_get, vault_hint,
	// vault_code_health, vault_pattern_mine, vault_skill_match, vault_compress,
	// vault_session_start, vault_session_end, vault_summary, vault_save_prompt,
	// vault_stats, vault_delete, vault_query, vault_bulk_save, vault_sdd_phase,
	// vault_qa_checklist, vault_qa_checkpoint, vault_team_context, vault_export_lore
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

// ─── vault_qa_checklist ───────────────────────────────────────────────────────

func TestToolCall_VaultQAChecklist_ReturnsGeneral(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":20,"method":"tools/call","params":{"name":"vault_qa_checklist","arguments":{"phase":"apply"}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_qa_checklist error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "APP-001") {
		t.Errorf("apply checklist should contain APP-001, got: %s", text)
	}
}

func TestToolCall_VaultQAChecklist_WithLanguage(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":21,"method":"tools/call","params":{"name":"vault_qa_checklist","arguments":{"phase":"apply","language":"go"}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_qa_checklist error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text
	// Should include both general and Go-specific criteria.
	if !strings.Contains(text, "APP-001") {
		t.Errorf("expected APP-001 in go/apply checklist")
	}
	if !strings.Contains(text, "GO-APP-001") {
		t.Errorf("expected GO-APP-001 in go/apply checklist")
	}
}

func TestToolCall_VaultQAChecklist_MissingPhase(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":22,"method":"tools/call","params":{"name":"vault_qa_checklist","arguments":{}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if !result.IsError {
		t.Error("missing phase should return isError=true")
	}
}

// ─── vault_qa_checkpoint ──────────────────────────────────────────────────────

func TestToolCall_VaultQACheckpoint_Save(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":23,"method":"tools/call","params":{"name":"vault_qa_checkpoint","arguments":{"project":"p","phase":"apply","status":"pass","score":85,"gate_passed":true,"findings":[{"rule":"APP-001","status":"pass","notes":"all tests present"}]}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_qa_checkpoint error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"id"`) {
		t.Errorf("response should contain checkpoint id, got: %s", text)
	}
	if !strings.Contains(text, "gate_unlocked") {
		t.Errorf("passing gate should include gate_unlocked message, got: %s", text)
	}
}

func TestToolCall_VaultQACheckpoint_Fail_HasGateNote(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":24,"method":"tools/call","params":{"name":"vault_qa_checkpoint","arguments":{"project":"p","phase":"apply","status":"fail","score":40,"gate_passed":false}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("vault_qa_checkpoint error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text
	if !strings.Contains(text, "gate_note") {
		t.Errorf("failing gate should include gate_note, got: %s", text)
	}
}

// ─── vault_sdd_phase gate enforcement ────────────────────────────────────────

func TestToolCall_SDDPhase_GatedTransition_Blocked(t *testing.T) {
	// Try to advance from apply → verify without a passing checkpoint.
	// The server should reject the transition.
	srv, out := newTestServer(t, "")
	srv.reader = bufio.NewReader(strings.NewReader(
		// Set phase to apply first.
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"vault_sdd_phase","arguments":{"project":"proj","phase":"apply"}}}` + "\n" +
			// Now try to advance to verify — should fail.
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_sdd_phase","arguments":{"project":"proj","phase":"verify"}}}` + "\n",
	))
	_ = srv.Run()

	dec := json.NewDecoder(out)
	var r1, r2 Response
	dec.Decode(&r1)
	dec.Decode(&r2)

	if r1.Error != nil {
		t.Fatalf("set apply phase error: %v", r1.Error)
	}

	// r2 result should be isError=true (gate blocked).
	raw, _ := json.Marshal(r2.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if !result.IsError {
		t.Error("advancing apply→verify without checkpoint should be blocked (isError=true)")
	}
	if !strings.Contains(result.Content[0].Text, "quality gate") {
		t.Errorf("error message should mention quality gate, got: %s", result.Content[0].Text)
	}
}

func TestToolCall_SDDPhase_GatedTransition_Unlocked(t *testing.T) {
	// Save a passing checkpoint then advance the phase — should succeed.
	srv, out := newTestServer(t, "")
	srv.reader = bufio.NewReader(strings.NewReader(
		// Set phase to apply.
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"vault_sdd_phase","arguments":{"project":"proj2","phase":"apply"}}}` + "\n" +
			// Submit passing checkpoint.
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_qa_checkpoint","arguments":{"project":"proj2","phase":"apply","status":"pass","score":90,"gate_passed":true}}}` + "\n" +
			// Now advance to verify — should succeed.
			`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"vault_sdd_phase","arguments":{"project":"proj2","phase":"verify"}}}` + "\n",
	))
	_ = srv.Run()

	dec := json.NewDecoder(out)
	var r1, r2, r3 Response
	dec.Decode(&r1)
	dec.Decode(&r2)
	dec.Decode(&r3)

	if r1.Error != nil {
		t.Fatalf("set apply: %v", r1.Error)
	}
	if r2.Error != nil {
		t.Fatalf("qa_checkpoint: %v", r2.Error)
	}

	raw, _ := json.Marshal(r3.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if result.IsError {
		t.Errorf("advance to verify should succeed after passing checkpoint, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "verify") {
		t.Errorf("response should confirm verify phase, got: %s", result.Content[0].Text)
	}
}

func TestToolCall_SDDPhase_UngatedTransition_AlwaysAllowed(t *testing.T) {
	// explore → propose is not gated, should always work.
	req := `{"jsonrpc":"2.0","id":30,"method":"tools/call","params":{"name":"vault_sdd_phase","arguments":{"project":"free-proj","phase":"propose"}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("RPC error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if result.IsError {
		t.Errorf("ungated transition should succeed, got: %s", result.Content[0].Text)
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

// ─── hybrid context (CloudSearcher) ──────────────────────────────────────────

// stubCloud is an in-test CloudSearcher.
type stubCloud struct {
	hits []CloudHit
	err  error
}

func (sc *stubCloud) Search(_ context.Context, _ string, _ int) ([]CloudHit, error) {
	return sc.hits, sc.err
}

func TestToolContext_HiveDisabled(t *testing.T) {
	// No CloudSearcher attached → hive_status must be "disabled".
	req := `{"jsonrpc":"2.0","id":40,"method":"tools/call","params":{"name":"vault_context","arguments":{"project":"proj1"}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("RPC error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"hive_status"`) || !strings.Contains(text, "disabled") {
		t.Errorf("hive_status should be 'disabled' when no CloudSearcher, got: %s", text)
	}
}

func TestToolContext_HiveAvailable(t *testing.T) {
	// Attach a stubCloud that returns one hit → hive_context + hive_status="ok".
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var out bytes.Buffer
	req := `{"jsonrpc":"2.0","id":41,"method":"tools/call","params":{"name":"vault_context","arguments":{"project":"proj1"}}}` + "\n"
	srv := &Server{
		store:  s,
		reader: bufio.NewReader(strings.NewReader(req)),
		writer: &out,
		logger: log.New(bytes.NewBuffer(nil), "", 0),
		cloud: &stubCloud{hits: []CloudHit{
			{ID: "h1", Type: "pattern", Title: "Hive hit", Content: "from cloud", Source: "hive"},
		}},
	}
	_ = srv.Run()

	var resp Response
	if err := json.NewDecoder(&out).Decode(&resp); err != nil {
		t.Fatalf("decode: %v — output: %s", err, out.String())
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text
	if !strings.Contains(text, `"hive_status"`) || !strings.Contains(text, "ok") {
		t.Errorf("hive_status should be ok, got: %s", text)
	}
	if !strings.Contains(text, "Hive hit") {
		t.Errorf("hive_context should contain the cloud hit, got: %s", text)
	}
}

func TestToolContext_HiveUnavailable(t *testing.T) {
	// Attach a stubCloud that returns an error → hive_status="unavailable",
	// but the tool must still succeed (local context returned).
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var out bytes.Buffer
	req := `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"vault_context","arguments":{"project":"proj1"}}}` + "\n"
	srv := &Server{
		store:  s,
		reader: bufio.NewReader(strings.NewReader(req)),
		writer: &out,
		logger: log.New(bytes.NewBuffer(nil), "", 0),
		cloud:  &stubCloud{err: fmt.Errorf("hive unreachable")},
	}
	_ = srv.Run()

	var resp Response
	if err := json.NewDecoder(&out).Decode(&resp); err != nil {
		t.Fatalf("decode: %v — output: %s", err, out.String())
	}
	if resp.Error != nil {
		t.Fatalf("RPC level error — hive failure should not surface as RPC error: %v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	json.Unmarshal(raw, &result)
	text := result.Content[0].Text
	if result.IsError {
		t.Errorf("tool should succeed even when hive is down: %s", text)
	}
	if !strings.Contains(text, "unavailable") {
		t.Errorf("hive_status should be 'unavailable', got: %s", text)
	}
}

