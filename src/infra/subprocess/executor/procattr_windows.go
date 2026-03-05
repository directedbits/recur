//go:build windows

package executorsubprocess

import (
	"os/exec"
	"syscall"
)

// setProcessGroup configures the command to create a new process group on Windows
// using CREATE_NEW_PROCESS_GROUP.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// cancelFunc returns a function that kills the process on Windows.
func cancelFunc(cmd *exec.Cmd) func() error {
	return func() error {
		return cmd.Process.Kill()
	}
}

// killProcessGroup kills the process on Windows.
// Windows doesn't have Unix-style process groups, so we kill the lead process.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}
