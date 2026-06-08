//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

// daemonSysProcAttr returns process attributes for running the daemon
// as a detached process on Windows.
func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// detachProcess releases the daemon process so it runs independently.
func detachProcess(proc *exec.Cmd) {
	proc.Process.Release()
}
