package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure your AI editors to use Korva Vault MCP",
	Long: `Automatically configures VS Code, Cursor, and Claude Code to use the
Korva Vault MCP server. Detects installed editors and updates their
settings files with the correct MCP configuration.

  --global   Write only to global editor config files (no workspace files).
             Use this once after installing Korva to enable it everywhere.
  --local    Write only workspace-level config for the current project
             (e.g. .vscode/mcp.json). Use when adding Korva to a new repo.

Without flags, both global and workspace files are written.
Safe to re-run — it never duplicates settings.`,
	RunE: runSetup,
}

var (
	setupAll    bool
	setupVSCode bool
	setupCursor bool
	setupClaude bool
	setupGemini bool
	setupOpen   bool
	setupCodex  bool
	setupForce  bool
	setupGlobal bool
	setupLocal  bool
)

func init() {
	setupCmd.Flags().BoolVar(&setupAll, "all", false, "Configure all detected editors")
	setupCmd.Flags().BoolVar(&setupVSCode, "vscode", false, "Configure VS Code")
	setupCmd.Flags().BoolVar(&setupCursor, "cursor", false, "Configure Cursor")
	setupCmd.Flags().BoolVar(&setupClaude, "claude", false, "Configure Claude Code")
	setupCmd.Flags().BoolVar(&setupGemini, "gemini-cli", false, "Configure Google Gemini CLI")
	setupCmd.Flags().BoolVar(&setupOpen, "opencode", false, "Configure OpenCode")
	setupCmd.Flags().BoolVar(&setupCodex, "codex", false, "Configure OpenAI Codex CLI")
	setupCmd.Flags().BoolVar(&setupForce, "force", false, "Overwrite existing MCP config even if already set")
	setupCmd.Flags().BoolVar(&setupGlobal, "global", false, "Configure global editor settings only (skip workspace files)")
	setupCmd.Flags().BoolVar(&setupLocal, "local", false, "Write only workspace-level files (.vscode/mcp.json) for the current project")
}

// korvaVaultBin returns the full path to korva-vault or just "korva-vault" if in PATH.
func korvaVaultBin() string {
	if p, err := exec.LookPath("korva-vault"); err == nil {
		return p
	}
	return "korva-vault"
}

// mcpServerEntry is the MCP server entry for korva-vault.
//
//nolint:unused // referenced indirectly via JSON marshaling in editor manifests
type mcpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func runSetup(cmd *cobra.Command, args []string) error {
	if setupGlobal && setupLocal {
		return fmt.Errorf("cannot use --global and --local together")
	}

	bin := korvaVaultBin()

	// Auto-detect if no specific editor flag given
	if !setupVSCode && !setupCursor && !setupClaude && !setupGemini && !setupOpen && !setupCodex && !setupAll {
		setupAll = true
	}

	configured := 0
	skipped := 0

	type editor struct {
		name    string
		enabled bool
		fn      func(string) error
	}

	editors := []editor{
		{"VS Code", setupAll || setupVSCode, func(b string) error { return setupVSCodeEditor(b) }},
		{"Cursor", setupAll || setupCursor, func(b string) error { return setupCursorEditor(b) }},
		{"Claude Code", setupAll || setupClaude, func(b string) error { return setupClaudeCodeEditor(b) }},
		{"Gemini CLI", setupAll || setupGemini, func(b string) error { return setupGeminiCLIEditor(b) }},
		{"OpenCode", setupAll || setupOpen, func(b string) error { return setupOpenCodeEditor(b) }},
		{"Codex CLI", setupAll || setupCodex, func(b string) error { return setupCodexEditor(b) }},
	}

	scope := "global + local"
	if setupGlobal {
		scope = "global"
	} else if setupLocal {
		scope = "local (workspace)"
	}
	fmt.Printf("Korva Setup — configuring AI editors (%s)\n", scope)
	fmt.Println()

	for _, ed := range editors {
		if !ed.enabled {
			continue
		}
		if err := ed.fn(bin); err != nil {
			if strings.Contains(err.Error(), "not installed") {
				printInfo(fmt.Sprintf("%-14s → not installed, skipping", ed.name))
				skipped++
			} else {
				printError(fmt.Sprintf("%-14s → %v", ed.name, err))
			}
		} else {
			configured++
		}
	}

	fmt.Println()
	if configured == 0 && skipped > 0 {
		fmt.Println("No editors found. Install VS Code, Cursor, or Claude Code, then run 'korva setup' again.")
		return nil
	}

	if configured > 0 {
		fmt.Printf("%d editor(s) configured.\n", configured)
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Restart your editor(s) to load the new MCP settings")
		fmt.Println("  2. Start the vault server: korva-vault")
		fmt.Println("  3. In Copilot/Claude/Cursor chat, try: vault_stats")
	}

	return nil
}

