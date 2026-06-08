package recurfileyaml

import (
	"fmt"
	"strings"

	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
)

// StubTrigger holds the trigger type and optional manifest options for stub generation.
type StubTrigger struct {
	Type    string
	Options []manifestyaml.OptionDef // nil when --stub is not used
}

// StubAction holds the action type, optional manifest options, and shorthand info.
type StubAction struct {
	Type      string
	Options   []manifestyaml.OptionDef // nil when --stub is not used
	Shorthand string               // name of the shorthand option, if any
}

// GenerateGroupStub produces a YAML fragment for a single group with the given
// triggers and actions. When options are provided (--stub mode), they are rendered
// with defaults and description comments per the plan rules:
//   - Options without defaults → uncommented, empty value
//   - Options with defaults → commented out with default value shown
//   - Description as inline YAML comment
//   - Shorthand actions use shorthand form
func GenerateGroupStub(name string, triggers []StubTrigger, actions []StubAction) string {
	var b strings.Builder

	b.WriteString(name + ":\n")

	if len(triggers) > 0 {
		b.WriteString("  on:\n")
		for _, t := range triggers {
			b.WriteString("    - type: " + t.Type + "\n")
			if len(t.Options) > 0 {
				b.WriteString("      options:\n")
				for _, opt := range t.Options {
					writeOption(&b, "        ", opt)
				}
			}
		}
	}

	if len(actions) > 0 {
		b.WriteString("  do:\n")
		for _, a := range actions {
			if a.Shorthand != "" {
				// Shorthand form: - ActionType: "default"
				val := shorthandDefault(a.Options, a.Shorthand)
				desc := shorthandDescription(a.Options, a.Shorthand)
				if desc != "" {
					b.WriteString(fmt.Sprintf("    - %s: %s  # %s\n", a.Type, quoteYAML(val), desc))
				} else {
					b.WriteString(fmt.Sprintf("    - %s: %s\n", a.Type, quoteYAML(val)))
				}
			} else if len(a.Options) > 0 {
				// Detailed form with options
				b.WriteString("    - type: " + a.Type + "\n")
				b.WriteString("      options:\n")
				for _, opt := range a.Options {
					writeOption(&b, "        ", opt)
				}
			} else {
				// Bare action, no options known
				b.WriteString("    - type: " + a.Type + "\n")
			}
		}
	}

	return b.String()
}

// writeOption writes a single option line. Options with a non-nil default are
// commented out; options without a default are uncommented with an empty value.
func writeOption(b *strings.Builder, indent string, opt manifestyaml.OptionDef) {
	if opt.Default != nil {
		// Commented out with default value
		val := formatDefault(opt.Default)
		if opt.Description != "" {
			b.WriteString(fmt.Sprintf("%s# %s: %s  # %s\n", indent, opt.Name, quoteYAML(val), opt.Description))
		} else {
			b.WriteString(fmt.Sprintf("%s# %s: %s\n", indent, opt.Name, quoteYAML(val)))
		}
	} else {
		// Uncommented, empty value (required)
		if opt.Description != "" {
			b.WriteString(fmt.Sprintf("%s%s: \"\"  # %s\n", indent, opt.Name, opt.Description))
		} else {
			b.WriteString(fmt.Sprintf("%s%s: \"\"\n", indent, opt.Name))
		}
	}
}

// formatDefault converts a default value to its string representation.
func formatDefault(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%g", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// quoteYAML wraps a value in double quotes for YAML output.
func quoteYAML(s string) string {
	return fmt.Sprintf("%q", s)
}

// shorthandDefault returns the default value for the shorthand option, or empty string.
func shorthandDefault(options []manifestyaml.OptionDef, shorthandName string) string {
	for _, opt := range options {
		if opt.Name == shorthandName && opt.Default != nil {
			return formatDefault(opt.Default)
		}
	}
	return ""
}

// shorthandDescription returns the description for the shorthand option.
func shorthandDescription(options []manifestyaml.OptionDef, shorthandName string) string {
	for _, opt := range options {
		if opt.Name == shorthandName {
			return opt.Description
		}
	}
	return ""
}
