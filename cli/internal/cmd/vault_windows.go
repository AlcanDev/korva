//go:build windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// detachProcess on Windows starts the child in a new process group so it
// continues to run after the parent console closes.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// CREATE_NEW_PROCESS_GROUP: child gets its own process group.
		// DETACHED_PROCESS (0x00000008): child does not inherit console.
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
	}
	cmd.Stdin = nil
}

// isProcessAlive on Windows uses FindProcess to check for the process.
// Note: FindProcess on Windows returns a handle even if the process has
// exited; for MVP we rely on the HTTP healthz check as the authoritative
// liveness indicator rather than trying to open the process handle.
func isProcessAlive(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil && pid > 0
}
