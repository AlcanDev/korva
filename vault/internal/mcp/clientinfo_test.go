package mcp

import (
	"testing"

	"github.com/alcandev/korva/internal/harness"
)

func TestResolveMCPClientEditor(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		// Empty / whitespace → anonymous.
		{"", ""},
		{"   ", ""},
		// Exact matches.
		{"claude", "claude"},
		{"cursor", "cursor"},
		{"windsurf", "windsurf"},
		{"continue", "continue"},
		{"copilot", "copilot"},
		{"aider", "aider"},
		{"codex", "codex"},
		// Real-world variants observed from MCP clients.
		{"claude-ai", "claude"},
		{"claude-code", "claude"},
		{"Cursor", "cursor"},
		{"CURSOR", "cursor"},
		{"  windsurf  ", "windsurf"},
		{"github-copilot", "copilot"},
		{"copilot-cli", "copilot"},
		// Version suffix stripped.
		{"claude/1.2.3", "claude"},
		{"cursor/0.42", "cursor"},
		// Unknown → anonymous (forward-compat).
		{"emacs-llm", ""},
		{"neovim-codecompanion", ""},
		{"random-thing", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveMCPClientEditor(tc.name); got != tc.want {
				t.Errorf("resolveMCPClientEditor(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestResolveMCPClientEditor_CoversAllEditors pins drift between
// harness.AllEditors and the editorAliases table. Adding a new
// editor to AllEditors without an alias row makes this fail with
// the editor name in the message — so the fix-it path is obvious.
func TestResolveMCPClientEditor_CoversAllEditors(t *testing.T) {
	have := make(map[harness.Editor]bool, len(editorAliases))
	for _, a := range editorAliases {
		have[a.editor] = true
	}
	for _, e := range harness.AllEditors {
		if !have[e] {
			t.Errorf("editor %q in harness.AllEditors has no alias in editorAliases — add a row to clientinfo.go", e)
		}
	}
}
