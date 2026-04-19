//go:build !windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// detachProcess configures cmd to start a new session (setsid) so the child
// process survives the parent shell exiting.  Stdin is set to /dev/null so
// the process never blocks waiting for terminal input.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // create new process group / session
	}
	cmd.Stdin = nil
}

// isProcessAlive sends signal 0 to the process — a POSIX-standard existence
// check that does not actually deliver a signal but returns an error if the
// PID does not exist or belongs to a different user.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
