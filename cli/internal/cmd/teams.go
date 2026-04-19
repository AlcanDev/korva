package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/profile"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "Korva for Teams — manage team profile and members",
}

var teamsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync team profile: check Beacon first, fall back to Git",
	RunE:  runTeamsSync,
}

var teamsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active team profile source (beacon or git)",
	RunE:  runTeamsStatus,
}

var teamsSyncGitMirror string

func init() {
	teamsCmd.AddCommand(teamsSyncCmd)
	teamsCmd.AddCommand(teamsStatusCmd)
	teamsSyncCmd.Flags().StringVar(&teamsSyncGitMirror, "git-mirror", "",
		"If set, export the active profile to this Git repo URL after sync")
}

func runTeamsSync(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	lic, _ := license.Load(paths.LicenseFile) // nil = community tier
	mgr := profile.NewManager(paths, lic)

	// Read admin key for Beacon API auth
	adminKey := readAdminKey(paths.AdminKey)

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	// Step 1: try Beacon API
	beacon, err := mgr.FetchBeaconProfile(ctx, adminKey)
	if err != nil {
		printInfo(fmt.Sprintf("Beacon unreachable (%v), falling back to Git", err))
	}

	if beacon != nil {
		printSuccess(fmt.Sprintf("Beacon profile active — team: %v", beacon.Team["name"]))
		if teamsSyncGitMirror != "" {
			printInfo("Git mirror export not yet implemented in this version")
		}
		return nil
	}

	// Step 2: fall back to Git profile sync
	profileID, err := mgr.ActiveProfileID()
	if err != nil || profileID == "" {
		return fmt.Errorf("no active team profile — run 'korva init --profile <url>' or configure via Beacon panel")
	}

	printInfo(fmt.Sprintf("Syncing Git profile '%s'…", profileID))
	baseCfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if _, err := mgr.Sync(profileID, baseCfg); err != nil {
		return fmt.Errorf("git sync failed: %w", err)
	}
	printSuccess(fmt.Sprintf("Team profile '%s' synced from Git", profileID))
	return nil
}

func runTeamsStatus(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	lic, _ := license.Load(paths.LicenseFile)
	mgr := profile.NewManager(paths, lic)
	adminKey := readAdminKey(paths.AdminKey)

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	beacon, _ := mgr.FetchBeaconProfile(ctx, adminKey)
	if beacon != nil {
		fmt.Printf("  Source  : beacon (local vault)\n")
		if name, ok := beacon.Team["name"].(string); ok {
			fmt.Printf("  Team    : %s\n", name)
		}
		fmt.Printf("  Members : %d\n", len(beacon.Members))
		return nil
	}

	profileID, _ := mgr.ActiveProfileID()
	if profileID == "" {
		printInfo("No team profile configured (Community tier)")
		return nil
	}
	fmt.Printf("  Source     : git\n")
	fmt.Printf("  Profile ID : %s\n", profileID)
	return nil
}

// readAdminKey reads the admin key value from the key file.
func readAdminKey(keyPath string) string {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return ""
	}
	var kf admin.AdminConfig
	if err := json.Unmarshal(data, &kf); err != nil {
		return ""
	}
	return kf.Key
}
