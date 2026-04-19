package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBulkSave_HappyPath(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"vault_bulk_save","arguments":{"observations":[{"title":"A","content":"alpha","type":"pattern"},{"title":"B","content":"beta","type":"decision"},{"title":"C","content":"gamma","type":"learning"}]}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}

	result := parseToolResult(t, resp)
	text := result.Content[0].Text

	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("result is not JSON: %v\ntext: %s", err, text)
	}
	if payload["saved"].(float64) != 3 {
		t.Fatalf("want saved=3, got %v", payload["saved"])
	}
	ids := payload["ids"].([]any)
	if len(ids) != 3 {
		t.Fatalf("want 3 IDs, got %d", len(ids))
	}
	for i, id := range ids {
		if s, ok := id.(string); !ok || s == "" {
			t.Errorf("ids[%d] is empty", i)
		}
	}
}

func TestBulkSave_EmptyArray(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"vault_bulk_save","arguments":{"observations":[]}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Error("want IsError=true for empty array")
	}
	if !strings.Contains(result.Content[0].Text, "empty") {
		t.Errorf("want 'empty' in error, got: %s", result.Content[0].Text)
	}
}

func TestBulkSave_TooMany(t *testing.T) {
	// Build 51 items
	items := make([]string, 51)
	for i := range items {
		items[i] = `{"title":"x","content":"y","type":"pattern"}`
	}
	req := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"vault_bulk_save","arguments":{"observations":[` +
		strings.Join(items, ",") + `]}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Error("want IsError=true for >50 items")
	}
	if !strings.Contains(result.Content[0].Text, "too many") {
		t.Errorf("want 'too many' in error, got: %s", result.Content[0].Text)
	}
}

func TestBulkSave_MissingObservations(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"vault_bulk_save","arguments":{}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Error("want IsError=true when observations key is missing")
	}
}

func TestBulkSave_DefaultType(t *testing.T) {
	// Omitting "type" should default to context, not error.
	req := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"vault_bulk_save","arguments":{"observations":[{"title":"no type","content":"body"}]}}}`
	resp := sendAndReceive(t, req)

	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %v", resp.Error)
	}
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Errorf("want success with default type, got error: %s", result.Content[0].Text)
	}

	var payload map[string]any
	json.Unmarshal([]byte(result.Content[0].Text), &payload) //nolint:errcheck
	if payload["saved"].(float64) != 1 {
		t.Fatalf("want saved=1, got %v", payload["saved"])
	}
}

// parseToolResult extracts the ToolCallResult from a Response. Fails the test
// if the response cannot be parsed.
func parseToolResult(t *testing.T, resp Response) ToolCallResult {
	t.Helper()
	raw, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal ToolCallResult: %v", err)
	}
	return result
}
