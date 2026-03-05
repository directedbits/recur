// Package recurfile handles loading and validating recurfile YAML documents.
package recurfileyaml

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	domainrf "github.com/directedbits/recur/src/domain/recurfile"
	"github.com/directedbits/recur/src/domain/secret"
)

// The structural types (RawFile, RawGroup, RawTrigger, RawAction) live in
// domain/recurfile; SecretDef lives in domain/secret. This package re-exports
// them so callers that already import infra/recurfile (for the parser) get
// the type names for free.
func init() {
	// Make Parse available to domain/recurfile.Merge so the domain merge
	// logic can validate input without importing infra.
	domainrf.RegisterParser(Parse)
}

type (
	SecretDef  = secret.SecretDef
	RawFile    = domainrf.RawFile
	RawGroup   = domainrf.RawGroup
	RawTrigger = domainrf.RawTrigger
	RawAction  = domainrf.RawAction
)

// Load reads and parses a recurfile from the given path.
func Load(path string) (*RawFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read recurfile: %w", err)
	}

	f, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("recurfile %s: %w", path, err)
	}
	f.Path = path

	// Resolve relative !file secret paths against the recurfile's directory so
	// that the daemon's working directory does not affect lookup.
	dir := filepath.Dir(path)
	for i, s := range f.Secrets {
		if s.Source == "file" && !filepath.IsAbs(s.Ref) {
			f.Secrets[i].Ref = filepath.Join(dir, s.Ref)
		}
	}

	return f, nil
}

// Parse parses recurfile YAML data and returns a validated File.
func Parse(data []byte) (*RawFile, error) {
	// Parse into ordered map to preserve group order and detect reserved keys.
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("expected YAML document")
	}

	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected top-level mapping, got %s", nodeKindName(mapping.Kind))
	}

	f := &RawFile{}
	var errs []string

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		key := keyNode.Value

		if key == "aliases" {
			aliases, err := parseAliases(valNode)
			if err != nil {
				errs = append(errs, fmt.Sprintf("aliases: %v", err))
				continue
			}
			f.Aliases = aliases
			continue
		}

		if key == "secrets" {
			secrets, secretErrs := parseSecrets(valNode)
			for _, e := range secretErrs {
				errs = append(errs, fmt.Sprintf("secrets: %s", e))
			}
			f.Secrets = secrets
			continue
		}

		// Everything else is a group
		group, groupErrs := parseGroup(key, valNode)
		if len(groupErrs) > 0 {
			for _, e := range groupErrs {
				errs = append(errs, fmt.Sprintf("group %q: %s", key, e))
			}
		}
		if group != nil {
			f.Groups = append(f.Groups, *group)
		}
	}

	if len(f.Groups) == 0 && len(errs) == 0 {
		errs = append(errs, "recurfile must define at least one group")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("validation errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return f, nil
}

func parseAliases(node *yaml.Node) (map[string]string, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping")
	}
	aliases := make(map[string]string)
	for i := 0; i < len(node.Content)-1; i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1].Value
		if k == "" || v == "" {
			return nil, fmt.Errorf("alias key and value must be non-empty strings")
		}
		aliases[k] = v
	}
	return aliases, nil
}

func parseGroup(name string, node *yaml.Node) (*RawGroup, []string) {
	if node.Kind != yaml.MappingNode {
		return nil, []string{"expected mapping"}
	}

	g := &RawGroup{Name: name}
	var errs []string

	// Decode the group node into a raw map to process known keys
	raw := make(map[string]yaml.Node)
	for i := 0; i < len(node.Content)-1; i += 2 {
		raw[node.Content[i].Value] = *node.Content[i+1]
	}

	// Aliases
	if n, ok := raw["aliases"]; ok {
		aliases, err := parseAliases(&n)
		if err != nil {
			errs = append(errs, fmt.Sprintf("aliases: %v", err))
		} else {
			g.Aliases = aliases
		}
	}

	// Options
	if n, ok := raw["options"]; ok {
		var opts map[string]any
		if err := n.Decode(&opts); err != nil {
			errs = append(errs, fmt.Sprintf("options: %v", err))
		} else {
			g.Options = opts
		}
	}

	// Triggers (key is "on", but accept "triggers" as a common mistake)
	if n, ok := raw["on"]; ok {
		triggers, trigErrs := parseTriggers(&n)
		errs = append(errs, trigErrs...)
		g.Triggers = triggers
	} else if n, ok := raw["triggers"]; ok {
		triggers, trigErrs := parseTriggers(&n)
		errs = append(errs, trigErrs...)
		g.Triggers = triggers
	} else {
		errs = append(errs, "missing required key: on")
	}

	// Do (group-level default actions)
	if n, ok := raw["do"]; ok {
		actions, actErrs := parseActions(&n)
		errs = append(errs, actErrs...)
		g.Actions = actions
	}

	// Warn about unknown keys
	for k := range raw {
		switch k {
		case "aliases", "options", "on", "triggers", "do":
			// known ("triggers" is accepted as synonym for "on")
		default:
			errs = append(errs, fmt.Sprintf("unknown key: %q", k))
		}
	}

	return g, errs
}

