package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/identity"
	"github.com/alcandev/korva/internal/license"
	"github.com/alcandev/korva/internal/version"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health checks on your Korva setup",
	RunE:  runDoctor,
}

var doctorRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Plan, dry-run, or apply repair operations for vault data integrity",
	Long: `Repair operations available:
  - rebuild_fts5            rebuild the observations full-text index
  - purge_orphan_relations  delete observation_relations with deleted endpoints
  - expire_dead_outbox      mark stuck cloud_outbox rows as failed
  - prune_stale_snapshots   delete config_snapshots older than --retention-days

Modes:
  --plan      describe what would run (default; does not read row counts)
  --dry-run   count rows that would be touched but do not write
  --apply     execute the operations and report rows changed

Example:
  korva doctor repair --dry-run
  korva doctor repair --apply --retention-days 30`,
	RunE: runDoctorRepair,
}

func init() {
	doctorCmd.AddCommand(doctorRepairCmd)
	doctorRepairCmd.Flags().Bool("plan", false, "Describe the planned operations without reading row counts")
	doctorRepairCmd.Flags().Bool("dry-run", false, "Count rows that would be touched; do not write")
	doctorRepairCmd.Flags().Bool("apply", false, "Execute the operations")
	doctorRepairCmd.Flags().StringSlice("op", nil, "Restrict to specific operations (repeatable)")
	doctorRepairCmd.Flags().Int("retention-days", 0, "Used by prune_stale_snapshots — 0 disables snapshot prune")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Printf("Korva Doctor — %s\n", version.String())
	fmt.Printf("Platform: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)

	allGood := true

	check := func(name string, ok bool, detail string) {
		if ok {
			fmt.Printf("  ✓ %s\n", name)
		} else {
			fmt.Printf("  ✗ %s: %s\n", name, detail)
			allGood = false
		}
	}

	paths, err := config.PlatformPaths()
	if err != nil {
		fmt.Printf("  ✗ Platform paths: %v\n", err)
		return nil
	}

	// Check korva home dir
	_, err = os.Stat(paths.HomeDir)
	check("Korva home dir exists", err == nil, fmt.Sprintf("run 'korva init' to create %s", paths.HomeDir))

	// Check config
	_, err = os.Stat(paths.ConfigFile)
	check("Config file exists", err == nil, fmt.Sprintf("missing %s", paths.ConfigFile))

	// Check Vault binary
	vaultBin, err := exec.LookPath("korva-vault")
	check("korva-vault in PATH", err == nil, "install korva-vault or add it to your PATH")
	_ = vaultBin

	// Check Vault running
	cfg, _ := config.Load(paths.ConfigFile)
	vaultURL := fmt.Sprintf("http://127.0.0.1:%d/healthz", cfg.Vault.Port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(vaultURL)
	if err == nil {
		resp.Body.Close()
		check("Vault server running", resp.StatusCode == 200, "")
	} else {
		check("Vault server running", false, fmt.Sprintf("start with: korva-vault  (port %d)", cfg.Vault.Port))
	}

	// Check install identity
	installID, idErr := identity.LoadInstallID(paths.InstallID)
	if idErr != nil {
		check("Install ID", false, "run 'korva init' to generate ~/.korva/install.id")
	} else {
		check(fmt.Sprintf("Install ID (%s…)", installID[:8]), true, "")
	}

	_, hkErr := identity.LoadHiveKey(paths.HiveKey)
	check("Hive key", hkErr == nil, "run 'korva init' to generate ~/.korva/hive.key")

	// Check license
	lic, licErr := license.Load(paths.LicenseFile)
	switch {
	case licErr == license.ErrMissing:
		fmt.Printf("  ○ License: community tier (korva license activate <key> to upgrade)\n")
	case licErr != nil:
		check("License", false, licErr.Error())
	default:
		state, _ := license.LoadState(paths.LicenseStateFile)
		tier := lic.CurrentTier(state)
		check(fmt.Sprintf("License: %s (expires %s)", tier, lic.ExpiresAt.Format("2006-01-02")), true, "")
	}

	// Check admin key
	_, err = admin.Load(paths.AdminKey)
	switch {
	case errors.Is(err, admin.ErrNoAdminKey):
		fmt.Printf("  ○ Admin key: not configured (only needed for the admin)\n")
	case err == nil:
		check("Admin key", true, "")
	}

	// Check git
	_, err = exec.LookPath("git")
	check("Git in PATH", err == nil, "required for team profile sync")

	// Check Vault DB
	_, err = os.Stat(paths.VaultDB())
	check("Vault database exists", err == nil, "will be created on first vault_save")

	// Check korva.config.json in current project
	_, err = os.Stat("korva.config.json")
	check("korva.config.json in current dir", err == nil, "run 'korva init' in your project directory")

	// Check Teams session (optional — only shown if session file exists)
	sessionToken, sessErr := readSessionToken(paths.SessionTokenFile)
	if sessErr == nil && sessionToken != "" {
		fmt.Println("")
		fmt.Println("  Teams session:")
		sessClient := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/auth/me", cfg.Vault.Port), nil)
		req.Header.Set("X-Session-Token", sessionToken)
		if r, err := sessClient.Do(req); err == nil {
			defer func() { _ = r.Body.Close() }()
			raw, _ := io.ReadAll(r.Body)
			switch r.StatusCode {
			case http.StatusOK:
				var me struct {
					Email     string `json:"email"`
					Team      string `json:"team"`
					ExpiresAt string `json:"expires_at"`
				}
				json.Unmarshal(raw, &me)
				fmt.Printf("  ✓ Active — %s (%s)\n", me.Email, me.Team)
				if t, err := time.Parse(time.RFC3339, me.ExpiresAt); err == nil {
					remaining := time.Until(t)
					if remaining < 7*24*time.Hour {
						days := int(remaining.Hours() / 24)
						fmt.Printf("  ⚠  Expires in %d day(s) — run 'korva auth redeem' to renew\n", days)
					} else {
						fmt.Printf("  ✓ Valid until %s\n", t.Local().Format("2006-01-02"))
					}
				}
			case http.StatusUnauthorized:
				fmt.Printf("  ✗ Session expired — run 'korva auth redeem <invite-token>'\n")
				allGood = false
			}
		} else {
			fmt.Printf("  ○ Session file found but vault unreachable (skipping check)\n")
		}
	}

	// Vault data integrity (best-effort: only when the vault HTTP API is reachable).
	if integrity, ok := fetchIntegrityReport(cfg.Vault.Port, paths); ok {
		fmt.Println("")
		fmt.Println("  Vault data integrity:")
		for _, c := range integrity.Checks {
			label := c.Name
			switch c.Status {
			case "ok":
				fmt.Printf("    ✓ %s\n", label)
			case "warning":
				fmt.Printf("    ⚠ %s — %s", label, c.Detail)
				if c.Repair != "" {
					fmt.Printf("  (fix: korva doctor repair --apply --op %s)", c.Repair)
				}
				fmt.Println()
				allGood = false
			case "error":
				fmt.Printf("    ✗ %s — %s\n", label, c.Detail)
				allGood = false
			}
		}
	}

	fmt.Println("")
	if allGood {
		fmt.Println("All checks passed.")
	} else {
		fmt.Println("Some checks failed. Review the issues above.")
	}

	return nil
}

// ── integrity & repair ─────────────────────────────────────────────────────

// integrityReport mirrors store.IntegrityReport on the wire. We keep a local
// copy so the CLI does not have to import the vault module.
type integrityReport struct {
	Healthy bool             `json:"healthy"`
	Checks  []integrityCheck `json:"checks"`
}

type integrityCheck struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	Detail        string `json:"detail,omitempty"`
	AffectedCount int    `json:"affected_count"`
	Repair        string `json:"repair,omitempty"`
}

// fetchIntegrityReport calls GET /admin/integrity. Returns false on any error
// so the doctor command degrades gracefully when the vault is offline or the
// admin key is missing — we do not want a network blip to look like a failure.
func fetchIntegrityReport(port int, paths *config.Paths) (*integrityReport, bool) {
	key, err := admin.Load(paths.AdminKey)
	if err != nil {
		return nil, false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/admin/integrity", port), nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("X-Admin-Key", key.Key)
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var out integrityReport
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false
	}
	return &out, true
}

// repairAction mirrors store.RepairAction for the CLI report.
type repairAction struct {
	Operation     string `json:"operation"`
	Description   string `json:"description"`
	EstimatedRows int    `json:"estimated_rows"`
	AppliedRows   int    `json:"applied_rows,omitempty"`
	Error         string `json:"error,omitempty"`
}

type repairReport struct {
	Mode    string         `json:"mode"`
	Actions []repairAction `json:"actions"`
}

// runDoctorRepair POSTs /admin/integrity/repair with the chosen mode and ops.
func runDoctorRepair(cmd *cobra.Command, args []string) error {
	paths, err := config.PlatformPaths()
	if err != nil {
		return err
	}
	cfg, _ := config.Load(paths.ConfigFile)
	key, err := admin.Load(paths.AdminKey)
	if err != nil {
		return fmt.Errorf("admin key required for repair — run `korva init --admin` first")
	}

	mode, err := resolveRepairMode(cmd)
	if err != nil {
		return err
	}
	ops, _ := cmd.Flags().GetStringSlice("op")
	retention, _ := cmd.Flags().GetInt("retention-days")

	body, _ := json.Marshal(map[string]any{
		"mode":                    mode,
		"operations":              ops,
		"snapshot_retention_days": retention,
	})

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("http://127.0.0.1:%d/admin/integrity/repair", cfg.Vault.Port),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", key.Key)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("calling repair endpoint: %w (is the vault running?)", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("repair failed (%d): %s", resp.StatusCode, string(raw))
	}
	var report repairReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("decoding repair response: %w", err)
	}

	fmt.Printf("Repair (mode=%s)\n", report.Mode)
	fmt.Println("─────────────────────────────────")
	for _, a := range report.Actions {
		status := "○"
		if a.Error != "" {
			status = "✗"
		} else if a.AppliedRows > 0 {
			status = "✓"
		} else if a.EstimatedRows > 0 {
			status = "⚠"
		}
		fmt.Printf("  %s %s — %s\n", status, a.Operation, a.Description)
		if a.Error != "" {
			fmt.Printf("    error: %s\n", a.Error)
			continue
		}
		switch report.Mode {
		case "apply":
			fmt.Printf("    applied %d row(s)\n", a.AppliedRows)
		default:
			fmt.Printf("    would touch %d row(s)\n", a.EstimatedRows)
		}
	}
	return nil
}

// resolveRepairMode reads the three mutually-exclusive mode flags and returns
// the canonical mode name. Defaults to "plan" when none is set.
func resolveRepairMode(cmd *cobra.Command) (string, error) {
	plan, _ := cmd.Flags().GetBool("plan")
	dry, _ := cmd.Flags().GetBool("dry-run")
	apply, _ := cmd.Flags().GetBool("apply")

	set := 0
	for _, v := range []bool{plan, dry, apply} {
		if v {
			set++
		}
	}
	if set > 1 {
		return "", fmt.Errorf("only one of --plan, --dry-run, --apply can be set")
	}
	switch {
	case apply:
		return "apply", nil
	case dry:
		return "dry_run", nil
	default:
		return "plan", nil
	}
}
