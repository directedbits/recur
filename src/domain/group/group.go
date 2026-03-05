// Package group defines the group aggregate — a named collection of triggers and actions
// within a recurfile.
package group

// Group represents a named group of triggers and actions.
type Group struct {
	ID          string
	Name        string
	RecurfileID string
	TriggerIDs  []string
	ActionIDs   []string
	Options     map[string]any
	Aliases     map[string]string
}
