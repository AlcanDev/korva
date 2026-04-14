package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/profile"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Korva in the current project",
	Long: `Initialize Korva: creates ~/.korva/ directories, writes korva.config.json,
detects your AI agent, and optionally applies a team profile.

Run without flags for an interactive TUI setup, or pass flags for scripted setup.`,
	RunE: runInit,
}

var (
	initProfile string
	initAdmin   bool
	initOwner   string
	initAgent   string
	initDryRun  bool
)

func init() {
	initCmd.Flags().StringVar(&initProfile, "profile", "", "Team profile repo URL (e.g., git@github.com:org/korva-profile.git)")
	initCmd.Flags().BoolVar(&initAdmin, "admin", false, "Initialize as admin (generates admin.key)")
	initCmd.Flags().StringVar(&initOwner, "owner", "", "Admin owner email (used with --admin)")
	initCmd.Flags().StringVar(&initAgent, "agent", "", "AI agent: copilot | claude | cursor (auto-detected if not set)")
	initCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Show what would be done without making changes")
}

func runInit(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return fmt.Errorf("cannot determine platform paths: %w", err)
	}

	if initDryRun {
		fmt.Println("Dry run — no changes will be made:")
		printInfo(fmt.Sprintf("Would create %s", paths.HomeDir))
		if initAdmin {
			printInfo(fmt.Sprintf("Would create %s (admin key)", paths.AdminKey))
		}
		if initProfile != "" {
			printInfo(fmt.Sprintf("Would clone team profile from %s", initProfile))
		}
		return nil
	}

	fmt.Println("Initializing Korva...")

	// 1. Create all directories
	if err := paths.EnsureAll(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}
	printSuccess(fmt.Sprintf("Created %s", paths.HomeDir))

	// 2. Generate admin key if requested
	if initAdmin {
		if initOwner == "" {
			return fmt.Errorf("--owner is required with --admin (e.g., --owner=you@domain.com)")
		}
		cfg, err := admin.Generate(paths.AdminKey, initOwner, false)
		if err != nil {
			return fmt.Errorf("generating admin key: %w", err)
		}
		printSuccess(fmt.Sprintf("Generated admin key for %s (v%d)", cfg.Owner, cfg.Version))
		fmt.Printf("\n  Admin key created at: %s\n", paths.AdminKey)
		fmt.Printf("  Keep this file secure — it grants full admin access to Vault.\n\n")
	}

	// 3. Detect agent if not specified
	if initAgent == "" {
		initAgent = detectAgent()
	}
	printSuccess(fmt.Sprintf("Agent: %s", initAgent))

	// 4. Write base config if not exists
	if _, err := os.Stat(paths.ConfigFile); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		cfg.Agent = initAgent
		if err := config.Save(cfg, paths.ConfigFile); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
		printSuccess("Created ~/.korva/config.json")
	}

	// 5. Apply team profile if specified
	if initProfile != "" {
		printInfo(fmt.Sprintf("Cloning team profile from %s ...", initProfile))
		mgr := profile.NewManager(paths)

		profileDir, err := mgr.Clone(initProfile)
		if err != nil {
			return fmt.Errorf("cloning team profile: %w", err)
		}
		printSuccess("Team profile cloned")

		baseCfg, err := config.Load(paths.ConfigFile)
		if err != nil {
			return fmt.Errorf("loading base config: %w", err)
		}

		if _, err := mgr.Apply(profileDir, baseCfg); err != nil {
			return fmt.Errorf("applying team profile: %w", err)
		}
		printSuccess("Team profile applied")

		if err := mgr.InstallScrolls(profileDir); err != nil {
			return fmt.Errorf("installing private scrolls: %w", err)
		}
		printSuccess("Private scrolls installed")

		// Merge instructions into current project
		wd, _ := os.Getwd()
		if err := mgr.MergeInstructions(profileDir, wd); err != nil {
			printInfo(fmt.Sprintf("Could not merge instructions: %v (skipping)", err))
		} else {
			printSuccess("Team instructions merged into project")
		}
	}

	// 6. Write korva.config.json in current project if not exists
	projectConfig := "korva.config.json"
	if _, err := os.Stat(projectConfig); os.IsNotExist(err) {
		template := map[string]any{
			"$schema": "https://korva.dev/schemas/config/v1.json",
			"version": "1",
			"project": filepath.Base(currentDir()),
			"team":    "",
			"country": "CL",
			"vault":   map[string]any{"port": 7437, "auto_start": true},
			"lore":    map[string]any{"active_scrolls": []string{"forge-sdd"}},
			"sentinel": map[string]any{"enabled": true},
			"agent":   initAgent,
		}
		data, _ := json.MarshalIndent(template, "", "  ")
		os.WriteFile(projectConfig, data, 0644)
		printSuccess("Created korva.config.json in current directory")
	}

	fmt.Println("\nKorva initialized successfully.")
	fmt.Println("\nNext steps:")
	fmt.Println("  korva sentinel install   — install pre-commit hooks")
	fmt.Println("  korva status             — verify your setup")
	fmt.Println("  korva doctor             — run health checks")

	return nil
}

func detectAgent() string {
	cwd, _ := os.Getwd()
	// Check for Copilot
	if fileExists(filepath.Join(cwd, ".github", "copilot-instructions.md")) ||
		fileExists(filepath.Join(cwd, ".vscode", "settings.json")) {
		return "copilot"
	}
	// Check for Claude Code
	if fileExists(filepath.Join(cwd, "CLAUDE.md")) ||
		fileExists(filepath.Join(cwd, ".claude")) {
		return "claude"
	}
	// Check for Cursor
	if fileExists(filepath.Join(cwd, ".cursorrules")) ||
		fileExists(filepath.Join(cwd, ".cursor")) {
		return "cursor"
	}
	return "copilot"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func currentDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "my-project"
	}
	return wd
}
