// Package state handles reading and writing daemon state to ~/.config/recur/state/state.json.
// The state file tracks operational data — which recurfiles are registered, trigger/action
// status, error counts, etc. Recurfiles remain the source of truth for *what* is configured;
// the state file tracks *how it's running*.
package statejsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/directedbits/recur/src/infra/fs/atomicfile"
)

// LaunchArgs records the arguments used to start the daemon, so that
// restart can replay them exactly.
type LaunchArgs struct {
	ConfigPath    string `json:"config_path,omitempty"`
	SocketAddress string `json:"socket_address,omitempty"`
	LogLevel      string `json:"log_level,omitempty"`
	Foreground    bool   `json:"foreground,omitempty"`
}

// File represents the persisted daemon state.
type File struct {
	LaunchArgs *LaunchArgs      `json:"launch_args,omitempty"`
	Recurfiles []RecurfileState `json:"recurfiles"`
}

// LoadLaunchArgs is a convenience function that loads only the LaunchArgs
// from the state file at the given path. Returns nil (no error) if the file
// does not exist or contains no launch args.
func LoadLaunchArgs(path string) (*LaunchArgs, error) {
	f, err := Load(path)
	if err != nil {
		return nil, err
	}
	return f.LaunchArgs, nil
}

// RecurfileState tracks a registered recurfile and its entity states.
type RecurfileState struct {
	ID       string        `json:"id"`
	FilePath string        `json:"file_path"`
	Triggers []EntityState `json:"triggers,omitempty"`
	Actions  []EntityState `json:"actions,omitempty"`
}

// EntityState tracks the operational state of a trigger or action.
type EntityState struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	ErrorCount   int    `json:"error_count,omitempty"`
	LastActivity string `json:"last_activity,omitempty"` // RFC 3339 timestamp: last_fired (trigger) or last_executed (action)
}

// DefaultPath returns the default state file path (~/.config/recur/state/state.json).
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "recur", "state", "state.json"), nil
}

// Recover promotes any orphaned temp files from interrupted writes.
// Call this once at startup before Load.
func Recover(path string) error {
	return atomicfile.Recover(path)
}

// Load reads and parses the state file. Returns an empty File if the file doesn't exist.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &f, nil
}

// Save writes the state file atomically using a timestamped temp file.
func Save(f *File, path string) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	data = append(data, '\n')

	return atomicfile.Write(path, data)
}
