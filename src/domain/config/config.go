// Package config defines the daemon configuration shape and pointer helpers.
// This is pure domain: no file I/O, no platform-specific defaults, no
// interpolation of environment variables. Those live in infra (yaml/config,
// os/defaults). Callers that build, inspect, or mutate a Config value reach
// for this package; callers that need to load or persist one reach for
// infra/yaml/config.
package config

import "strings"

// Config represents the daemon configuration.
//
// Pointer fields use nil to indicate "not set" — this lets the overlay
// system distinguish between "explicitly set to zero" and "unset". After a
// store has been hydrated from defaults plus any user layers, every
// pointer field provided by DefaultConfig is guaranteed non-nil.
type Config struct {
	DefaultShell          *string                   `yaml:"default_shell,omitempty"`
	ErrorThreshold        *int                      `yaml:"error_threshold,omitempty"`
	TriggerErrorThreshold *int                      `yaml:"trigger_error_threshold,omitempty"`
	ActionErrorThreshold  *int                      `yaml:"action_error_threshold,omitempty"`
	ConcurrencyMode       *string                   `yaml:"concurrency_mode,omitempty"`
	MaxQueueSize          *int                      `yaml:"max_queue_size,omitempty"`
	Debounce              *string                   `yaml:"debounce,omitempty"`
	ShutdownTimeout       *string                   `yaml:"shutdown_timeout,omitempty"`
	LogLevel              *string                   `yaml:"log_level,omitempty"`
	SocketAddress         *string                   `yaml:"socket_address,omitempty"`
	AllowedHosts          *string                   `yaml:"allowed_hosts,omitempty"`
	Plugins               map[string]map[string]any `yaml:"plugins,omitempty"`
}

// IsHostAllowed checks whether a hostname is in the AllowedHosts list. The
// list is a comma-separated string compared case-insensitively after
// trimming whitespace.
func (cfg *Config) IsHostAllowed(host string) bool {
	allowed := DerefStr(cfg.AllowedHosts)
	if allowed == "" {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	for _, h := range strings.Split(allowed, ",") {
		if strings.ToLower(strings.TrimSpace(h)) == host {
			return true
		}
	}
	return false
}

// DerefStr returns the pointed-to string, or "" if nil.
func DerefStr(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// DerefInt returns the pointed-to int, or 0 if nil.
func DerefInt(p *int) int {
	if p != nil {
		return *p
	}
	return 0
}

// EffectiveInt returns the pointed-to int, or fallback if nil.
func EffectiveInt(ptr *int, fallback int) int {
	if ptr != nil {
		return *ptr
	}
	return fallback
}
