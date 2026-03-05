package plugin

// TriggerDefaultsFor returns the namespace of the plugin that owns the
// given trigger type and that trigger's manifest-declared engine-level
// defaults. The namespace is returned even when defaults is empty so
// callers can still look up daemon-level overrides keyed by that namespace
// (eg. plugins.<namespace>.trigger_defaults in the daemon config).
//
// Returns ("", nil) if no plugin owns the type.
//
// The first match wins, so if two plugins declare the same trigger name
// (a conflict that infra/plugin.CheckConflicts flags at startup), the
// earlier plugin in the slice provides the defaults.
func TriggerDefaultsFor(plugins []*Plugin, triggerType string) (namespace string, defaults map[string]any) {
	for _, p := range plugins {
		if p == nil {
			continue
		}
		for _, t := range p.Triggers {
			if t.Name == triggerType {
				if len(t.Defaults) == 0 {
					return p.Namespace, nil
				}
				return p.Namespace, t.Defaults
			}
		}
	}
	return "", nil
}
