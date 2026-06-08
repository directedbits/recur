// Package manifest handles loading and validating plugin manifest files (manifest.yaml).
package manifestyaml

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest represents a plugin's manifest.yaml file.
type Manifest struct {
	Name          string          `yaml:"name"`
	Namespace     string          `yaml:"namespace"`
	Version       string          `yaml:"version"`
	Description   string          `yaml:"description,omitempty"`
	Dependencies  []string        `yaml:"dependencies,omitempty"`
	Configuration []ConfigEntry   `yaml:"configuration,omitempty"`
	Triggers      []TriggerDef    `yaml:"triggers,omitempty"`
	Actions       []ActionDef     `yaml:"actions,omitempty"`
}

// ConfigEntry defines a configuration key a plugin accepts.
type ConfigEntry struct {
	Key         string `yaml:"key"`
	Type        string `yaml:"type"`
	Default     any    `yaml:"default,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// TriggerDef defines a trigger exposed by a plugin.
//
// Defaults provides per-trigger fallback values for engine-level settings
// (debounce, concurrency_mode, max_queue_size, error_threshold). These slot
// in between daemon defaults and recurfile group/trigger options — the
// effective precedence is: daemon < plugin manifest < group < trigger.
type TriggerDef struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Options     []OptionDef    `yaml:"options,omitempty"`
	Context     []ContextDef   `yaml:"context,omitempty"`
	Defaults    map[string]any `yaml:"defaults,omitempty"`
}

// triggerDefaultsKeys lists the engine-level settings a plugin manifest may
// pre-set for one of its triggers. Anything outside this list is rejected
// at install time to catch typos early.
var triggerDefaultsKeys = map[string]string{
	"debounce":         "string",
	"concurrency_mode": "string",
	"max_queue_size":   "number",
	"error_threshold":  "number",
}

// ActionDef defines an action exposed by a plugin.
type ActionDef struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description,omitempty"`
	Options     []OptionDef `yaml:"options,omitempty"`
}

// OptionDef defines an option for a trigger or action.
type OptionDef struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Default     any    `yaml:"default,omitempty"`
	Description string `yaml:"description,omitempty"`
	Shorthand   bool   `yaml:"shorthand,omitempty"`
	Sensitive   bool   `yaml:"sensitive,omitempty"`
}

// ContextDef defines a template variable a trigger provides when fired.
type ContextDef struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description,omitempty"`
}

// validTypes lists the accepted option/config/context type values.
var validTypes = map[string]bool{
	"string": true,
	"bool":   true,
	"number": true,
	"list":   true,
	"map":    true,
}

// Load reads and validates a manifest from the given file path.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read manifest: %w", err)
	}
	return Parse(data)
}

// Parse parses and validates manifest YAML bytes.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("could not parse manifest: %w", err)
	}
	if err := validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// validate checks all required fields and constraints.
func validate(m *Manifest) error {
	var errs []string

	if m.Name == "" {
		errs = append(errs, "name is required")
	}
	if m.Namespace == "" {
		errs = append(errs, "namespace is required")
	}
	if m.Version == "" {
		errs = append(errs, "version is required")
	}
	if len(m.Triggers) == 0 && len(m.Actions) == 0 {
		errs = append(errs, "at least one trigger or action is required")
	}

	for i, c := range m.Configuration {
		prefix := fmt.Sprintf("configuration[%d]", i)
		if c.Key == "" {
			errs = append(errs, prefix+": key is required")
		}
		if !validTypes[c.Type] {
			errs = append(errs, fmt.Sprintf("%s: invalid type %q", prefix, c.Type))
		}
		if c.Default != nil {
			if err := checkDefaultType(c.Type, c.Default); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", prefix, err))
			}
		}
	}

	for i, tr := range m.Triggers {
		prefix := fmt.Sprintf("triggers[%d]", i)
		if tr.Name == "" {
			errs = append(errs, prefix+": name is required")
		}
		validateOptions(tr.Options, prefix, false, &errs)
		for key, val := range tr.Defaults {
			typ, ok := triggerDefaultsKeys[key]
			if !ok {
				errs = append(errs, fmt.Sprintf("%s.defaults: unknown key %q", prefix, key))
				continue
			}
			if err := checkDefaultType(typ, val); err != nil {
				errs = append(errs, fmt.Sprintf("%s.defaults.%s: %v", prefix, key, err))
			}
		}
		for j, ctx := range tr.Context {
			ctxPrefix := fmt.Sprintf("%s.context[%d]", prefix, j)
			if ctx.Name == "" {
				errs = append(errs, ctxPrefix+": name is required")
			}
			if !validTypes[ctx.Type] {
				errs = append(errs, fmt.Sprintf("%s: invalid type %q", ctxPrefix, ctx.Type))
			}
		}
	}

	for i, act := range m.Actions {
		prefix := fmt.Sprintf("actions[%d]", i)
		if act.Name == "" {
			errs = append(errs, prefix+": name is required")
		}
		validateOptions(act.Options, prefix, true, &errs)
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid manifest:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// validateOptions checks option definitions. allowShorthand is true for action options.
func validateOptions(opts []OptionDef, prefix string, allowShorthand bool, errs *[]string) {
	shorthandCount := 0
	for j, opt := range opts {
		optPrefix := fmt.Sprintf("%s.options[%d]", prefix, j)
		if opt.Name == "" {
			*errs = append(*errs, optPrefix+": name is required")
		}
		if !validTypes[opt.Type] {
			*errs = append(*errs, fmt.Sprintf("%s: invalid type %q", optPrefix, opt.Type))
		}
		if opt.Shorthand {
			if !allowShorthand {
				*errs = append(*errs, optPrefix+": shorthand is only valid on action options")
			}
			shorthandCount++
		}
		if opt.Default != nil {
			if err := checkDefaultType(opt.Type, opt.Default); err != nil {
				*errs = append(*errs, fmt.Sprintf("%s: %v", optPrefix, err))
			}
		}
	}
	if allowShorthand && shorthandCount > 1 {
		*errs = append(*errs, prefix+": at most one option may be marked shorthand")
	}
}

// checkDefaultType verifies that a default value matches the declared type.
func checkDefaultType(typ string, value any) error {
	switch typ {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("default value must be a string, got %T", value)
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("default value must be a bool, got %T", value)
		}
	case "number":
		switch value.(type) {
		case int, float64:
			// ok
		default:
			return fmt.Errorf("default value must be a number, got %T", value)
		}
	case "list":
		switch value.(type) {
		case []any, []string, []int, []float64:
			// ok
		default:
			return fmt.Errorf("default value must be a list, got %T", value)
		}
	case "map":
		switch value.(type) {
		case map[string]any, map[any]any:
			// ok
		default:
			return fmt.Errorf("default value must be a map, got %T", value)
		}
	}
	return nil
}