// ── VS Code ───────────────────────────────────────────────────────────────────

func setupVSCodeEditor(bin string) error {
	if !isInstalled("code") {
		// Fallback: VS Code installed but the `code` CLI not yet added to PATH
		// (macOS requires "Shell Command: Install 'code' command in PATH" manually).
		// Mirror Cursor/Claude detection: accept if the settings directory exists.
		settingsDir, _ := vscodeSettingsPath()
		if _, statErr := os.Stat(filepath.Dir(settingsDir)); os.IsNotExist(statErr) {
			return fmt.Errorf("not installed")
		}
	}

	// --local: only write workspace file
	if setupLocal {
		wd, _ := os.Getwd()
		workspaceMCP := filepath.Join(wd, ".vscode", "mcp.json")
		if err := os.MkdirAll(filepath.Join(wd, ".vscode"), 0755); err != nil {
			return err
		}
		if err := upsertVSCodeWorkspaceMCP(workspaceMCP, bin); err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("VS Code     → %s", workspaceMCP))
		return nil
	}

	settingsPath, err := vscodeSettingsPath()
	if err != nil {
		return fmt.Errorf("cannot locate VS Code settings: %w", err)
	}

	if err := upsertMCPSettings(settingsPath, bin, "vscode"); err != nil {
		return err
	}

	// Also create workspace .vscode/mcp.json unless --global
	if !setupGlobal {
		wd, _ := os.Getwd()
		workspaceMCP := filepath.Join(wd, ".vscode", "mcp.json")
		if err := os.MkdirAll(filepath.Join(wd, ".vscode"), 0755); err == nil {
			_ = upsertVSCodeWorkspaceMCP(workspaceMCP, bin)
		}
	}

	printSuccess(fmt.Sprintf("VS Code     → %s", settingsPath))
	return nil
}

// upsertVSCodeWorkspaceMCP writes the workspace-level .vscode/mcp.json
func upsertVSCodeWorkspaceMCP(path, bin string) error {
	type workspaceMCP struct {
		Servers map[string]map[string]any `json:"servers"`
	}

	var existing workspaceMCP
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	if existing.Servers == nil {
		existing.Servers = map[string]map[string]any{}
	}

	existing.Servers["korva-vault"] = map[string]any{
		"type":    "stdio",
		"command": bin,
		"args":    []string{"--mode=mcp"},
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func vscodeSettingsPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json"), nil
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Code", "User", "settings.json"), nil
	default: // linux
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "Code", "User", "settings.json"), nil
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "Code", "User", "settings.json"), nil
	}
}

// ── Cursor ────────────────────────────────────────────────────────────────────

func setupCursorEditor(bin string) error {
	mcpPath, err := cursorMCPPath()
	if err != nil {
		return fmt.Errorf("cannot locate Cursor config: %w", err)
	}

	// Detect Cursor: check binary in PATH OR ~/.cursor dir already exists
	if !isInstalled("cursor") {
		if _, statErr := os.Stat(filepath.Dir(mcpPath)); os.IsNotExist(statErr) {
			return fmt.Errorf("not installed")
		}
	}

	if err := upsertCursorMCP(mcpPath, bin); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Cursor      → %s", mcpPath))
	return nil
}

func cursorMCPPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cursor", "mcp.json"), nil
	case "windows":
		return filepath.Join(os.Getenv("USERPROFILE"), ".cursor", "mcp.json"), nil
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cursor", "mcp.json"), nil
	}
}

func upsertCursorMCP(path, bin string) error {
	type cursorMCP struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}

	var existing cursorMCP
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	if existing.MCPServers == nil {
		existing.MCPServers = map[string]map[string]any{}
	}

	existing.MCPServers["korva-vault"] = map[string]any{
		"command": bin,
		"args":    []string{"--mode=mcp"},
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── Claude Code ───────────────────────────────────────────────────────────────

func setupClaudeCodeEditor(bin string) error {
	mcpPath, err := claudeSettingsPath()
	if err != nil {
		return fmt.Errorf("cannot locate Claude Code settings: %w", err)
	}

	// Detect Claude Code: check binary in PATH OR config directory already exists
	if !isInstalled("claude") {
		if _, statErr := os.Stat(filepath.Dir(mcpPath)); os.IsNotExist(statErr) {
			return fmt.Errorf("not installed")
		}
	}

	if err := upsertClaudeMCP(mcpPath, bin); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Claude Code → %s", mcpPath))
	return nil
}

func claudeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		// Claude Code on Windows stores settings in %APPDATA%\Claude\
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Claude", "settings.json"), nil
	default:
		return filepath.Join(home, ".claude", "settings.json"), nil
	}
}

func upsertClaudeMCP(path, bin string) error {
	type claudeSettings struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}

	var existing claudeSettings
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	if existing.MCPServers == nil {
		existing.MCPServers = map[string]map[string]any{}
	}

	existing.MCPServers["korva-vault"] = map[string]any{
		"command": bin,
		"args":    []string{"--mode=mcp"},
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── VS Code user settings (global MCP config) ─────────────────────────────────

func upsertMCPSettings(path, bin, editor string) error {
	// Read existing settings (may not exist)
	var settings map[string]any
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}

	// Upsert the MCP servers section
	mcp, _ := settings["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	servers, _ := mcp["servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}

	servers["korva-vault"] = map[string]any{
		"type":    "stdio",
		"command": bin,
		"args":    []string{"--mode=mcp"},
	}
	mcp["servers"] = servers
	settings["mcp"] = mcp

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── Gemini CLI ───────────────────────────────────────────────────────────────
//
// Google's Gemini CLI keeps its config in ~/.gemini/settings.json with the
// MCP servers under the conventional `mcpServers` key (same shape as Cursor
// and Claude Code).

func setupGeminiCLIEditor(bin string) error {
	mcpPath, err := geminiSettingsPath()
	if err != nil {
		return fmt.Errorf("cannot locate Gemini CLI config: %w", err)
	}
	if !isInstalled("gemini") {
		if _, statErr := os.Stat(filepath.Dir(mcpPath)); os.IsNotExist(statErr) {
			return fmt.Errorf("not installed")
		}
	}
	if err := upsertMCPServersFile(mcpPath, bin); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Gemini CLI  → %s", mcpPath))
	return nil
}

func geminiSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		// Windows: prefer %USERPROFILE%\.gemini\ to match the Unix layout
		profile := os.Getenv("USERPROFILE")
		if profile == "" {
			profile = home
		}
		return filepath.Join(profile, ".gemini", "settings.json"), nil
	default:
		return filepath.Join(home, ".gemini", "settings.json"), nil
	}
}

// ── OpenCode ──────────────────────────────────────────────────────────────────
//
// OpenCode (the open-source AI coding agent) keeps user config in the XDG
// config dir, with MCP servers under an `mcp` map. We treat each server as
// a stdio entry — that's the only transport korva-vault speaks.

func setupOpenCodeEditor(bin string) error {
	mcpPath, err := opencodeConfigPath()
	if err != nil {
		return fmt.Errorf("cannot locate OpenCode config: %w", err)
	}
	if !isInstalled("opencode") {
		if _, statErr := os.Stat(filepath.Dir(mcpPath)); os.IsNotExist(statErr) {
			return fmt.Errorf("not installed")
		}
	}
	if err := upsertOpenCodeMCP(mcpPath, bin); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("OpenCode    → %s", mcpPath))
	return nil
}

func opencodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "opencode", "opencode.json"), nil
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "opencode", "opencode.json"), nil
		}
		return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
	}
}

func upsertOpenCodeMCP(path, bin string) error {
	// We round-trip the file as a generic map so we don't drop unknown
	// top-level keys (theme, autosave, $schema, etc.) that the user may
	// have hand-tuned.
	var raw map[string]any
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = map[string]any{}
	}
	mcp, _ := raw["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	mcp["korva-vault"] = map[string]any{
		"type":    "local",
		"command": []string{bin, "--mode=mcp"},
		"enabled": true,
	}
	raw["mcp"] = mcp

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── Codex CLI ────────────────────────────────────────────────────────────────
//
// OpenAI's Codex CLI uses TOML at ~/.codex/config.toml. We avoid pulling in a
// TOML library for one section — instead we read the file, look for an
// existing [mcp_servers.korva-vault] header, and either skip (no --force) or
// append the canonical section block at EOF.

func setupCodexEditor(bin string) error {
	cfgPath, err := codexConfigPath()
	if err != nil {
		return fmt.Errorf("cannot locate Codex config: %w", err)
	}
	if !isInstalled("codex") {
		if _, statErr := os.Stat(filepath.Dir(cfgPath)); os.IsNotExist(statErr) {
			return fmt.Errorf("not installed")
		}
	}
	if err := upsertCodexMCP(cfgPath, bin); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Codex CLI   → %s", cfgPath))
	return nil
}

func codexConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		profile := os.Getenv("USERPROFILE")
		if profile == "" {
			profile = home
		}
		return filepath.Join(profile, ".codex", "config.toml"), nil
	default:
		return filepath.Join(home, ".codex", "config.toml"), nil
	}
}

// codexMCPBlock is the canonical TOML stanza we append for korva-vault.
const codexMCPBlock = `
[mcp_servers.korva-vault]
command = %q
args = ["--mode=mcp"]
`

func upsertCodexMCP(path, bin string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	existing, _ := os.ReadFile(path)
	header := "[mcp_servers.korva-vault]"
	if strings.Contains(string(existing), header) && !setupForce {
		return nil // already configured — leave the user's hand-tuned settings alone
	}
	if strings.Contains(string(existing), header) && setupForce {
		// Strip the previous block before re-appending so we don't have two.
		existing = []byte(stripCodexBlock(string(existing), header))
	}
	// Ensure file ends with a newline before appending.
	out := string(existing)
	if len(out) > 0 && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += fmt.Sprintf(codexMCPBlock, bin)
	return os.WriteFile(path, []byte(out), 0644)
}

// stripCodexBlock removes a previous "[mcp_servers.korva-vault]" stanza so
// --force can rewrite it cleanly. The stanza ends at the next "[" section
// header or EOF, whichever comes first.
func stripCodexBlock(content, header string) string {
	idx := strings.Index(content, header)
	if idx < 0 {
		return content
	}
	// Find the start of the line containing the header.
	start := idx
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	// Find the end of the stanza: next line starting with "[" or EOF.
	rest := content[idx+len(header):]
	endRel := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\n' && i+1 < len(rest) && rest[i+1] == '[' {
			endRel = i + 1
			break
		}
	}
	if endRel < 0 {
		return content[:start]
	}
	return content[:start] + rest[endRel:]
}

// ── shared upsert: mcpServers JSON shape ─────────────────────────────────────
//
// Reused by Gemini CLI (and other JSON editors with the same shape) so we
// don't repeat the unmarshal/upsert dance for every new editor.

func upsertMCPServersFile(path, bin string) error {
	type mcpServersFile struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	var existing mcpServersFile
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	if existing.MCPServers == nil {
		existing.MCPServers = map[string]map[string]any{}
	}
	existing.MCPServers["korva-vault"] = map[string]any{
		"command": bin,
		"args":    []string{"--mode=mcp"},
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// isInstalled checks if a binary is available in PATH.
func isInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
