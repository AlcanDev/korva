package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
)

var loreCmd = &cobra.Command{
	Use:   "lore",
	Short: "Manage knowledge Scrolls",
}

var loreListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available and active Scrolls",
	RunE:  runLoreList,
}

var loreAddCmd = &cobra.Command{
	Use:   "add [scroll-name]",
	Short: "Add a Scroll to the active project config",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoreAdd,
}

func init() {
	loreCmd.AddCommand(loreListCmd)
	loreCmd.AddCommand(loreAddCmd)
}

func runLoreList(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	cfg, _ := config.Load(paths.ConfigFile)
	activeScrolls := make(map[string]bool)
	for _, s := range cfg.Lore.ActiveScrolls {
		activeScrolls[s] = true
	}

	fmt.Println("Available Scrolls:")
	fmt.Println("")

	// Public scrolls from korva lore/curated/
	korvaLore := findKorvaLoreDir()
	if korvaLore != "" {
		entries, _ := os.ReadDir(korvaLore)
		for _, e := range entries {
			if e.IsDir() {
				name := e.Name()
				active := ""
				if activeScrolls[name] {
					active = " [active]"
				}
				fmt.Printf("  · %-30s%s\n", name, active)
			}
		}
	}

	// Private scrolls
	privateScrolls, _ := os.ReadDir(paths.PrivateLoreDir())
	if len(privateScrolls) > 0 {
		fmt.Println("\nPrivate Scrolls (team):")
		for _, e := range privateScrolls {
			if e.IsDir() {
				name := e.Name()
				active := ""
				if activeScrolls[name] {
					active = " [active]"
				}
				fmt.Printf("  · %-30s%s\n", name, active)
			}
		}
	}

	return nil
}

func runLoreAdd(cmd *cobra.Command, args []string) error {
	scrollName := args[0]

	// Validate scroll name
	if strings.ContainsAny(scrollName, "/\\.") {
		return fmt.Errorf("invalid scroll name: %s", scrollName)
	}

	// Update korva.config.json in current dir
	projectCfg, err := config.Load("korva.config.json")
	if err != nil {
		return fmt.Errorf("loading project config: %w — run 'korva init' first", err)
	}

	for _, s := range projectCfg.Lore.ActiveScrolls {
		if s == scrollName {
			printInfo(fmt.Sprintf("Scroll '%s' is already active", scrollName))
			return nil
		}
	}

	projectCfg.Lore.ActiveScrolls = append(projectCfg.Lore.ActiveScrolls, scrollName)
	if err := config.Save(projectCfg, "korva.config.json"); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	printSuccess(fmt.Sprintf("Added scroll '%s' to active scrolls", scrollName))
	return nil
}

// findKorvaLoreDir looks for the lore/curated directory relative to the korva installation.
func findKorvaLoreDir() string {
	// Check a few common locations
	candidates := []string{
		"lore/curated",
		filepath.Join(os.Getenv("KORVA_HOME"), "lore", "curated"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}
