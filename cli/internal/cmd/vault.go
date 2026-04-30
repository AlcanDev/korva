package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alcandev/korva/internal/config"
	"github.com/alcandev/korva/internal/db"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage the Korva Vault server",
}

var vaultStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the vault server in the background",
	RunE:  runVaultStart,
}

var vaultStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running vault server",
	RunE:  runVaultStop,
}

var vaultStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show vault server status",
	RunE:  runVaultStatus,
}

var vaultLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Print the vault server log file path",
	RunE:  runVaultLogs,
}

var vaultCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove duplicate observations from the vault",
	Long: `Scans the vault for observations with an identical (project, type, content)
triplet and removes all but the oldest copy.

Use --dry-run to preview what would be removed without making any changes.
Use --project to limit deduplication to a specific project.`,
	RunE: runVaultClean,
}

func init() {
	vaultCmd.AddCommand(vaultStartCmd)
	vaultCmd.AddCommand(vaultStopCmd)
	vaultCmd.AddCommand(vaultStatusCmd)
	vaultCmd.AddCommand(vaultLogsCmd)
	vaultCmd.AddCommand(vaultCleanCmd)

	vaultCleanCmd.Flags().Bool("dry-run", false, "Preview duplicates without deleting")
	vaultCleanCmd.Flags().String("project", "", "Limit deduplication to this project")
}

// runVaultStart launches korva-vault in the background, writes its PID to
// ~/.korva/vault/vault.pid, and polls /healthz until the server is ready.
func runVaultStart(cmd *cobra.Command, args []string) error {
	paths := mustPaths()

	// Check if already running.
	if pid, err := readVaultPID(paths); err == nil {
		if isProcessAlive(pid) {
			printInfo(fmt.Sprintf("Vault is already running (PID %d)", pid))
			return nil
		}
		// Stale PID file from a crashed process — clean up.
		os.Remove(paths.VaultPIDFile()) //nolint:errcheck
	}

	// Locate the korva-vault binary.
	bin, err := exec.LookPath("korva-vault")
	if err != nil {
		return fmt.Errorf("korva-vault not found in PATH — install it or add it to PATH")
	}

	// Open the log file (append mode, human-readable).
	logFile, err := os.OpenFile(paths.VaultLogFile(),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open log file %s: %w", paths.VaultLogFile(), err)
	}

	vaultProc := exec.Command(bin, "--mode", "http")
	vaultProc.Stdout = logFile
	vaultProc.Stderr = logFile
	detachProcess(vaultProc) // platform-specific: setsid (Unix) / detached (Windows)

	if err := vaultProc.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start vault: %w", err)
	}
	// Parent closes its copy of the file handle; child keeps writing.
	logFile.Close()

	pid := vaultProc.Process.Pid
	if err := os.WriteFile(paths.VaultPIDFile(),
		[]byte(strconv.Itoa(pid)+"\n"), 0644); err != nil {
		return fmt.Errorf("cannot write PID file: %w", err)
	}

	// Resolve the configured port (default 7437).
	port := vaultPort(paths)

	// Poll /healthz for up to 5 seconds.
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				printSuccess(fmt.Sprintf("Vault started (PID %d) on port %d", pid, port))
				printInfo(fmt.Sprintf("Logs: %s", paths.VaultLogFile()))
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Server didn't respond within 5 s — still report PID so the user can check later.
	printSuccess(fmt.Sprintf("Vault starting (PID %d) on port %d", pid, port))
	printInfo("Server did not respond within 5 s — check 'korva vault status' or logs")
	return nil
}

// runVaultStop sends SIGINT to the vault process and removes the PID file.
func runVaultStop(cmd *cobra.Command, args []string) error {
	paths := mustPaths()

	pid, err := readVaultPID(paths)
	if err != nil {
		// intentional: missing PID file means vault isn't running
		printInfo("Vault is not running (no PID file found)")
		return nil //nolint:nilerr
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(paths.VaultPIDFile())
		// intentional: stale PID file, treat as already-stopped
		printInfo("Vault process not found")
		return nil //nolint:nilerr
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		// Process may have already exited.
		os.Remove(paths.VaultPIDFile()) //nolint:errcheck
		return fmt.Errorf("cannot signal vault process (PID %d): %w", pid, err)
	}

	os.Remove(paths.VaultPIDFile()) //nolint:errcheck
	printSuccess(fmt.Sprintf("Vault stopped (PID %d)", pid))
	return nil
}

