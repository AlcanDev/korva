//go:build windows

package api

import "syscall"

// restartSysProcAttr returns nil on Windows. The default StartProcess attributes
// already create the new vault as a detached process from the operator's
// perspective; the POSIX-specific Setsid does not apply here.
func restartSysProcAttr() *syscall.SysProcAttr {
	return nil
}
