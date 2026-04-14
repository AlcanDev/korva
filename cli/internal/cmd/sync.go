package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/profile"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync Vault and team profile with remote repositories",
	RunE:  runSync,
}

var syncProfileFlag bool

func init() {
	syncCmd.Flags().BoolVar(&syncProfileFlag, "profile", false, "Pull latest team profile and re-apply")
}

func runSync(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	if syncProfileFlag {
		return syncProfile(paths)
	}

	fmt.Println("Vault Git Sync not implemented yet. Use --profile to sync team config.")
	return nil
}

func syncProfile(paths *config.Paths) error {
	mgr := profile.NewManager(paths)

	profileID, err := mgr.ActiveProfileID()
	if err != nil || profileID == "" {
		return fmt.Errorf("no active team profile found — run 'korva init --profile <url>' first")
	}

	printInfo(fmt.Sprintf("Syncing team profile '%s' ...", profileID))

	baseCfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if _, err := mgr.Sync(profileID, baseCfg); err != nil {
		return fmt.Errorf("syncing team profile: %w", err)
	}

	printSuccess("Team profile synced")
	return nil
}
