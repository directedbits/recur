// Package process handles daemon process management — PID files, subprocess spawning, signal handling.
package processos

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
)

// RunDir returns the recur runtime directory (~/.config/recur/run/), creating it if needed.
func RunDir() (string, error) {
	configDir, err := configyaml.ConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "run")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("could not create run directory: %w", err)
	}
	return dir, nil
}

// PIDPath returns the default PID file path (~/.config/recur/run/recur.pid).
func PIDPath() (string, error) {
	dir, err := RunDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "recur.pid"), nil
}

// DefaultSocketPath returns the default daemon address.
// On Linux: Unix socket path (~/.config/recur/run/recur.sock).
// On Windows: TCP address (localhost:19384).
func DefaultSocketPath() (string, error) {
	return defaultSocketPath()
}

// WritePID writes a PID to the given file path using atomic write (temp file + rename).
func WritePID(path string, pid int) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create PID directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "recur-pid-*.tmp")
	if err != nil {
		return fmt.Errorf("could not create temp PID file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := fmt.Fprintf(tmp, "%d\n", pid); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("could not write PID: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("could not close temp PID file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("could not rename PID file: %w", err)
	}

	return nil
}

// ReadPID reads the PID from the given file path.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file contents: %w", err)
	}

	return pid, nil
}

// RemovePID removes the PID file at the given path.
func RemovePID(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove PID file: %w", err)
	}
	return nil
}

// IsRunning checks if a daemon process is running by reading the PID file
// and checking if the process is alive. Returns running status, PID, and any error.
func IsRunning(path string) (bool, int, error) {
	pid, err := ReadPID(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	if !IsProcessAlive(pid) {
		// Process not running — stale PID file
		return false, pid, nil
	}

	return true, pid, nil
}
