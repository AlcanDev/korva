package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/identity"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/teamconfig"
)

var connectCmd = &cobra.Command{
	Use:   "connect --key <license-key>",
	Short: "Connect to Korva Cloud using your Teams license key",
	Long: `Connect links this installation to Korva Cloud using your Teams license key.

It will:
  1. Activate the license locally (secures a seat)
  2. Save the key for team config sync (~/.korva/team.key)
  3. Download your team's config (scrolls, rules, instructions, skills)
  4. Write config to ~/.korva/team-config/

After connecting, run 'korva sync --team-config' to update at any time.`,
	RunE: runConnect,
}

var connectKeyFlag string

func init() {
	connectCmd.Flags().StringVar(&connectKeyFlag, "key", "", "Korva Teams license key (KORVA-XXXX-XXXX-XXXX-XXXX)")
	_ = connectCmd.MarkFlagRequired("key")
}

func runConnect(cmd *cobra.Command, _ []string) error {
	key := strings.ToUpper(strings.TrimSpace(connectKeyFlag))
	if key == "" {
		return fmt.Errorf("--key is required")
	}

	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}
	if err := paths.EnsureAll(); err != nil {
		return fmt.Errorf("prepare directories: %w", err)
	}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return err
	}

	installID, err := identity.LoadInstallID(paths.InstallID)
	if err != nil {
		return fmt.Errorf("install ID not found — run 'korva init' first: %w", err)
	}

	// Step 1: Activate the license (get JWS, store in ~/.korva/license.key).
	printInfo("Activating license...")
	ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
	defer cancel()

	lic, err := license.Activate(ctx, cfg.License.ActivationURL, key, installID,
		paths.LicenseFile, paths.LicenseStateFile)
	if err != nil {
		return fmt.Errorf("license activation failed: %w", err)
	}
	printSuccess(fmt.Sprintf("License activated — tier: %s, seats: %d", lic.Tier, lic.Seats))

	// Step 2: Save raw key for team config sync.
	if err := teamconfig.SaveTeamKey(paths.TeamKeyFile, key); err != nil {
		return fmt.Errorf("save team key: %w", err)
	}

	// Step 3: Download team config bundle.
	printInfo("Downloading team config from Korva Cloud...")
	teamConfigURL := teamConfigBaseURL(cfg.License.ActivationURL)
	c := teamconfig.New(teamConfigURL, key)

	ctx2, cancel2 := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel2()

	bundle, err := c.DownloadBundle(ctx2)
	if errors.Is(err, teamconfig.ErrNotEnabled) {
		// Server doesn't have team config enabled yet — not a fatal error.
		printInfo("Team config not yet enabled on server — skipping config download")
		printSuccess("Connected (license only)")
		return nil
	}
	if err != nil {
		printInfo(fmt.Sprintf("Team config download failed: %v", err))
		printInfo("You can retry with: korva sync --team-config")
		printSuccess("Connected (license activated, config pending)")
		return nil
	}

	// Step 4: Write config to disk.
	result, err := teamconfig.WriteBundleToDisk(paths.TeamConfigDir(), bundle)
	if err != nil {
		return fmt.Errorf("write team config: %w", err)
	}

	// Save sync state.
	state := teamconfig.SyncState{
		SyncedAt:      time.Now().UTC(),
		BundleVersion: bundle.Version,
		LicenseID:     bundle.LicenseID,
		ItemCount:     len(bundle.Items),
	}
	teamconfig.SaveSyncState(paths.TeamConfigSyncState(), state) //nolint:errcheck

	printSuccess(fmt.Sprintf("Team config synced — %d items downloaded, %d skipped",
		result.Written, result.Skipped))
	fmt.Printf("\n")
	fmt.Printf("  License ID : %s\n", bundle.LicenseID)
	fmt.Printf("  Tier       : %s\n", bundle.Tier)
	fmt.Printf("  Config dir : %s\n", paths.TeamConfigDir())
	if !lic.ExpiresAt.IsZero() {
		fmt.Printf("  Expires    : %s\n", lic.ExpiresAt.Format("2006-01-02"))
	}
	fmt.Printf("\n")
	printInfo("Run 'korva sync --team-config' to update at any time")

	sendUsageEvent("team_connected", map[string]any{"tier": lic.Tier})
	return nil
}

// teamConfigBaseURL derives the base URL from the activation URL.
// e.g. "https://licensing.korva.dev/v1/activate" → "https://licensing.korva.dev"
func teamConfigBaseURL(activationURL string) string {
	if i := strings.Index(activationURL, "/v1/"); i != -1 {
		return activationURL[:i]
	}
	return activationURL
}
