package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ── top-level command ─────────────────────────────────────────────────────────

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage and sync AI skills for your team",
}

// ── flags ─────────────────────────────────────────────────────────────────────

var (
	skillsSyncGlobal  bool
	skillsSyncProject string
	skillsSyncQuiet   bool
)

func init() {
	skillsSyncCmd.Flags().BoolVar(&skillsSyncGlobal, "global", true,
		"sync skills to the global ~/.claude/ directory (default)")
	skillsSyncCmd.Flags().StringVar(&skillsSyncProject, "project", "",
		"sync skills to a specific project .claude/ directory")
	skillsSyncCmd.Flags().BoolVar(&skillsSyncQuiet, "quiet", false,
		"suppress output (useful in hooks)")

	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsSyncCmd)
	skillsCmd.AddCommand(skillsHistoryCmd)
	skillsCmd.AddCommand(skillsHookCmd)
	skillsHookCmd.AddCommand(skillsHookInstallCmd)
	skillsHookCmd.AddCommand(skillsHookRemoveCmd)
}

// ── korva skills list ─────────────────────────────────────────────────────────

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all skills available for your team",
	RunE:  runSkillsList,
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	token, err := readSessionToken(paths.SessionTokenFile)
	if err != nil {
		return fmt.Errorf("not authenticated — run 'korva auth redeem <invite-token>'")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", vaultBase()+"/team/skills", nil)
	req.Header.Set("X-Session-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault unreachable (%w)", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("skills list failed (status %d)", resp.StatusCode)
	}

	var result struct {
		Skills []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Version   int    `json:"version"`
			UpdatedBy string `json:"updated_by"`
			Scope     string `json:"scope"`
			UpdatedAt string `json:"updated_at"`
		} `json:"skills"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("unexpected response: %w", err)
	}

	if result.Count == 0 {
		printInfo("No skills found — save a skill via the Beacon dashboard or 'korva skills publish'")
		return nil
	}

	fmt.Printf("\n  %-30s  %3s  %-22s  %-6s  %s\n", "NAME", "VER", "UPDATED BY", "SCOPE", "UPDATED AT")
	fmt.Printf("  %-30s  %3s  %-22s  %-6s  %s\n",
		strings.Repeat("─", 30), "───", strings.Repeat("─", 22), "──────", strings.Repeat("─", 20))
	for _, sk := range result.Skills {
		updatedAt := sk.UpdatedAt
		if t, err := time.Parse(time.RFC3339, sk.UpdatedAt); err == nil {
			updatedAt = t.Local().Format("2006-01-02 15:04")
		}
		by := sk.UpdatedBy
		if by == "" {
			by = "—"
		}
		fmt.Printf("  %-30s  %3d  %-22s  %-6s  %s\n",
			sk.Name, sk.Version, by, sk.Scope, updatedAt)
	}
	fmt.Printf("\n  %d skill(s) available\n\n", result.Count)
	return nil
}

// ── korva skills sync ─────────────────────────────────────────────────────────

var skillsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync team skills to the local Claude Code directory",
	Long: `Pull skills from the Vault and write them as .md files in ~/.claude/ (global)
or .claude/ (project). Only skills changed since the last sync are downloaded.

The last sync timestamp is stored in ~/.korva/skills_sync.json.`,
	RunE: runSkillsSync,
}

type syncState struct {
	LastSync string `json:"last_sync"`
	Target   string `json:"target"`
}

func runSkillsSync(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	token, err := readSessionToken(paths.SessionTokenFile)
	if err != nil {
		return fmt.Errorf("not authenticated — run 'korva auth redeem <invite-token>'")
	}

	// Determine target directory.
	targetDir, err := resolveSkillsTargetDir(skillsSyncProject, skillsSyncGlobal)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	// Load last sync state.
	state := loadSyncState(paths.SkillsSyncFile)

	// Build sync URL with optional ?since filter.
	syncURL := vaultBase() + "/team/skills/sync"
	if state.LastSync != "" {
		syncURL += "?since=" + url.QueryEscape(state.LastSync)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", syncURL, nil)
	req.Header.Set("X-Session-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault unreachable (%w)", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sync failed (status %d)", resp.StatusCode)
	}

	var result struct {
		Skills []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Body      string `json:"body"`
			Tags      string `json:"tags"`
			Version   int    `json:"version"`
			UpdatedBy string `json:"updated_by"`
			Scope     string `json:"scope"`
			UpdatedAt string `json:"updated_at"`
			Deleted   bool   `json:"deleted"`
		} `json:"skills"`
		SyncedAt string `json:"synced_at"`
		Count    int    `json:"count"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("unexpected response: %w", err)
	}

	var updated, removed int
	for _, sk := range result.Skills {
		filename := sanitizeSkillFilename(sk.Name) + ".md"
		localPath := filepath.Join(targetDir, filename)

		if sk.Deleted {
			if err := os.Remove(localPath); err == nil {
				removed++
				if !skillsSyncQuiet {
					printInfo(fmt.Sprintf("removed  %s", filename))
				}
			}
			continue
		}

		content := buildSkillFileContent(sk.Name, sk.Body, sk.Version, sk.UpdatedBy, sk.UpdatedAt, result.SyncedAt)
		if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
			if !skillsSyncQuiet {
				printError(fmt.Sprintf("write %s: %v", filename, err))
			}
			continue
		}
		updated++
		if !skillsSyncQuiet {
			printSuccess(fmt.Sprintf("synced   %s  (v%d)", filename, sk.Version))
		}
	}

	// Persist new sync state.
	newState := syncState{LastSync: result.SyncedAt, Target: targetDir}
	saveSyncState(paths.SkillsSyncFile, newState)

	// Report sync telemetry — best-effort, never fails the command.
	reportSyncTelemetry(token, updated+removed, targetDir)

	if !skillsSyncQuiet {
		if updated == 0 && removed == 0 {
			printInfo(fmt.Sprintf("skills up to date (synced at %s)", fmtTimestamp(result.SyncedAt)))
		} else {
			fmt.Printf("\n  %d updated, %d removed — synced at %s\n\n",
				updated, removed, fmtTimestamp(result.SyncedAt))
		}
	}
	return nil
}

