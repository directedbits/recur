// Package plugin defines the plugin aggregate — loadable extensions that provide triggers and actions.
package plugin

// OptionType represents the type of a plugin option value.
type OptionType string

const (
	TypeString OptionType = "string"
	TypeBool   OptionType = "bool"
	TypeNumber OptionType = "number"
	TypeList   OptionType = "list"
	TypeMap    OptionType = "map"
)

// Plugin is the domain projection of a discovered plugin's manifest. It
// carries the metadata fields plus the trigger and action types the plugin
// declares. Lookup helpers (UnknownTypes, TriggerDefaultsFor, …) operate
// on slices of *Plugin.
type Plugin struct {
	ID          string
	Name        string
	Namespace   string
	Version     string
	Description string
	Triggers    []TriggerType
	Actions     []ActionType
}

// TriggerType is the domain projection of one trigger declared by a plugin.
// Defaults carries the manifest's per-trigger engine-level defaults
// (concurrency_mode, max_queue_size, debounce, error_threshold,
// action_error_threshold) that feed the precedence chain inside
// domain/recurfile.BuildTriggerSettings.
type TriggerType struct {
	Name     string
	Defaults map[string]any
}

// ActionType is the domain projection of one action declared by a plugin.
type ActionType struct {
	Name string
}

// Option defines a configurable option exposed by a trigger or action.
type Option struct {
	Name        string
	Type        OptionType
	Default     any
	Description string
	Shorthand   bool
}
