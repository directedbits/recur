//go:build windows

package triggerengine

import (
	"os/exec"
	"syscall"
)

// setPluginProcessGroup configures the plugin command on Windows using
// CREATE_NEW_PROCESS_GROUP.
func setPluginProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killPluginProcess terminates the plugin process on Windows.
func killPluginProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// forceKillPluginProcess force-kills the plugin process on Windows.
// On Windows, Kill() is already a hard kill, so this is the same.
func forceKillPluginProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}
