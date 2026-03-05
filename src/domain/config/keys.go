package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	pkgconfig "github.com/directedbits/recur/pkg/config"
)

// KeyDef describes a config key's mapping, type, and validation rules.
type KeyDef struct {
	Field    string          // PascalCase struct field name
	Type     string          // "string" or "int"
	Validate map[string]bool // allowed values (nil = no validation)
	Fallback string          // yaml key to fall back to when nil (e.g., "error_threshold")
}

// Keys maps YAML config key names to their definitions.
var Keys = map[string]KeyDef{
	"default_shell":           {Field: "DefaultShell", Type: "string"},
	"error_threshold":         {Field: "ErrorThreshold", Type: "int"},
	"trigger_error_threshold": {Field: "TriggerErrorThreshold", Type: "int", Fallback: "error_threshold"},
	"action_error_threshold":  {Field: "ActionErrorThreshold", Type: "int", Fallback: "error_threshold"},
	"concurrency_mode":        {Field: "ConcurrencyMode", Type: "string", Validate: ValidConcurrencyModes},
	"max_queue_size":          {Field: "MaxQueueSize", Type: "int"},
	"debounce":                {Field: "Debounce", Type: "string"},
	"shutdown_timeout":        {Field: "ShutdownTimeout", Type: "string"},
	"log_level":               {Field: "LogLevel", Type: "string", Validate: ValidLogLevels},
	"socket_address":          {Field: "SocketAddress", Type: "string"},
	"allowed_hosts":           {Field: "AllowedHosts", Type: "string"},
}

// ValidConcurrencyModes lists acceptable concurrency_mode values.
var ValidConcurrencyModes = map[string]bool{
	"queue":    true,
	"parallel": true,
	"drop":     true,
	"abort":    true,
}

// ValidLogLevels lists acceptable log_level values.
var ValidLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// KeyValue represents a config key-value pair for display.
type KeyValue struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// LookupKey returns the key definition for a yaml key, or an error if unknown.
// Handles both plain keys and plugin dot-path keys.
func LookupKey(yamlKey string) (*KeyDef, error) {
	if strings.HasPrefix(yamlKey, "plugins.") {
		return &KeyDef{Type: "string"}, nil
	}
	def, ok := Keys[yamlKey]
	if !ok {
		return nil, fmt.Errorf("unknown config key: %s", yamlKey)
	}
	return &def, nil
}

// GetByKey retrieves the effective value for a yaml key from the store.
// Handles threshold fallbacks (trigger_error_threshold → error_threshold).
func GetByKey(store *pkgconfig.Store[Config], yamlKey string) (any, error) {
	if strings.HasPrefix(yamlKey, "plugins.") {
		return getPluginByKey(store, yamlKey)
	}

	def, err := LookupKey(yamlKey)
	if err != nil {
		return nil, err
	}

	val, err := store.GetField(def.Field)
	if err != nil {
		return nil, err
	}

	if def.Fallback != "" && isNilValue(val) {
		fallbackDef, ok := Keys[def.Fallback]
		if ok {
			val, _ = store.GetField(fallbackDef.Field)
		}
	}

	return derefValue(val), nil
}

// SetByKey parses a string value and sets it on a layer in the store.
// Validates the value against the key's allowed values if defined.
func SetByKey(store *pkgconfig.Store[Config], layer, yamlKey, value string) error {
	if strings.HasPrefix(yamlKey, "plugins.") {
		return setPluginByKey(store, layer, yamlKey, value)
	}

	def, err := LookupKey(yamlKey)
	if err != nil {
		return err
	}

	if def.Validate != nil {
		normalized := strings.ToLower(value)
		if !def.Validate[normalized] {
			allowed := make([]string, 0, len(def.Validate))
			for k := range def.Validate {
				allowed = append(allowed, k)
			}
			return fmt.Errorf("invalid %s: %q (must be one of: %s)", yamlKey, value, strings.Join(allowed, ", "))
		}
		value = normalized
	}

	switch def.Type {
	case "int":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: expected integer, got %q", yamlKey, value)
		}
		return store.SetField(layer, def.Field, &v)
	case "string":
		return store.SetField(layer, def.Field, &value)
	}

	return nil
}

// DeleteByKey clears a key from a layer in the store.
func DeleteByKey(store *pkgconfig.Store[Config], layer, yamlKey string) error {
	if strings.HasPrefix(yamlKey, "plugins.") {
		return deletePluginByKey(store, layer, yamlKey)
	}

	def, err := LookupKey(yamlKey)
	if err != nil {
		return err
	}

	return store.ClearLayerField(layer, def.Field)
}

