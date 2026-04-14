package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/admin"
	"github.com/alcandev/korva/internal/config"
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

	// Check admin key
	_, err = admin.Load(paths.AdminKey)
	if err == admin.ErrNoAdminKey {
		fmt.Printf("  ○ Admin key: not configured (only needed for the admin)\n")
	} else if err == nil {
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

	fmt.Println("")
	if allGood {
		fmt.Println("All checks passed.")
	} else {
		fmt.Println("Some checks failed. Review the issues above.")
	}

	return nil
}
