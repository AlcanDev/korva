package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests cover the per-editor upsert helpers that drive `korva setup`.
// They write to a t.TempDir() rather than the real ~/.gemini etc., so they
// stay hermetic and safe to run in CI.

// ── Gemini CLI (JSON mcpServers shape) ───────────────────────────────────────

func TestUpsertMCPServersFile_CreatesFileWithKorvaVault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := upsertMCPServersFile(path, "/usr/local/bin/korva-vault"); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var got struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	srv, ok := got.MCPServers["korva-vault"]
	if !ok {
		t.Fatalf("korva-vault entry missing, got %+v", got.MCPServers)
	}
	if srv["command"] != "/usr/local/bin/korva-vault" {
		t.Errorf("command = %v, want /usr/local/bin/korva-vault", srv["command"])
	}
	args, _ := srv["args"].([]any)
	if len(args) != 1 || args[0] != "--mode=mcp" {
		t.Errorf("args = %v, want [--mode=mcp]", args)
	}
}

func TestUpsertMCPServersFile_PreservesOtherServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	initial := `{"mcpServers":{"other":{"command":"other-bin"}}}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := upsertMCPServersFile(path, "korva-vault"); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var got struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	data, _ := os.ReadFile(path)
	_ = json.Unmarshal(data, &got)
	if _, ok := got.MCPServers["other"]; !ok {
		t.Error("upsert dropped the pre-existing 'other' server")
	}
	if _, ok := got.MCPServers["korva-vault"]; !ok {
		t.Error("upsert did not add korva-vault")
	}
}

// ── OpenCode (JSON with `mcp` map) ───────────────────────────────────────────

func TestUpsertOpenCodeMCP_WritesLocalCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")

	if err := upsertOpenCodeMCP(path, "/opt/korva-vault"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var got map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	mcp, _ := got["mcp"].(map[string]any)
	srv, _ := mcp["korva-vault"].(map[string]any)
	if srv == nil {
		t.Fatalf("korva-vault entry missing: %+v", got)
	}
	if srv["type"] != "local" {
		t.Errorf("type = %v, want local", srv["type"])
	}
	cmd, _ := srv["command"].([]any)
	if len(cmd) != 2 || cmd[0] != "/opt/korva-vault" || cmd[1] != "--mode=mcp" {
		t.Errorf("command = %v, want [/opt/korva-vault --mode=mcp]", cmd)
	}
	if enabled, _ := srv["enabled"].(bool); !enabled {
		t.Error("enabled should be true")
	}
}

func TestUpsertOpenCodeMCP_PreservesUnknownTopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	initial := `{"theme":"tokyonight","autosave":true,"mcp":{"other":{"type":"local"}}}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := upsertOpenCodeMCP(path, "korva-vault"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var got map[string]any
	data, _ := os.ReadFile(path)
	_ = json.Unmarshal(data, &got)
	if got["theme"] != "tokyonight" {
		t.Errorf("theme dropped: %+v", got)
	}
	if got["autosave"] != true {
		t.Errorf("autosave dropped: %+v", got)
	}
	mcp, _ := got["mcp"].(map[string]any)
	if _, ok := mcp["other"]; !ok {
		t.Error("pre-existing mcp.other was lost")
	}
}

// ── Codex CLI (TOML append-if-absent) ────────────────────────────────────────

func TestUpsertCodexMCP_AppendsBlockWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	initial := "model = \"gpt-4.1\"\n[other]\nfoo = 1\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := upsertCodexMCP(path, "/usr/local/bin/korva-vault"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	data, _ := os.ReadFile(path)
	out := string(data)
	if !strings.Contains(out, "[mcp_servers.korva-vault]") {
		t.Errorf("missing korva-vault stanza:\n%s", out)
	}
	if !strings.Contains(out, `command = "/usr/local/bin/korva-vault"`) {
		t.Errorf("missing command line:\n%s", out)
	}
	if !strings.Contains(out, "[other]") || !strings.Contains(out, "foo = 1") {
		t.Errorf("pre-existing [other] table lost:\n%s", out)
	}
}

func TestUpsertCodexMCP_IdempotentWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := upsertCodexMCP(path, "/bin/korva-vault"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	first, _ := os.ReadFile(path)

	// Re-run without --force: file should be unchanged.
	setupForce = false
	if err := upsertCodexMCP(path, "/different/path"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Errorf("second run mutated the file:\nbefore=%s\nafter=%s", first, second)
	}
}

func TestUpsertCodexMCP_ForceReplacesBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := upsertCodexMCP(path, "/old/path"); err != nil {
		t.Fatal(err)
	}
	// Switch to --force and rewrite with a different bin.
	setupForce = true
	t.Cleanup(func() { setupForce = false })
	if err := upsertCodexMCP(path, "/new/path"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, "/old/path") {
		t.Errorf("force did not strip the old block:\n%s", out)
	}
	if !strings.Contains(out, `command = "/new/path"`) {
		t.Errorf("force did not write the new block:\n%s", out)
	}
	// And there must still be exactly one stanza for korva-vault.
	if strings.Count(out, "[mcp_servers.korva-vault]") != 1 {
		t.Errorf("expected exactly one korva-vault stanza, got %d:\n%s",
			strings.Count(out, "[mcp_servers.korva-vault]"), out)
	}
}

func TestStripCodexBlock_ReturnsContentMinusStanza(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "stanza followed by another section",
			in: `model = "gpt-4.1"

[mcp_servers.korva-vault]
command = "x"
args = ["a"]

[other]
foo = 1
`,
			want: `model = "gpt-4.1"

[other]
foo = 1
`,
		},
		{
			name: "stanza at end of file",
			in: `model = "gpt-4.1"

[mcp_servers.korva-vault]
command = "x"
`,
			want: `model = "gpt-4.1"

`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripCodexBlock(tc.in, "[mcp_servers.korva-vault]")
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}
