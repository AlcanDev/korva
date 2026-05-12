package api

import (
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"
)

// restartDelay gives the HTTP layer a brief window to flush the response
// before the process restarts.
const restartDelay = 150 * time.Millisecond

// adminRestartVault handles POST /admin/vault/restart — replaces the running
// vault process with a fresh one. The response is sent before the actual
// restart so the client can poll /healthz to know when the new server is up.
//
// Implementation: spawn a copy of os.Args[0] with the same arguments and
// environment using os.StartProcess, then exit cleanly. POSIX could use
// syscall.Exec for a true exec(3); spawn-and-exit is simpler and works on
// Windows too.
func adminRestartVault() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Best-effort: if we can't even resolve the executable, fail loudly so
		// the client doesn't think the restart succeeded.
		exe, err := os.Executable()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not resolve executable: "+err.Error())
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":     "restarting",
			"old_pid":    os.Getpid(),
			"executable": exe,
		})

		go performRestart(exe, os.Args[1:], os.Environ())
	}
}

// performRestart spawns a replacement process and exits.
func performRestart(exe string, args []string, env []string) {
	time.Sleep(restartDelay)

	attr := &os.ProcAttr{
		Env:   env,
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys:   restartSysProcAttr(),
	}

	// os.StartProcess wants argv[0] as the first element.
	argv := append([]string{exe}, args...)
	if _, err := os.StartProcess(exe, argv, attr); err != nil {
		// Don't exit if we can't spawn the replacement — log to stderr and
		// keep the current server alive so the operator can investigate.
		_, _ = os.Stderr.WriteString("vault restart failed: " + err.Error() + "\n")
		return
	}

	// Hand off cleanly. The parent has already responded.
	os.Exit(0)
}

// restartSysProcAttr returns OS-specific process attributes for the spawned
// replacement. On POSIX we put the new process in its own session group so it
// survives the parent exit; on Windows we use the default attributes.
func restartSysProcAttr() *syscall.SysProcAttr {
	if runtime.GOOS == "windows" {
		return nil
	}
	return &syscall.SysProcAttr{Setsid: true}
}
