// Package trigger defines the trigger aggregate — the core unit of event detection.
// A trigger watches for a specific condition (e.g., file created) and fires when met.
package trigger

import (
	"time"

	"github.com/directedbits/recur/src/domain/plugin"
)

// Status represents the current state of a trigger.
type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusError     Status = "error"
)

// Trigger represents a registered trigger instance.
type Trigger struct {
	ID              string
	Type            string
	Name            string // optional user-defined label for organization
	GroupID         string
	GroupName       string
	RecurfileID     string
	RecurfilePath   string
	PluginID        string
	Options         map[string]any
	Status          Status
	ErrorCount      int
	ErrorThreshold  int // max consecutive errors before deactivation (from config)
	LastFired       time.Time
	ConcurrencyMode string        // queue, parallel, drop, or abort
	MaxQueueSize    int
	Debounce        time.Duration
}

// ContextVar represents a template variable provided when a trigger fires.
type ContextVar struct {
	Name  string
	Type  plugin.OptionType
	Value any
}
