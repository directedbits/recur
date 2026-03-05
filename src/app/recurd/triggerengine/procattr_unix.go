//go:build !windows

package triggerengine

import (
	"os/exec"
	"syscall"
)

// setPluginProcessGroup configures the plugin command to run in its own
// process group so the entire tree can be signalled on stop.
func setPluginProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killPluginProcess sends SIGTERM to the plugin's process group.
func killPluginProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}

// forceKillPluginProcess sends SIGKILL to the plugin's process group.
func forceKillPluginProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
