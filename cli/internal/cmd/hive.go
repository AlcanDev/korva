package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/db"
	"github.com/alcandev/korva/internal/hive"
	"github.com/alcandev/korva/internal/identity"
	"github.com/alcandev/korva/internal/privacy/cloud"
)

var hiveCmd = &cobra.Command{
	Use:   "hive",
	Short: "Korva Hive — community brain cloud sync",
	Long: `Korva Hive ships anonymized observations to the community cloud
so every team's local knowledge contributes to (and benefits from) a
shared brain. Sync is automatic. Use these commands to inspect, flush,
or temporarily disable it.`,
}

var hivePushCmd = &cobra.Command{
	Use:   "push",
	Short: "Flush the local outbox to Hive once",
	RunE:  runHivePush,
}

var hiveStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show outbox counts (pending / sent / rejected / failed)",
	RunE:  runHiveStatus,
}

var hiveEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Re-enable Hive sync (writes config)",
	RunE:  func(c *cobra.Command, _ []string) error { return setHiveEnabled(true) },
}

var hiveDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable Hive sync (writes config). Pending rows stay queued.",
	RunE:  func(c *cobra.Command, _ []string) error { return setHiveEnabled(false) },
}

var hiveRetryCmd = &cobra.Command{
	Use:   "retry",
	Short: "Re-enqueue all rows parked at status='failed'",
	RunE:  runHiveRetry,
}

var hiveRotateKeyCmd = &cobra.Command{
	Use:   "rotate-key",
	Short: "Generate a new Hive API key (you must coordinate revocation server-side)",
	RunE:  runHiveRotateKey,
}

func init() {
	hiveCmd.AddCommand(hivePushCmd, hiveStatusCmd, hiveEnableCmd, hiveDisableCmd, hiveRetryCmd, hiveRotateKeyCmd)
}

func runHivePush(cmd *cobra.Command, args []string) error {
	w, paths, err := buildHiveWorker()
	if err != nil {
		return err
	}
	defer paths.close()

	n, err := w.FlushOnce(context.Background())
	if err != nil {
		return fmt.Errorf("hive push: %w", err)
	}
	printSuccess(fmt.Sprintf("Processed %d outbox row(s)", n))
	return nil
}

func runHiveStatus(cmd *cobra.Command, args []string) error {
	hb, paths, err := openOutbox()
	if err != nil {
		return err
	}
	defer paths.close()

	c, err := hb.Status()
	if err != nil {
		return err
	}
	fmt.Printf("Hive outbox:\n")
	fmt.Printf("  pending : %d\n", c.Pending)
	fmt.Printf("  sent    : %d\n", c.Sent)
	fmt.Printf("  rejected: %d (privacy filter)\n", c.Rejected)
	fmt.Printf("  failed  : %d (run 'korva hive retry' to retry)\n", c.Failed)
	return nil
}

func runHiveRetry(cmd *cobra.Command, args []string) error {
	hb, paths, err := openOutbox()
	if err != nil {
		return err
	}
	defer paths.close()

	n, err := hb.Retry()
	if err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Re-enqueued %d failed row(s)", n))
	return nil
}

func runHiveRotateKey(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}
	if _, err := identity.RotateHiveKey(paths.HiveKey); err != nil {
		return fmt.Errorf("rotating hive key: %w", err)
	}
	printSuccess("Hive key rotated.")
	fmt.Println("\nThe old key is invalidated locally.")
	fmt.Println("If you have already pushed to Hive, contact support to revoke the previous key.")
	return nil
}

func setHiveEnabled(enabled bool) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return err
	}
	cfg.Hive.Enabled = enabled
	if err := config.Save(cfg, paths.ConfigFile); err != nil {
		return err
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	printSuccess(fmt.Sprintf("Hive sync %s", state))
	return nil
}

// --- shared bootstrap ---

type vaultHandle struct {
	closer func()
}

func (v vaultHandle) close() {
	if v.closer != nil {
		v.closer()
	}
}

// openOutbox opens the local Vault DB and returns a wired Outbox.
// Caller must invoke .close().
func openOutbox() (*hive.Outbox, vaultHandle, error) {
	paths, err := config.PlatformPaths()
	if err != nil {
		return nil, vaultHandle{}, err
	}
	d, err := db.Open(paths.VaultDB())
	if err != nil {
		return nil, vaultHandle{}, fmt.Errorf("opening vault: %w", err)
	}
	if err := db.Migrate(d); err != nil {
		_ = d.Close()
		return nil, vaultHandle{}, fmt.Errorf("migrating vault: %w", err)
	}
	return hive.NewOutbox(d), vaultHandle{closer: func() { _ = d.Close() }}, nil
}

// buildHiveWorker wires Outbox + Client + Filter ready for a one-shot flush.
func buildHiveWorker() (*hive.Worker, vaultHandle, error) {
	paths, err := config.PlatformPaths()
	if err != nil {
		return nil, vaultHandle{}, err
	}
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return nil, vaultHandle{}, fmt.Errorf("loading config: %w", err)
	}
	if !cfg.Hive.Enabled {
		return nil, vaultHandle{}, fmt.Errorf("hive sync is disabled (run 'korva hive enable' to turn it back on)")
	}
	hiveKey, err := identity.LoadHiveKey(paths.HiveKey)
	if err != nil {
		return nil, vaultHandle{}, fmt.Errorf("loading hive key (run 'korva init' first): %w", err)
	}
	installID, err := identity.LoadInstallID(paths.InstallID)
	if err != nil {
		return nil, vaultHandle{}, fmt.Errorf("loading install id: %w", err)
	}

	outbox, vh, err := openOutbox()
	if err != nil {
		return nil, vaultHandle{}, err
	}

	client := hive.NewClient(cfg.Hive.Endpoint, hiveKey)
	filter := cloud.New(cfg.Hive.AllowedTypes, installID)
	worker := hive.NewWorker(outbox, client, filter, installID, time.Duration(cfg.Hive.IntervalMin)*time.Minute)
	return worker, vh, nil
}
