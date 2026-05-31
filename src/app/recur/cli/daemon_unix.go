//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

// daemonSysProcAttr returns process attributes for detaching the daemon.
func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// detachProcess releases the daemon process so it runs independently.
func detachProcess(proc *exec.Cmd) {
	_ = proc.Process.Release()
}
