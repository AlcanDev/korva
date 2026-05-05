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
	setupForce  bool
	setupGlobal bool
	setupLocal  bool
)

func init() {
	setupCmd.Flags().BoolVar(&setupAll, "all", false, "Configure all detected editors")
	setupCmd.Flags().BoolVar(&setupVSCode, "vscode", false, "Configure VS Code")
	setupCmd.Flags().BoolVar(&setupCursor, "cursor", false, "Configure Cursor")
	setupCmd.Flags().BoolVar(&setupClaude, "claude", false, "Configure Claude Code")
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
	if !setupVSCode && !setupCursor && !setupClaude && !setupAll {
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
		return fmt.Errorf("not installed")
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

// ── helpers ───────────────────────────────────────────────────────────────────

// isInstalled checks if a binary is available in PATH.
func isInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
