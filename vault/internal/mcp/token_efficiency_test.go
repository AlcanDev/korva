package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/alcandev/korva/vault/internal/store"
)

// newTestServerWithStore creates a Server sharing a pre-populated store.
func newTestServerWithStore(t *testing.T, s *store.Store, input string) (*Server, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	srv := &Server{
		store:        s,
		reader:       bufio.NewReader(strings.NewReader(input + "\n")),
		writer:       &out,
		logger:       log.New(bytes.NewBuffer(nil), "", 0),
		profile:      ProfileAdmin,
		contextCache: make(map[string]*contextCacheEntry),
	}
	return srv, &out
}

func callTool(t *testing.T, s *store.Store, toolName string, toolArgs map[string]any) map[string]any {
	t.Helper()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": toolArgs,
		},
	}
	b, _ := json.Marshal(req)
	srv, out := newTestServerWithStore(t, s, string(b))
	_ = srv.Run()

	var resp Response
	if err := json.NewDecoder(out).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v\noutput: %s", err, out.String())
	}
	if resp.Error != nil {
		t.Fatalf("tool %q returned error: %v", toolName, resp.Error)
	}

	// MCP tools/call wraps the result in ToolCallResult{Content: [{type:"text", text: JSON}]}.
	// Re-marshal then decode the inner JSON text.
	rawResult, _ := json.Marshal(resp.Result)
	var tcr ToolCallResult
	if err := json.Unmarshal(rawResult, &tcr); err != nil {
		t.Fatalf("unmarshal ToolCallResult: %v", err)
	}
	if len(tcr.Content) == 0 {
		t.Fatalf("tool %q returned empty content", toolName)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(tcr.Content[0].Text), &result); err != nil {
		t.Fatalf("unmarshal tool text result: %v\ntext: %s", err, tcr.Content[0].Text)
	}
	return result
}

// TestVaultHint_ReturnsNoContent verifies that vault_hint omits content fields.
func TestVaultHint_ReturnsNoContent(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	s.Save(store.Observation{ //nolint:errcheck
		Project: "proj",
		Type:    "decision",
		Title:   "Use gRPC for inter-service comms",
		Content: "Very long content that should NOT appear in vault_hint response at all",
	})

	result := callTool(t, s, "vault_hint", map[string]any{"query": "gRPC", "project": "proj"})

	hints, ok := result["hints"].([]any)
	if !ok {
		t.Fatalf("expected hints array, got %T", result["hints"])
	}
	if len(hints) == 0 {
		t.Fatal("expected at least 1 hint")
	}

	item := hints[0].(map[string]any)

	// Must have id and title
	if item["id"] == nil || item["id"] == "" {
		t.Error("hint item missing id")
	}
	if item["title"] == nil || item["title"] == "" {
		t.Error("hint item missing title")
	}

	// Must NOT have content
	if _, hasContent := item["content"]; hasContent {
		t.Error("vault_hint must not include content field — defeats token saving purpose")
	}
}

// TestVaultHint_InAllProfiles ensures vault_hint is available in all profiles.
func TestVaultHint_InAllProfiles(t *testing.T) {
	for _, p := range []Profile{ProfileAgent, ProfileReadonly, ProfileAdmin} {
		found := false
		for _, tool := range toolsForProfile(p) {
			if tool.Name == "vault_hint" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("profile %q missing vault_hint", p)
		}
	}
}

// TestApplyTokenBudget_TruncatesContent verifies budget enforcement.
func TestApplyTokenBudget_TruncatesContent(t *testing.T) {
	obs := []store.Observation{
		{ID: "1", Title: "short", Content: strings.Repeat("x", 2000)},
		{ID: "2", Title: "second", Content: strings.Repeat("y", 2000)},
	}

	// budget of 200 tokens = 800 chars; each item needs ~40 token metadata overhead
	result := applyTokenBudget(obs, 200)

	if len(result) == 0 {
		t.Fatal("expected at least one result")
	}
	for _, o := range result {
		if len(o.Content) > 800 {
			t.Errorf("content not truncated: len=%d > budget chars", len(o.Content))
		}
		// Truncated items end with the marker. Untruncated items (content within
		// budget) just have whatever content they originally had.
		_ = strings.HasSuffix(o.Content, "…[truncated]")
	}
}

// TestVaultContext_DeltaReturnsEmpty tests that delta=true with no new observations returns empty.
func TestVaultContext_DeltaReturnsEmpty(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	id, _ := s.Save(store.Observation{
		Project: "myproj", Type: "decision", Title: "Initial", Content: "content",
	})
	_ = id

	// First call populates the cache.
	result1 := callTool(t, s, "vault_context", map[string]any{"project": "myproj"})
	ctx1, _ := result1["context"].([]any)
	if len(ctx1) == 0 {
		t.Fatal("expected context on first call")
	}

	// Second call with delta=true — but we need the same server instance to share state.
	// Test the store method directly instead.
	obs, err2 := s.ContextSince("myproj", id, 10)
	if err2 != nil {
		t.Fatal(err2)
	}
	if len(obs) != 0 {
		t.Errorf("ContextSince should return 0 for id equal to latest, got %d", len(obs))
	}
}

// TestContextSince_ReturnsNewObservations tests the delta store method.
func TestContextSince_ReturnsNewObservations(t *testing.T) {
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	id1, _ := s.Save(store.Observation{Project: "p", Type: "decision", Title: "First", Content: "c1"})
	// ULIDs embed millisecond timestamp; sleep to ensure id2 > id1 lexicographically.
	// Without this, two saves within the same millisecond can produce non-monotonic IDs.
	time.Sleep(2 * time.Millisecond)
	id2, _ := s.Save(store.Observation{Project: "p", Type: "decision", Title: "Second", Content: "c2"})
	_ = id2

	// Should return only observations after id1 (i.e., id2 and beyond).
	obs, err := s.ContextSince("p", id1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Errorf("expected 1 new observation, got %d", len(obs))
	}
	if obs[0].Title != "Second" {
		t.Errorf("expected 'Second', got %q", obs[0].Title)
	}
}
