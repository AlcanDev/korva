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

// Phase 19.A — verify that handleInitialize parses clientInfo.name
// into Server.editor, and that dispatch propagates it to the
// mcp_calls row.

func newServerWithBufferedStore(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	s, err := store.NewMemory(nil)
	if err != nil {
		t.Fatalf("store.NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	srv := &Server{
		store:   s,
		reader:  bufio.NewReader(strings.NewReader("")),
		writer:  &bytes.Buffer{},
		logger:  log.New(bytes.NewBuffer(nil), "", 0),
		profile: ProfileAdmin,
	}
	return srv, s
}

func TestHandleInitialize_ParsesClientInfoIntoEditor(t *testing.T) {
	cases := []struct {
		name       string
		clientName string
		want       string
	}{
		{"claude-ai → claude", "claude-ai", "claude"},
		{"cursor", "Cursor", "cursor"},
		{"codex/0.1", "codex/0.1.0", "codex"},
		{"unknown stays empty", "neovim-mcp", ""},
		{"empty stays empty", "", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newServerWithBufferedStore(t)
			params, _ := json.Marshal(map[string]any{
				"clientInfo": map[string]any{
					"name":    tc.clientName,
					"version": "1.0",
				},
			})
			srv.handleInitialize(Request{
				ID:     json.RawMessage(`1`),
				Method: "initialize",
				Params: params,
			})
			if srv.editor != tc.want {
				t.Errorf("Server.editor = %q, want %q", srv.editor, tc.want)
			}
		})
	}
}

func TestHandleInitialize_NoParamsKeepsEditorEmpty(t *testing.T) {
	srv, _ := newServerWithBufferedStore(t)
	srv.handleInitialize(Request{ID: json.RawMessage(`1`), Method: "initialize"})
	if srv.editor != "" {
		t.Errorf("Server.editor = %q, want empty when no params", srv.editor)
	}
}

func TestHandleInitialize_SessionTokenAndClientInfoCoexist(t *testing.T) {
	srv, _ := newServerWithBufferedStore(t)
	// Both fields present — clientInfo should be parsed even though
	// the session token is empty / unresolvable. We don't have a
	// valid token to assert resolution, but the editor should still
	// land in place.
	params, _ := json.Marshal(map[string]any{
		"session_token": "irrelevant-for-this-test",
		"clientInfo":    map[string]any{"name": "windsurf"},
	})
	srv.handleInitialize(Request{ID: json.RawMessage(`1`), Method: "initialize", Params: params})
	if srv.editor != "windsurf" {
		t.Errorf("Server.editor = %q, want windsurf", srv.editor)
	}
}

func TestDispatch_LogsEditorFromServerState(t *testing.T) {
	srv, s := newServerWithBufferedStore(t)

	// Simulate an initialize that set editor=cursor.
	params, _ := json.Marshal(map[string]any{
		"clientInfo": map[string]any{"name": "cursor"},
	})
	srv.handleInitialize(Request{ID: json.RawMessage(`1`), Method: "initialize", Params: params})

	// Dispatch a known-safe read-only tool so we hit the logging
	// path without needing harness scaffolding.
	if _, err := srv.dispatch("vault_status", map[string]any{}); err != nil {
		// vault_status may error if it queries the store with no
		// rows; we only care that LogCall ran.
		t.Logf("vault_status returned %v (ok — logging is what we test)", err)
	}

	calls, err := s.ListCalls(store.CallFilters{Limit: 10})
	if err != nil {
		t.Fatalf("ListCalls: %v", err)
	}
	if len(calls) == 0 {
		t.Fatal("expected one logged call")
	}
	if calls[0].Editor != "cursor" {
		t.Errorf("logged call editor = %q, want cursor", calls[0].Editor)
	}
}

func TestDispatch_EmptyEditorWhenInitializeWasAnonymous(t *testing.T) {
	srv, s := newServerWithBufferedStore(t)
	// No initialize call → editor stays empty.
	_, _ = srv.dispatch("vault_status", map[string]any{})

	calls, _ := s.ListCalls(store.CallFilters{Limit: 1})
	if len(calls) == 0 {
		t.Fatal("expected one logged call")
	}
	if calls[0].Editor != "" {
		t.Errorf("anonymous call should record empty editor, got %q", calls[0].Editor)
	}
}
