// Package config provides a generic, application-agnostic configuration
// overlay system with named layers and a repository pattern.
//
// The core concept is that configuration comes from multiple sources
// (defaults, files, CLI flags, etc.) organized as named layers from
// least-specific to most-specific. The repository computes the effective
// configuration by overlaying layers on demand.
package config

// LayerValue represents a field's value at a specific layer.
// Used by Inspect to show the full resolution chain for diagnostics.
type LayerValue struct {
	Layer   string // Layer name (e.g., "default", "file", "cli args")
	Value   any    // The field's value at this layer
	Defined bool   // Whether the field is defined (set) at this layer
}