func parseTriggers(node *yaml.Node) ([]RawTrigger, []string) {
	if node.Kind != yaml.SequenceNode {
		return nil, []string{"on: expected list"}
	}

	if len(node.Content) == 0 {
		return nil, []string{"on: must not be empty"}
	}

	var triggers []RawTrigger
	var errs []string

	for i, item := range node.Content {
		t, trigErrs := parseTrigger(i, item)
		for _, e := range trigErrs {
			errs = append(errs, fmt.Sprintf("trigger[%d]: %s", i, e))
		}
		if t != nil {
			triggers = append(triggers, *t)
		}
	}

	return triggers, errs
}

func parseTrigger(index int, node *yaml.Node) (*RawTrigger, []string) {
	node = resolveNode(node)
	if node.Kind != yaml.MappingNode {
		return nil, []string{"expected mapping"}
	}

	raw := make(map[string]yaml.Node)
	for i := 0; i < len(node.Content)-1; i += 2 {
		raw[node.Content[i].Value] = *node.Content[i+1]
	}

	t := &RawTrigger{}
	var errs []string

	// Type
	if n, ok := raw["type"]; ok {
		t.Type = n.Value
		if t.Type == "" {
			errs = append(errs, "type must be a non-empty string")
		}
	} else {
		errs = append(errs, "missing required key: type")
	}

	// Name (optional label)
	if n, ok := raw["name"]; ok {
		t.Name = n.Value
	}

	// Options
	if n, ok := raw["options"]; ok {
		var opts map[string]any
		if err := n.Decode(&opts); err != nil {
			errs = append(errs, fmt.Sprintf("options: %v", err))
		} else {
			t.Options = opts
		}
	}

	// Do (trigger-level actions)
	if n, ok := raw["do"]; ok {
		actions, actErrs := parseActions(&n)
		errs = append(errs, actErrs...)
		t.Actions = actions
	}

	// Unknown keys
	for k := range raw {
		switch k {
		case "type", "name", "options", "do":
			// known
		default:
			errs = append(errs, fmt.Sprintf("unknown key: %q", k))
		}
	}

	return t, errs
}

func parseActions(node *yaml.Node) ([]RawAction, []string) {
	if node.Kind != yaml.SequenceNode {
		return nil, []string{"do: expected list"}
	}

	var actions []RawAction
	var errs []string

	for i, item := range node.Content {
		a, actErrs := parseAction(i, item)
		for _, e := range actErrs {
			errs = append(errs, fmt.Sprintf("do[%d]: %s", i, e))
		}
		if a != nil {
			actions = append(actions, *a)
		}
	}

	return actions, errs
}

// reservedActionKeys are keys that cannot be used as plugin names in shorthand form.
var reservedActionKeys = map[string]bool{
	"type":    true,
	"name":    true,
	"options": true,
}

func parseAction(index int, node *yaml.Node) (*RawAction, []string) {
	node = resolveNode(node)
	if node.Kind != yaml.MappingNode {
		return nil, []string{"expected mapping"}
	}

	raw := make(map[string]yaml.Node)
	for i := 0; i < len(node.Content)-1; i += 2 {
		raw[node.Content[i].Value] = *node.Content[i+1]
	}

	// Detect form: detailed (has "type" key) or shorthand (single key that is the plugin name)
	_, hasType := raw["type"]

	if hasType {
		return parseDetailedAction(raw)
	}

	return parseShorthandAction(raw)
}