// reportSyncTelemetry posts a sync event to the Vault so the admin can see
// who has synced. Silently ignored on any error — telemetry must never block sync.
func reportSyncTelemetry(token string, count int, target string) {
	type payload struct {
		SkillsCount int    `json:"skills_count"`
		Target      string `json:"target"`
	}
	body, _ := json.Marshal(payload{SkillsCount: count, Target: target})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST",
		vaultBase()+"/team/skills/sync/report",
		strings.NewReader(string(body)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// buildSkillFileContent renders the skill body with a YAML front-matter header
// that Claude Code uses to identify the skill name and description.
func buildSkillFileContent(name, body string, version int, updatedBy, updatedAt, syncedAt string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", name))
	sb.WriteString(fmt.Sprintf("version: %d\n", version))
	if updatedBy != "" {
		sb.WriteString(fmt.Sprintf("updated_by: %s\n", updatedBy))
	}
	if updatedAt != "" {
		sb.WriteString(fmt.Sprintf("updated_at: %s\n", updatedAt))
	}
	if syncedAt != "" {
		sb.WriteString(fmt.Sprintf("synced_at: %s\n", syncedAt))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

// sanitizeSkillFilename converts a skill name to a safe filename.
// Spaces → hyphens, strips non-alphanumeric chars except hyphens and underscores.
func sanitizeSkillFilename(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")
	var sb strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

// resolveSkillsTargetDir returns the directory where skill files should be written.
// If projectPath is set, returns <projectPath>/.claude/.
// Otherwise returns ~/.claude/.
func resolveSkillsTargetDir(projectPath string, global bool) (string, error) {
	if projectPath != "" {
		abs, err := filepath.Abs(projectPath)
		if err != nil {
			return "", fmt.Errorf("invalid project path: %w", err)
		}
		return filepath.Join(abs, ".claude"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

func loadSyncState(path string) syncState {
	data, err := os.ReadFile(path)
	if err != nil {
		return syncState{}
	}
	var s syncState
	json.Unmarshal(data, &s) //nolint:errcheck
	return s
}

func saveSyncState(path string, s syncState) {
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(path, data, 0644) //nolint:errcheck
}

func fmtTimestamp(ts string) string {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Local().Format("2006-01-02 15:04:05")
	}
	return ts
}

// ── korva skills history ──────────────────────────────────────────────────────

var skillsHistoryCmd = &cobra.Command{
	Use:   "history <skill-name>",
	Short: "Show version history for a skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsHistory,
}

func runSkillsHistory(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	token, err := readSessionToken(paths.SessionTokenFile)
	if err != nil {
		return fmt.Errorf("not authenticated — run 'korva auth redeem <invite-token>'")
	}

	skillName := args[0]
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	// Find skill ID by name via the list endpoint.
	req, _ := http.NewRequestWithContext(ctx, "GET", vaultBase()+"/team/skills", nil)
	req.Header.Set("X-Session-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault unreachable (%w)", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var listResult struct {
		Skills []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"skills"`
	}
	json.Unmarshal(raw, &listResult) //nolint:errcheck

	var skillID string
	for _, sk := range listResult.Skills {
		if strings.EqualFold(sk.Name, skillName) {
			skillID = sk.ID
			break
		}
	}
	if skillID == "" {
		return fmt.Errorf("skill %q not found", skillName)
	}

	// Fetch history.
	ctx2, cancel2 := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel2()
	req2, _ := http.NewRequestWithContext(ctx2, "GET",
		vaultBase()+"/team/skills/"+skillID+"/history", nil)
	req2.Header.Set("X-Session-Token", token)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return fmt.Errorf("vault unreachable (%w)", err)
	}
	defer resp2.Body.Close()
	raw2, _ := io.ReadAll(resp2.Body)

	var histResult struct {
		History []struct {
			Version   int    `json:"version"`
			ChangedBy string `json:"changed_by"`
			Summary   string `json:"summary"`
			ChangedAt string `json:"changed_at"`
		} `json:"history"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(raw2, &histResult); err != nil {
		return fmt.Errorf("unexpected response: %w", err)
	}

	fmt.Printf("\n  History for: %s (%d versions)\n\n", skillName, histResult.Count)
	for _, h := range histResult.History {
		ts := fmtTimestamp(h.ChangedAt)
		by := h.ChangedBy
		if by == "" {
			by = "unknown"
		}
		summary := h.Summary
		if summary == "" {
			summary = "(no summary)"
		}
		fmt.Printf("  v%-3d  %s  by %-24s  %s\n", h.Version, ts, by, summary)
	}
	fmt.Println()
	return nil
}

// ── korva skills hook ─────────────────────────────────────────────────────────

var skillsHookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Manage the Claude Code auto-sync hook",
}

var skillsHookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install auto-sync hook in ~/.claude/settings.json",
	Long: `Installs a PreToolUse hook in your global Claude Code settings so that
'korva skills sync --quiet' runs automatically before every Claude Code session.`,
	RunE: runSkillsHookInstall,
}

var skillsHookRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the auto-sync hook from ~/.claude/settings.json",
	RunE:  runSkillsHookRemove,
}

const hookCommand = "korva skills sync --quiet"

func runSkillsHookInstall(_ *cobra.Command, _ []string) error {
	settingsPath, err := claudeSettingsPath()
	if err != nil {
		return err
	}

	settings, err := loadClaudeSettings(settingsPath)
	if err != nil {
		return err
	}

	if hookAlreadyInstalled(settings) {
		printInfo("Hook already installed in " + settingsPath)
		return nil
	}

	injectHook(settings)

	if err := saveClaudeSettings(settingsPath, settings); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	printSuccess("Hook installed → skills will auto-sync before each Claude Code session")
	printInfo(settingsPath)
	return nil
}

func runSkillsHookRemove(_ *cobra.Command, _ []string) error {
	settingsPath, err := claudeSettingsPath()
	if err != nil {
		return err
	}

	settings, err := loadClaudeSettings(settingsPath)
	if err != nil {
		return err
	}

	if !hookAlreadyInstalled(settings) {
		printInfo("Hook not found in " + settingsPath)
		return nil
	}

	removeHook(settings)

	if err := saveClaudeSettings(settingsPath, settings); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	printSuccess("Hook removed from " + settingsPath)
	return nil
}

// loadClaudeSettings reads ~/.claude/settings.json into a raw map.
// Returns an empty map when the file does not exist yet.
func loadClaudeSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading settings: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing settings.json: %w", err)
	}
	return m, nil
}

func saveClaudeSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// hookAlreadyInstalled returns true if the korva sync hook is already present.
func hookAlreadyInstalled(settings map[string]any) bool {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}
	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		return false
	}
	for _, entry := range preToolUse {
		group, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		innerHooks, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if hm["command"] == hookCommand {
				return true
			}
		}
	}
	return false
}

// injectHook adds the korva sync PreToolUse hook to the settings map.
func injectHook(settings map[string]any) {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}

	newEntry := map[string]any{
		"matcher": ".*",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCommand,
			},
		},
	}

	switch existing := hooks["PreToolUse"].(type) {
	case []any:
		hooks["PreToolUse"] = append(existing, newEntry)
	default:
		hooks["PreToolUse"] = []any{newEntry}
	}
}

// removeHook removes all PreToolUse entries whose command matches hookCommand.
func removeHook(settings map[string]any) {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return
	}
	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		return
	}

	filtered := make([]any, 0, len(preToolUse))
	for _, entry := range preToolUse {
		group, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		innerHooks, ok := group["hooks"].([]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		// Keep the group only if none of its hooks are ours.
		hasOurs := false
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if ok && hm["command"] == hookCommand {
				hasOurs = true
				break
			}
		}
		if !hasOurs {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
	}
}

