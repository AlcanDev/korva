package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/license"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of your Korva setup",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	cfg, _ := config.Load(paths.ConfigFile)

	fmt.Println("Korva Status")
	fmt.Println("─────────────────────────────────")

	// Config
	fmt.Printf("  Config:   %s\n", paths.ConfigFile)
	fmt.Printf("  Agent:    %s\n", cfg.Agent)
	fmt.Printf("  Project:  %s\n", cfg.Project)
	fmt.Printf("  Team:     %s\n", cfg.Team)
	fmt.Printf("  Country:  %s\n", cfg.Country)

	// License
	if lic, err := license.Load(paths.LicenseFile); err == nil {
		state, _ := license.LoadState(paths.LicenseStateFile)
		tier := lic.CurrentTier(state)
		exp := ""
		if !lic.ExpiresAt.IsZero() {
			exp = fmt.Sprintf("  · expires %s", lic.ExpiresAt.Format("2006-01-02"))
		}
		fmt.Printf("  License:  ● %s%s\n", tier, exp)
	} else {
		fmt.Printf("  License:  community (free)\n")
	}

	// Admin key
	if _, err := admin.Load(paths.AdminKey); err == nil {
		fmt.Printf("  Admin:    ● configured\n")
	} else {
		fmt.Printf("  Admin:    ○ not configured\n")
	}

	// Vault
	fmt.Println("")
	fmt.Println("  Vault:")
	vaultURL := fmt.Sprintf("http://127.0.0.1:%d/healthz", cfg.Vault.Port)
	if vaultRunning(vaultURL) {
		fmt.Printf("    ● Online  (%s)\n", vaultURL)
		showVaultStats(cfg.Vault.Port)
	} else {
		fmt.Printf("    ○ Offline  (start with: korva-vault)\n")
	}

	// Scrolls
	fmt.Println("")
	fmt.Println("  Active Scrolls:")
	if len(cfg.Lore.ActiveScrolls) == 0 {
		fmt.Println("    (none)")
	}
	for _, s := range cfg.Lore.ActiveScrolls {
		fmt.Printf("    · %s\n", s)
	}

	// Sentinel
	fmt.Println("")
	if cfg.Sentinel.Enabled {
		fmt.Println("  Sentinel: ● enabled")
	} else {
		fmt.Println("  Sentinel: ○ disabled")
	}

	// Private lore
	if info, err := os.Stat(paths.PrivateLoreDir()); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(paths.PrivateLoreDir())
		if len(entries) > 0 {
			fmt.Printf("\n  Private Scrolls: %d installed\n", len(entries))
		}
	}

	return nil
}

func vaultRunning(url string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func showVaultStats(port int) {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/stats", port))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var stats struct {
		TotalObservations int `json:"total_observations"`
		TotalSessions     int `json:"total_sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return
	}

	fmt.Printf("    %d observations  ·  %d sessions\n", stats.TotalObservations, stats.TotalSessions)
}
