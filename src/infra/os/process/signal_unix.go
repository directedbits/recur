//go:build !windows

package processos

import (
	"os"
	"path/filepath"
	"syscall"
)

// SendTermSignal sends SIGTERM to the given process.
func SendTermSignal(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

// IsProcessAlive checks if a process is running using signal 0.
func IsProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// defaultSocketPath returns the Unix socket path.
func defaultSocketPath() (string, error) {
	dir, err := RunDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "recur.sock"), nil
}
