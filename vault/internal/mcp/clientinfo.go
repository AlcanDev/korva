package mcp

import (
	"strings"

	"github.com/alcandev/korva/internal/harness"
)

// Phase 19.A — map MCP initialize `clientInfo.name` to a harness
// editor id. Mirrors the policy of the HTTP X-Korva-Editor header:
// unknown names normalize to "" (anonymous) rather than 400-ing,
// so a new editor's MCP client doesn't have to wait for a vault
// release to flow into the adoption stats.
//
// MCP spec: https://spec.modelcontextprotocol.io/specification/architecture/#initialization
// clientInfo carries `{name, version}` and clients self-report
// (Claude Code sends "claude-ai", Cursor "cursor", etc.). We
// match generously — substring + lowercase — because vendors have
// historically used different exact strings across versions.

// editorAliases maps a normalized clientInfo.name fragment to its
// harness editor id. The lookup is "first substring match wins" in
// the order declared here; more specific aliases must come first
// when ambiguity would otherwise pick the wrong editor.
//
// Add a new editor in two steps: append a constant to
// internal/harness.AllEditors, then add a row here. The
// drift-pinning test `TestResolveMCPClientEditor_CoversAllEditors`
// fails until you've done both.
var editorAliases = []struct {
	fragment string
	editor   harness.Editor
}{
	// Claude Code reports "claude-ai", "claude-code", or just
	// "claude" depending on release; match the prefix.
	{"claude", harness.EditorClaude},
	{"cursor", harness.EditorCursor},
	{"windsurf", harness.EditorWindsurf},
	{"continue", harness.EditorContinue},
	// GitHub Copilot CLI client identifies as "copilot-cli" /
	// "github-copilot"; substring match catches both.
	{"copilot", harness.EditorCopilot},
	{"aider", harness.EditorAider},
	// OpenAI Codex CLI reports "codex" in clientInfo per its
	// docs/protocol.md.
	{"codex", harness.EditorCodex},
}

// resolveMCPClientEditor normalizes name (lower-case, trim, no
// version suffix like "claude/1.2.3") and returns the matching
// harness editor id, or "" when nothing matches.
func resolveMCPClientEditor(name string) string {
	if name == "" {
		return ""
	}
	// Strip "/version" suffix MCP clients sometimes append.
	if i := strings.IndexByte(name, '/'); i >= 0 {
		name = name[:i]
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	for _, a := range editorAliases {
		if strings.Contains(name, a.fragment) {
			return string(a.editor)
		}
	}
	return ""
}