// runVaultStatus shows whether the vault process is alive and the HTTP API is up.
func runVaultStatus(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	port := vaultPort(paths)

	pid, pidErr := readVaultPID(paths)
	switch {
	case pidErr != nil:
		fmt.Println("  ○ Process : not running")
	case isProcessAlive(pid):
		fmt.Printf("  ✓ Process : running (PID %d)\n", pid)
	default:
		fmt.Printf("  ✗ Process : PID %d is no longer alive (stale PID file removed)\n", pid)
		os.Remove(paths.VaultPIDFile()) //nolint:errcheck
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("  ✓ HTTP API : up on port %d\n", port)
		} else {
			fmt.Printf("  ✗ HTTP API : unhealthy (status %d)\n", resp.StatusCode)
		}
	} else {
		fmt.Printf("  ✗ HTTP API : unreachable on port %d\n", port)
	}
	return nil
}

// runVaultLogs prints the log file path so users can tail it easily.
func runVaultLogs(cmd *cobra.Command, args []string) error {
	paths := mustPaths()
	fmt.Println(paths.VaultLogFile())
	return nil
}

// runVaultClean deduplicates the vault database in-process (no server required).
// It opens the SQLite file directly, so the vault server does not need to be running.
// The FTS5 sync triggers in the schema automatically update the search index on delete.
func runVaultClean(cmd *cobra.Command, _ []string) error {
	paths := mustPaths()

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	project, _ := cmd.Flags().GetString("project")

	dbPath := paths.VaultDB()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("vault database not found at %s — run 'korva vault start' first", dbPath)
	}

	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening vault database: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	// Build queries — scoped to a project when provided, global otherwise.
	var totalQ, dupQ, deleteQ string
	var qArgs, totalArgs []any

	if project != "" {
		totalQ = `SELECT COUNT(*) FROM observations WHERE project = ?`
		totalArgs = []any{project}
		dupQ = `SELECT COUNT(*) FROM observations
			WHERE project = ?
			AND id NOT IN (
				SELECT MIN(id) FROM observations
				WHERE project = ?
				GROUP BY project, type, lower(trim(content))
			)`
		deleteQ = `DELETE FROM observations
			WHERE project = ?
			AND id NOT IN (
				SELECT MIN(id) FROM observations
				WHERE project = ?
				GROUP BY project, type, lower(trim(content))
			)`
		qArgs = []any{project, project}
	} else {
		totalQ = `SELECT COUNT(*) FROM observations`
		dupQ = `SELECT COUNT(*) FROM observations
			WHERE id NOT IN (
				SELECT MIN(id) FROM observations
				GROUP BY project, type, lower(trim(content))
			)`
		deleteQ = `DELETE FROM observations
			WHERE id NOT IN (
				SELECT MIN(id) FROM observations
				GROUP BY project, type, lower(trim(content))
			)`
	}

	var total, duplicates int
	sqlDB.QueryRow(totalQ, totalArgs...).Scan(&total) //nolint:errcheck
	sqlDB.QueryRow(dupQ, qArgs...).Scan(&duplicates)  //nolint:errcheck

	if project != "" {
		fmt.Printf("  Project   : %s\n", project)
	} else {
		fmt.Println("  Project   : (all)")
	}
	fmt.Printf("  Total     : %d observations\n", total)
	fmt.Printf("  Duplicates: %d\n", duplicates)

	if dryRun {
		if duplicates == 0 {
			printSuccess("No duplicates found — vault is clean")
		} else {
			printInfo(fmt.Sprintf("Dry run — %d duplicate(s) would be removed", duplicates))
			printInfo("Run without --dry-run to apply")
		}
		return nil
	}
	if duplicates == 0 {
		printSuccess("No duplicates found — vault is clean")
		return nil
	}

	res, execErr := sqlDB.Exec(deleteQ, qArgs...)
	if execErr != nil {
		return fmt.Errorf("dedup: %w", execErr)
	}
	n, _ := res.RowsAffected()
	printSuccess(fmt.Sprintf("Removed %d duplicate observation(s)", n))
	return nil
}

// --- helpers ---

// readVaultPID reads the PID from ~/.korva/vault/vault.pid.
func readVaultPID(paths *config.Paths) (int, error) {
	data, err := os.ReadFile(paths.VaultPIDFile())
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("malformed PID file: %w", err)
	}
	return pid, nil
}

// vaultPort returns the configured vault HTTP port, defaulting to 7437.
func vaultPort(paths *config.Paths) int {
	cfg, err := config.Load(paths.ConfigFile)
	if err == nil && cfg.Vault.Port > 0 {
		return cfg.Vault.Port
	}
	return 7437
}
