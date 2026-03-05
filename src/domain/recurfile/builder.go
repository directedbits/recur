// Package recurfile builder.go encodes the rules for turning a resolved
// RawFile into the data fields of trigger/action/group/recurfile entities.
//
// The most important rule here is the **trigger-settings precedence chain**.
// Engine-level settings (concurrency_mode, max_queue_size, debounce,
// error_threshold, action_error_threshold) flow through five layers, from
// least- to most-specific:
//
//	daemon < plugin manifest < plugin override < group < trigger
//
//   - **daemon**          — the daemon's global trigger_defaults from
//     ~/.config/recur/config.yaml.
//   - **plugin manifest** — the plugin's per-trigger Defaults block; lets
//     a trigger type opt out of daemon-wide defaults that don't suit its
//     event model.
//   - **plugin override** — `plugins.<namespace>.trigger_defaults` in the
//     daemon config; the user's per-plugin escape hatch above what the
//     plugin shipped.
//   - **group**           — options at the recurfile group level.
//   - **trigger**         — options on the trigger instance itself.
//
// Each layer is a flat map[string]any; the most-specific layer that defines
// a key wins. The registry assembles the layer values from its sources
// (config, manifests, recurfile) and passes them to BuildTriggerSettings.
package recurfile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	pkgconfig "github.com/directedbits/recur/pkg/config"
)

// TriggerSettings holds the resolved engine-level settings for a single
// trigger instance, after the precedence chain has been applied.
type TriggerSettings struct {
	ConcurrencyMode      string
	MaxQueueSize         int
	Debounce             time.Duration
	ErrorThreshold       int
	ActionErrorThreshold int
}

// BuildTriggerSettings applies the precedence chain documented at the top of
// this file. Any layer may be nil to skip it.
func BuildTriggerSettings(daemon, pluginManifest, pluginOverride, group, trigger map[string]any) TriggerSettings {
	store := pkgconfig.NewMapStore("daemon", "plugin", "plugin_override", "group", "trigger")
	if daemon != nil {
		store.Set("daemon", daemon)
	}
	if pluginManifest != nil {
		store.Set("plugin", pluginManifest)
	}
	if pluginOverride != nil {
		store.Set("plugin_override", pluginOverride)
	}
	if group != nil {
		store.Set("group", group)
	}
	if trigger != nil {
		store.Set("trigger", trigger)
	}
	return TriggerSettings{
		ConcurrencyMode:      mapStoreString(store, "concurrency_mode"),
		MaxQueueSize:         mapStoreInt(store, "max_queue_size"),
		Debounce:             mapStoreDuration(store, "debounce"),
		ErrorThreshold:       mapStoreInt(store, "error_threshold"),
		ActionErrorThreshold: mapStoreInt(store, "action_error_threshold"),
	}
}

// EntityID computes a stable hex-encoded SHA-256 prefix from an entity type
// and a seed string. The same (entityType, seed) pair always yields the
// same ID so reloads keep entity identities stable across daemon restarts.
//
// Recommended seeds:
//
//	recurfile: absolute file path
//	group:     <recurfileID>:<groupName>
//	trigger:   <groupID>:<triggerType>:<occurrenceIndexWithinGroup>
//	action:    <triggerID>:<actionType>:<occurrenceIndexWithinTrigger>
//
// The occurrence-index suffixes ensure that same-typed siblings get
// different IDs, and that only *reordering* same-typed siblings changes IDs.
func EntityID(entityType, seed string) string {
	h := sha256.Sum256([]byte(entityType + ":" + seed))
	return hex.EncodeToString(h[:6]) // 12-char hex
}

// TriggerSeed and ActionSeed build the canonical seed strings used with
// EntityID. The occurrence index is the zero-based count of prior siblings
// of the same type within the parent scope.
func TriggerSeed(groupID, triggerType string, occurrence int) string {
	return fmt.Sprintf("%s:%s:%d", groupID, triggerType, occurrence)
}

func ActionSeed(triggerID, actionType string, occurrence int) string {
	return fmt.Sprintf("%s:%s:%d", triggerID, actionType, occurrence)
}

func GroupSeed(recurfileID, groupName string) string {
	return recurfileID + ":" + groupName
}

// mapStoreString reads a string from the precedence chain, returning "" when
// the key is unset or the value is not a string.
func mapStoreString(store *pkgconfig.MapStore, key string) string {
	v, ok := store.GetField(key)
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// mapStoreInt reads an int from the precedence chain. YAML unmarshals
// numbers as float64, so both int and float64 are accepted. Returns 0 when
// the key is unset or the value is neither.
func mapStoreInt(store *pkgconfig.MapStore, key string) int {
	v, ok := store.GetField(key)
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

// mapStoreDuration reads a duration from the precedence chain. Strings are
// parsed with time.ParseDuration. Returns 0 when the key is unset, the
// string is unparseable, or the value is neither a string nor a Duration.
func mapStoreDuration(store *pkgconfig.MapStore, key string) time.Duration {
	v, ok := store.GetField(key)
	if !ok {
		return 0
	}
	switch d := v.(type) {
	case string:
		if parsed, err := time.ParseDuration(d); err == nil {
			return parsed
		}
	case time.Duration:
		return d
	}
	return 0
}
