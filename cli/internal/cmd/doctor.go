package cmd

import (
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

	fmt.Println("")
	if allGood {
		fmt.Println("All checks passed.")
	} else {
		fmt.Println("Some checks failed. Review the issues above.")
	}

	return nil
}
