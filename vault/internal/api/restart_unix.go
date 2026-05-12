//go:build !windows

package api

import "syscall"

// restartSysProcAttr puts the spawned replacement vault into its own session
// group so it survives the parent process exit. Setsid is POSIX-only — the
// Windows build uses restart_windows.go which returns nil.
func restartSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
