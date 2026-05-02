package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/identity"
	"github.com/alcandev/korva/internal/license"
)

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Manage your Korva for Teams license",
}

var licenseActivateCmd = &cobra.Command{
	Use:   "activate <license-key>",
	Short: "Activate a Korva for Teams license key",
	Args:  cobra.ExactArgs(1),
	RunE:  runLicenseActivate,
}

var licenseStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current license tier and grace window",
	RunE:  runLicenseStatus,
}

var licenseDeactivateCmd = &cobra.Command{
	Use:   "deactivate",
	Short: "Remove the local license (reverts to Community tier)",
	RunE:  runLicenseDeactivate,
}

func init() {
	licenseCmd.AddCommand(licenseActivateCmd)
	licenseCmd.AddCommand(licenseStatusCmd)
	licenseCmd.AddCommand(licenseDeactivateCmd)
}

func runLicenseActivate(cmd *cobra.Command, args []string) error {
	key := args[0]

	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	installID, err := identity.LoadInstallID(paths.InstallID)
	if err != nil {
		return fmt.Errorf("install ID not found — run `korva init` first: %w", err)
	}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
	defer cancel()

	lic, err := license.Activate(ctx, cfg.License.ActivationURL, key, installID, paths.LicenseFile, paths.LicenseStateFile)
	if err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("License activated — tier: %s, seats: %d", lic.Tier, lic.Seats))
	if !lic.ExpiresAt.IsZero() {
		printInfo(fmt.Sprintf("Expires: %s", lic.ExpiresAt.Format("2006-01-02")))
	}
	sendUsageEvent("license_activated", map[string]any{"tier": lic.Tier})
	return nil
}

func runLicenseStatus(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}

	lic, err := license.Load(paths.LicenseFile)
	if err == license.ErrMissing {
		printInfo("No license installed — tier: community")
		return nil
	}
	if err != nil {
		return err
	}

	state, err := license.LoadState(paths.LicenseStateFile)
	if err != nil {
		return err
	}

	tier := lic.CurrentTier(state)
	fmt.Printf("  License ID : %s\n", lic.LicenseID)
	fmt.Printf("  Tier       : %s\n", tier)
	fmt.Printf("  Seats      : %d\n", lic.Seats)
	if !lic.ExpiresAt.IsZero() {
		fmt.Printf("  Expires    : %s\n", lic.ExpiresAt.Format("2006-01-02"))
	}
	if !state.LastHeartbeat.IsZero() {
		fmt.Printf("  Last check : %s\n", state.LastHeartbeat.Format(time.RFC3339))
	}
	rem := lic.GraceRemaining(state)
	if rem > 0 {
		fmt.Printf("  Grace left : %s\n", rem.Truncate(time.Hour))
	} else if lic.GraceDays > 0 {
		printInfo("Grace period lapsed — running as Community tier")
	}
	if len(lic.Features) > 0 {
		fmt.Printf("  Features   : %v\n", lic.Features)
	}
	return nil
}

func runLicenseDeactivate(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}
	if err := license.Deactivate(paths.LicenseFile, paths.LicenseStateFile); err != nil {
		return err
	}
	printSuccess("License removed — now running as Community tier")
	return nil
}