func parseDetailedAction(raw map[string]yaml.Node) (*RawAction, []string) {
	a := &RawAction{}
	var errs []string

	typeNode := raw["type"]
	a.Type = typeNode.Value
	if a.Type == "" {
		errs = append(errs, "type must be a non-empty string")
	}

	// Name (optional label)
	if n, ok := raw["name"]; ok {
		a.Name = n.Value
	}

	if n, ok := raw["options"]; ok {
		var opts map[string]any
		if err := n.Decode(&opts); err != nil {
			errs = append(errs, fmt.Sprintf("options: %v", err))
		} else {
			a.Options = opts
		}
	}

	// Check for extra keys (could be shorthand mixed with detailed)
	for k := range raw {
		switch k {
		case "type", "name", "options":
			// known
		default:
			errs = append(errs, fmt.Sprintf("unknown key %q in detailed action (has both 'type' and other keys)", k))
		}
	}

	return a, errs
}

func parseShorthandAction(raw map[string]yaml.Node) (*RawAction, []string) {
	if len(raw) == 0 {
		return nil, []string{"empty action entry"}
	}
	if len(raw) > 1 {
		keys := make([]string, 0, len(raw))
		for k := range raw {
			keys = append(keys, k)
		}
		return nil, []string{fmt.Sprintf("shorthand action must have exactly one key, got: %s", strings.Join(keys, ", "))}
	}

	for k, v := range raw {
		if reservedActionKeys[k] {
			return nil, []string{fmt.Sprintf("%q is a reserved key and cannot be used as a plugin name", k)}
		}
		switch v.Kind {
		case yaml.ScalarNode:
			return &RawAction{
				Type:    k,
				Options: map[string]any{"_shorthand": v.Value},
			}, nil
		case yaml.MappingNode:
			var opts map[string]any
			if err := v.Decode(&opts); err != nil {
				return nil, []string{fmt.Sprintf("options: %v", err)}
			}
			return &RawAction{
				Type:    k,
				Options: opts,
			}, nil
		default:
			return nil, []string{fmt.Sprintf("shorthand action value must be a scalar or mapping, got %v", v.Kind)}
		}
	}

	return nil, nil // unreachable
}

// resolveNode follows YAML alias nodes to their target.
// Returns the original node if it's not an alias.
func resolveNode(node *yaml.Node) *yaml.Node {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	return node
}

var envVarPattern = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)(?:(:[-?])(.+?))?\}$`)

func parseSecrets(node *yaml.Node) ([]SecretDef, []string) {
	if node.Kind != yaml.MappingNode {
		return nil, []string{"expected mapping"}
	}

	var secrets []SecretDef
	var errs []string

	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		name := keyNode.Value
		if name == "" {
			errs = append(errs, "secret name must be non-empty")
			continue
		}

		def, err := parseSecretDef(name, valNode)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%q: %v", name, err))
			continue
		}
		secrets = append(secrets, *def)
	}

	return secrets, errs
}

func parseSecretDef(name string, node *yaml.Node) (*SecretDef, error) {
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("expected scalar value")
	}

	switch node.Tag {
	case "!file":
		ref := strings.TrimSpace(node.Value)
		if ref == "" {
			return nil, fmt.Errorf("!file requires a file path")
		}
		return &SecretDef{Name: name, Source: "file", Ref: ref}, nil

	case "!keyring":
		ref := strings.TrimSpace(node.Value)
		if ref == "" {
			return nil, fmt.Errorf("!keyring requires a service/key reference")
		}
		if !strings.Contains(ref, "/") {
			return nil, fmt.Errorf("!keyring value must be in service/key format, got %q", ref)
		}
		return &SecretDef{Name: name, Source: "keyring", Ref: ref}, nil

	default:
		// Env var: ${VAR}, ${VAR:-default}, ${VAR:?error}
		val := strings.TrimSpace(node.Value)
		m := envVarPattern.FindStringSubmatch(val)
		if m == nil {
			return nil, fmt.Errorf("unrecognized secret format %q — expected ${VAR}, ${VAR:-default}, ${VAR:?error}, !file path, or !keyring service/key", val)
		}

		def := &SecretDef{Name: name, Source: "env", Ref: m[1]}
		if m[2] == ":-" {
			def.Default = m[3]
		} else if m[2] == ":?" {
			def.Required = true
			def.ErrorMsg = m[3]
		}
		return def, nil
	}
}

func nodeKindName(kind yaml.Kind) string {
	switch kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return fmt.Sprintf("unknown(%d)", kind)
	}
}
