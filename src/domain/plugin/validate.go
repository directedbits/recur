package plugin

import "strings"

// BuiltinActionNames lists action types the daemon provides out of the box,
// without any installed plugin. Currently just the shell action.
var BuiltinActionNames = []string{"shell"}

// KnownTriggerNames returns every trigger name declared by any plugin.
// Order matches the input plugin order; within a plugin, manifest order.
func KnownTriggerNames(plugins []*Plugin) []string {
	var out []string
	for _, p := range plugins {
		if p == nil {
			continue
		}
		for _, t := range p.Triggers {
			out = append(out, t.Name)
		}
	}
	return out
}

// KnownActionNames returns every action name declared by any plugin,
// prepended with the built-in action names.
func KnownActionNames(plugins []*Plugin) []string {
	out := append([]string(nil), BuiltinActionNames...)
	for _, p := range plugins {
		if p == nil {
			continue
		}
		for _, a := range p.Actions {
			out = append(out, a.Name)
		}
	}
	return out
}

// UnknownTypes returns the subset of trigger/action names not declared by
// any plugin (or by the built-in action list). Comparison is
// case-insensitive. The result preserves input order so the caller can
// emit error messages in the order the user provided.
func UnknownTypes(plugins []*Plugin, triggers, actions []string) (unknownTriggers, unknownActions []string) {
	known := func(needle string, haystack []string) bool {
		for _, h := range haystack {
			if strings.EqualFold(h, needle) {
				return true
			}
		}
		return false
	}
	knownTriggers := KnownTriggerNames(plugins)
	knownActions := KnownActionNames(plugins)
	for _, t := range triggers {
		if !known(t, knownTriggers) {
			unknownTriggers = append(unknownTriggers, t)
		}
	}
	for _, a := range actions {
		if !known(a, knownActions) {
			unknownActions = append(unknownActions, a)
		}
	}
	return unknownTriggers, unknownActions
}
