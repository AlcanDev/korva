package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
)

var beaconCmd = &cobra.Command{
	Use:   "beacon",
	Short: "Manage the Beacon UI dev server (local web dashboard)",
	Long: `Beacon is the Korva local web UI (React + Vite). It is served separately
from the Vault MCP server and runs on port 5173 by default.

Use these commands when you want the Korva CLI to spawn the Beacon dev
server in the background, the same way it manages the Vault process.`,
}

var beaconStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Beacon dev server in the background",
	RunE:  runBeaconStart,
}

var beaconStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running Beacon dev server",
	RunE:  runBeaconStop,
}

var beaconStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Beacon dev server status",
	RunE:  runBeaconStatus,
}

var beaconLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Print the Beacon dev server log file path",
	RunE:  runBeaconLogs,
}

func init() {
	beaconCmd.AddCommand(beaconStartCmd)
	beaconCmd.AddCommand(beaconStopCmd)
	beaconCmd.AddCommand(beaconStatusCmd)
	beaconCmd.AddCommand(beaconLogsCmd)
}

// runBeaconStart spawns `npm run dev` inside the Beacon source directory,
// writing the PID to ~/.korva/vault/beacon.pid and polling the dev server
// until it accepts HTTP connections.
func runBeaconStart(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	cfg, _ := config.Load(paths.ConfigFile)

	// Bail out if Beacon is already running.
	if pid, err := readBeaconPID(paths); err == nil {
		if isProcessAlive(pid) {
			printInfo(fmt.Sprintf("Beacon is already running (PID %d)", pid))
			return nil
		}
		// Stale PID file — clean it up before retrying.
		_ = os.Remove(paths.BeaconPIDFile())
	}

	devDir, err := resolveBeaconDevDir(cfg)
	if err != nil {
		return err
	}

	npmBin, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("`npm` not found in PATH — install Node.js 22+ and retry")
	}

	if err := os.MkdirAll(filepath.Dir(paths.BeaconLogFile()), 0o755); err != nil {
		return fmt.Errorf("creating log dir: %w", err)
	}
	logFile, err := os.OpenFile(paths.BeaconLogFile(),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open log file %s: %w", paths.BeaconLogFile(), err)
	}

	port := beaconPort(cfg)
	beaconProc := exec.Command(npmBin, "run", "dev", "--", "--port", strconv.Itoa(port))
	beaconProc.Dir = devDir
	beaconProc.Stdout = logFile
	beaconProc.Stderr = logFile
	beaconProc.Env = append(os.Environ(), "FORCE_COLOR=0")
	detachProcess(beaconProc)

	if err := beaconProc.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("failed to start beacon: %w", err)
	}
	_ = logFile.Close()

	pid := beaconProc.Process.Pid
	if err := os.WriteFile(paths.BeaconPIDFile(),
		[]byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return fmt.Errorf("cannot write PID file: %w", err)
	}

	// Poll until the Vite dev server responds (typical startup ~1.5 s).
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				printSuccess(fmt.Sprintf("Beacon started (PID %d) on http://localhost:%d", pid, port))
				printInfo(fmt.Sprintf("Logs: %s", paths.BeaconLogFile()))
				printInfo(fmt.Sprintf("Source: %s", devDir))
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Server not yet up — leave the PID file so `beacon status` works once it is.
	printSuccess(fmt.Sprintf("Beacon starting (PID %d) on port %d", pid, port))
	printInfo("Dev server did not respond within 15 s — check 'korva beacon status' or logs")
	return nil
}

// runBeaconStop signals the Beacon process to terminate and removes the PID file.
func runBeaconStop(cmd *cobra.Command, args []string) error {
	paths := mustPaths()

	pid, err := readBeaconPID(paths)
	if err != nil {
		printInfo("Beacon is not running (no PID file found)")
		return nil //nolint:nilerr
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(paths.BeaconPIDFile())
		printInfo("Beacon process not found")
		return nil //nolint:nilerr
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		_ = os.Remove(paths.BeaconPIDFile())
		return fmt.Errorf("cannot signal beacon process (PID %d): %w", pid, err)
	}

	_ = os.Remove(paths.BeaconPIDFile())
	printSuccess(fmt.Sprintf("Beacon stopped (PID %d)", pid))
	return nil
}

// runBeaconStatus prints the same shape as `vault status` so both feel symmetric.
func runBeaconStatus(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	cfg, _ := config.Load(paths.ConfigFile)
	port := beaconPort(cfg)

	pid, pidErr := readBeaconPID(paths)
	switch {
	case pidErr != nil:
		fmt.Println("  ○ Process : not running")
	case isProcessAlive(pid):
		fmt.Printf("  ✓ Process : running (PID %d)\n", pid)
	default:
		fmt.Printf("  ✗ Process : PID %d is no longer alive (stale PID file removed)\n", pid)
		_ = os.Remove(paths.BeaconPIDFile())
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("  ✓ Dev server : up on http://localhost:%d\n", port)
		} else {
			fmt.Printf("  ✗ Dev server : unhealthy (status %d)\n", resp.StatusCode)
		}
	} else {
		fmt.Printf("  ✗ Dev server : unreachable on port %d\n", port)
	}
	return nil
}

// runBeaconLogs prints the log file path for tailing convenience.
func runBeaconLogs(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	fmt.Println(paths.BeaconLogFile())
	return nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

// readBeaconPID reads ~/.korva/vault/beacon.pid.
func readBeaconPID(paths *config.Paths) (int, error) {
	data, err := os.ReadFile(paths.BeaconPIDFile())
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("malformed PID file: %w", err)
	}
	return pid, nil
}

// beaconPort returns the configured Beacon port (default 5173). Mirrors
// `vaultPort` so both daemons follow the same resolution rules.
func beaconPort(cfg config.KorvaConfig) int {
	if cfg.Beacon.Port > 0 {
		return cfg.Beacon.Port
	}
	return 5173
}

// resolveBeaconDevDir picks the Beacon source directory in order:
//
//  1. cfg.Beacon.DevDir (explicit config)
//  2. $KORVA_BEACON_DIR env var
//  3. CWD/beacon, CWD/../beacon, … up to 3 ancestor levels
//
// Returns a friendly error when no candidate has a package.json named
// "korva-beacon", so a user-friendly hint guides them to clone the repo.
var errBeaconDirNotFound = errors.New(
	"beacon source not found: pass --dev-dir, set $KORVA_BEACON_DIR, or set beacon.dev_dir in korva.config.json " +
		"(if you installed Korva from a binary release you need a clone of the repo for the Beacon UI)",
)

func resolveBeaconDevDir(cfg config.KorvaConfig) (string, error) {
	candidates := []string{}

	if cfg.Beacon.DevDir != "" {
		candidates = append(candidates, cfg.Beacon.DevDir)
	}
	if env := os.Getenv("KORVA_BEACON_DIR"); env != "" {
		candidates = append(candidates, env)
	}
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 4; i++ {
			candidates = append(candidates, filepath.Join(dir, "beacon"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	for _, c := range candidates {
		if isBeaconDir(c) {
			abs, err := filepath.Abs(c)
			if err != nil {
				return c, nil //nolint:nilerr // best-effort absolutification
			}
			return abs, nil
		}
	}

	return "", errBeaconDirNotFound
}

// isBeaconDir reports whether `path` looks like the korva-beacon source tree.
// We require a package.json with the canonical name to avoid false positives
// for any random "beacon/" directory the user might have lying around.
func isBeaconDir(path string) bool {
	data, err := os.ReadFile(filepath.Join(path, "package.json"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), `"name": "korva-beacon"`)
}
