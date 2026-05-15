package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/profile"
	"github.com/alcandev/korva/internal/teamconfig"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync Vault and team profile with remote services",
	RunE:  runSync,
}

var (
	syncProfileFlag    bool
	syncVaultFlag      bool
	syncQuietFlag      bool
	syncTeamConfigFlag bool
)

func init() {
	syncCmd.Flags().BoolVar(&syncProfileFlag, "profile", false, "Pull latest team profile and re-apply")
	syncCmd.Flags().BoolVar(&syncVaultFlag, "vault", false, "Flush the Hive outbox to cloud (default action when no flag is set)")
	syncCmd.Flags().BoolVar(&syncQuietFlag, "quiet", false, "Suppress non-error output (used by post-commit hook)")
	syncCmd.Flags().BoolVar(&syncTeamConfigFlag, "team-config", false, "Download latest team config from Korva Cloud")
}

func runSync(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	if syncTeamConfigFlag {
		return syncTeamConfig(cmd, paths)
	}

	if syncProfileFlag {
		return syncProfile(paths)
	}

	// Default action (and --vault): drain Hive outbox.
	w, vh, err := buildHiveWorker()
	if err != nil {
		// Hive disabled or not provisioned — silent for the post-commit hook,
		// surface for an interactive `korva sync`.
		if syncQuietFlag {
			return nil
		}
		return err
	}
	defer vh.close()

	n, err := w.FlushOnce(context.Background())
	if err != nil {
		if syncQuietFlag {
			return nil
		}
		return fmt.Errorf("hive sync: %w", err)
	}
	if !syncQuietFlag {
		printSuccess(fmt.Sprintf("Hive sync processed %d row(s)", n))
	}
	return nil
}

func syncTeamConfig(cmd *cobra.Command, paths *config.Paths) error {
	key, err := teamconfig.LoadTeamKey(paths.TeamKeyFile)
	if err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("no team key found — run 'korva connect --key KORVA-...' first")
	}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return err
	}

	baseURL := teamConfigBaseURL(cfg.License.ActivationURL)
	c := teamconfig.New(baseURL, key)

	if !syncQuietFlag {
		printInfo("Downloading team config from Korva Cloud...")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	bundle, err := c.DownloadBundle(ctx)
	if errors.Is(err, teamconfig.ErrNotEnabled) {
		if !syncQuietFlag {
			printInfo("Team config not enabled on server")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("team config sync: %w", err)
	}

	if err := paths.EnsureAll(); err != nil {
		return err
	}

	result, err := teamconfig.WriteBundleToDisk(paths.TeamConfigDir(), bundle)
	if err != nil {
		return fmt.Errorf("write team config: %w", err)
	}

	state := teamconfig.SyncState{
		SyncedAt:      time.Now().UTC(),
		BundleVersion: bundle.Version,
		LicenseID:     bundle.LicenseID,
		ItemCount:     len(bundle.Items),
	}
	teamconfig.SaveSyncState(paths.TeamConfigSyncState(), state) //nolint:errcheck

	if !syncQuietFlag {
		printSuccess(fmt.Sprintf("Team config synced — %d written, %d skipped, %d deleted",
			result.Written, result.Skipped, result.Deleted))
	}
	return nil
}

func syncProfile(paths *config.Paths) error {
	lic, _ := license.Load(paths.LicenseFile) // nil on community tier — safe
	mgr := profile.NewManager(paths, lic)

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