// AllKeys returns all config values as KeyValue pairs, ordered consistently.
func AllKeys(store *pkgconfig.Store[Config]) []KeyValue {
	cfg := store.Get()
	errThreshold := DerefInt(cfg.ErrorThreshold)

	result := []KeyValue{
		{"default_shell", DerefStr(cfg.DefaultShell)},
		{"error_threshold", errThreshold},
		{"trigger_error_threshold", EffectiveInt(cfg.TriggerErrorThreshold, errThreshold)},
		{"action_error_threshold", EffectiveInt(cfg.ActionErrorThreshold, errThreshold)},
		{"concurrency_mode", DerefStr(cfg.ConcurrencyMode)},
		{"max_queue_size", DerefInt(cfg.MaxQueueSize)},
		{"debounce", DerefStr(cfg.Debounce)},
		{"shutdown_timeout", DerefStr(cfg.ShutdownTimeout)},
		{"log_level", DerefStr(cfg.LogLevel)},
		{"socket_address", DerefStr(cfg.SocketAddress)},
		{"allowed_hosts", DerefStr(cfg.AllowedHosts)},
	}

	for ns, keys := range cfg.Plugins {
		for k, v := range keys {
			result = append(result, KeyValue{
				Key:   fmt.Sprintf("plugins.%s.%s", ns, k),
				Value: v,
			})
		}
	}

	return result
}

// --- Plugin key helpers ---

func getPluginByKey(store *pkgconfig.Store[Config], yamlKey string) (any, error) {
	cfg := store.Get()
	ns, field, err := SplitPluginKey(yamlKey)
	if err != nil {
		return nil, err
	}
	if cfg.Plugins == nil {
		return nil, fmt.Errorf("no plugin config set for namespace %q", ns)
	}
	nsConfig, ok := cfg.Plugins[ns]
	if !ok {
		return nil, fmt.Errorf("no plugin config set for namespace %q", ns)
	}
	v, ok := nsConfig[field]
	if !ok {
		return nil, fmt.Errorf("plugin config key %q not set for namespace %q", field, ns)
	}
	return v, nil
}

func setPluginByKey(store *pkgconfig.Store[Config], layer, yamlKey, value string) error {
	ns, field, err := SplitPluginKey(yamlKey)
	if err != nil {
		return err
	}

	layerVal, ok := store.GetLayer(layer)
	if !ok {
		layerVal = Config{}
	}
	if layerVal.Plugins == nil {
		layerVal.Plugins = make(map[string]map[string]any)
	}
	if layerVal.Plugins[ns] == nil {
		layerVal.Plugins[ns] = make(map[string]any)
	}
	layerVal.Plugins[ns][field] = value
	store.Set(layer, layerVal)
	return nil
}

func deletePluginByKey(store *pkgconfig.Store[Config], layer, yamlKey string) error {
	ns, field, err := SplitPluginKey(yamlKey)
	if err != nil {
		return err
	}

	layerVal, ok := store.GetLayer(layer)
	if !ok {
		return nil
	}
	if layerVal.Plugins == nil {
		return nil
	}
	nsConfig, ok := layerVal.Plugins[ns]
	if !ok {
		return nil
	}
	delete(nsConfig, field)
	if len(nsConfig) == 0 {
		delete(layerVal.Plugins, ns)
	}
	store.Set(layer, layerVal)
	return nil
}

// SplitPluginKey splits a plugin yaml key into namespace and field.
// Expects the full "plugins."-prefixed key. The field is the last dot-separated segment.
func SplitPluginKey(yamlKey string) (namespace string, field string, err error) {
	parts := strings.SplitN(yamlKey, ".", 2)
	if len(parts) < 2 || parts[1] == "" {
		return "", "", fmt.Errorf("invalid plugin config key: %s (expected plugins.<namespace>.<key>)", yamlKey)
	}

	rest := parts[1]
	lastDot := strings.LastIndex(rest, ".")
	if lastDot < 1 {
		return "", "", fmt.Errorf("expected <namespace>.<key>, got %q", rest)
	}
	return rest[:lastDot], rest[lastDot+1:], nil
}

func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil()
}

// derefValue dereferences a pointer value to its underlying value.
// Returns nil for nil pointers, passes non-pointers through unchanged.
func derefValue(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		return rv.Elem().Interface()
	}
	return v
}
