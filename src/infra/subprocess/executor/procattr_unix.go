//go:build !windows

package executorsubprocess

import (
	"os/exec"
	"syscall"
)

// setProcessGroup configures the command to run in its own process group
// so the entire tree can be signalled together.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// cancelFunc returns a function that sends SIGTERM to the process group.
func cancelFunc(cmd *exec.Cmd) func() error {
	return func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}

// killProcessGroup sends SIGTERM to the process group (negative PID).
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}
