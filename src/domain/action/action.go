// Package action defines the action aggregate — units of work executed when a trigger fires.
package action

import "time"

// Status represents the current state of an action.
type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusError     Status = "error"
)

// Action represents a registered action instance.
type Action struct {
	ID             string
	Type           string
	Name           string // optional user-defined label for organization
	GroupID        string
	GroupName      string
	TriggerID      string
	RecurfileID    string
	RecurfilePath  string
	PluginID       string
	Options        map[string]any
	Status         Status
	ErrorCount     int
	ErrorThreshold int // max consecutive errors before the action is disabled (from config)
	LastExecuted   time.Time
}

// ExecutionResult captures the outcome of running an action.
type ExecutionResult struct {
	ActionID   string
	ActionType string
	Success    bool
	ExitCode   int
	Output     string
	Error      string
	Duration   string
}
