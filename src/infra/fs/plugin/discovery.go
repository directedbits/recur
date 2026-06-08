// Package plugin handles plugin discovery and lifecycle management.
package pluginfs

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	domainplugin "github.com/directedbits/recur/src/domain/plugin"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
)

// InstalledPlugin represents a discovered plugin on disk with its parsed manifest.
type InstalledPlugin struct {
	ID       string // deterministic hash from namespace
	Dir      string // directory path
	Manifest *manifestyaml.Manifest
}

// pluginsDirFunc is the function used to resolve the plugins directory.
// Overridable in tests.
var pluginsDirFunc = defaultPluginsDir

// PluginsDir returns the recur plugins directory (~/.config/recur/plugins/), creating it if needed.
func PluginsDir() (string, error) {
	return pluginsDirFunc()
}

func defaultPluginsDir() (string, error) {
	configDir, err := configyaml.ConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "plugins")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("could not create plugins directory: %w", err)
	}
	return dir, nil
}

// Discover scans the plugins directory and returns all valid installed plugins.
// Invalid plugins (missing/broken manifests) are skipped with errors collected.
func Discover() ([]*InstalledPlugin, []error) {
	dir, err := PluginsDir()
	if err != nil {
		return nil, []error{err}
	}
	return DiscoverIn(dir)
}

// DiscoverIn scans the given directory for plugin subdirectories.
func DiscoverIn(dir string) ([]*InstalledPlugin, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("could not read plugins directory: %w", err)}
	}

	var plugins []*InstalledPlugin
	var errs []error

	for _, entry := range entries {
		// Accept both directories and symlinks to directories
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		pluginDir := filepath.Join(dir, entry.Name())
		p, err := LoadPlugin(pluginDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("plugin %q: %w", entry.Name(), err))
			continue
		}
		plugins = append(plugins, p)
	}

	// Sort by name for consistent ordering
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Manifest.Name < plugins[j].Manifest.Name
	})

	return plugins, errs
}

// LoadPlugin loads a single plugin from a directory containing manifest.yaml.
func LoadPlugin(dir string) (*InstalledPlugin, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	m, err := manifestyaml.Load(manifestPath)
	if err != nil {
		return nil, err
	}

	return &InstalledPlugin{
		ID:       generateID(m.Namespace),
		Dir:      dir,
		Manifest: m,
	}, nil
}

// generateID creates a deterministic short ID from the plugin namespace.
func generateID(namespace string) string {
	h := sha256.Sum256([]byte(namespace))
	return fmt.Sprintf("%x", h[:4]) // 8-char hex
}

// BinaryPath returns the path to the plugin's executable binary.
// By convention, the binary name matches the manifest name field.
// On Windows, the .exe extension is appended automatically. On other
// platforms, if the expected binary is not found, a .exe variant is
// tried as a fallback (e.g. when running Windows-built plugins in WSL).
func (plugin *InstalledPlugin) BinaryPath() string {
	paths := []string{}

	if runtime.GOOS != "windows" {
		paths = append(paths, filepath.Join(plugin.Dir, plugin.Manifest.Name))
	}

	paths = append(paths, filepath.Join(plugin.Dir, plugin.Manifest.Name+".exe"))

	// On non-Windows platforms, fall back to name.exe if the plain name
	// doesn't exist on disk. This handles cross-platform scenarios such
	// as running Windows-built plugin binaries under WSL.
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return paths[0]
}

// FindTriggerDefinition looks up a trigger definition by name in this plugin's manifest.
func (plugin *InstalledPlugin) FindTriggerDefinition(triggerType string) *manifestyaml.TriggerDef {
	for i := range plugin.Manifest.Triggers {
		if strings.EqualFold(plugin.Manifest.Triggers[i].Name, triggerType) {
			return &plugin.Manifest.Triggers[i]
		}
	}
	return nil
}

// FindActionDefinition looks up an action definition by name in this plugin's manifest.
func (plugin *InstalledPlugin) FindActionDefinition(actionName string) *manifestyaml.ActionDef {
	for i := range plugin.Manifest.Actions {
		if strings.EqualFold(plugin.Manifest.Actions[i].Name, actionName) {
			return &plugin.Manifest.Actions[i]
		}
	}
	return nil
}

// FindShorthandOption returns the name of the shorthand option for an action
// in this plugin's manifest. If no option is marked shorthand: true, falls back
// to the first option. The second return value is true if this was a fallback
// (no explicit shorthand declared). Returns empty string if no options exist.
func (plugin *InstalledPlugin) FindShorthandOption(actionName string) (string, bool) {
	def := plugin.FindActionDefinition(actionName)
	if def == nil {
		return "", false
	}
	for _, opt := range def.Options {
		if opt.Shorthand {
			return opt.Name, false
		}
	}
	// Fallback: first option
	if len(def.Options) > 0 {
		return def.Options[0].Name, true
	}
	return "", false
}

// Domain returns the domain projection of this installed plugin: metadata
// fields plus the manifest's triggers and actions mapped into
// domain/plugin.TriggerType / ActionType. Callers pass the result into
// domain-level helpers (UnknownTypes, TriggerDefaultsFor, …) so domain
// code does not need to know about manifestyaml.Manifest.
func (plugin *InstalledPlugin) Domain() *domainplugin.Plugin {
	if plugin == nil || plugin.Manifest == nil {
		return nil
	}
	out := &domainplugin.Plugin{
		ID:          plugin.ID,
		Name:        plugin.Manifest.Name,
		Namespace:   plugin.Manifest.Namespace,
		Version:     plugin.Manifest.Version,
		Description: plugin.Manifest.Description,
	}
	for _, t := range plugin.Manifest.Triggers {
		out.Triggers = append(out.Triggers, domainplugin.TriggerType{
			Name:     t.Name,
			Defaults: t.Defaults,
		})
	}
	for _, a := range plugin.Manifest.Actions {
		out.Actions = append(out.Actions, domainplugin.ActionType{Name: a.Name})
	}
	return out
}

// DomainAll converts a slice of InstalledPlugin into the domain projection.
func DomainAll(plugins []*InstalledPlugin) []*domainplugin.Plugin {
	if len(plugins) == 0 {
		return nil
	}
	out := make([]*domainplugin.Plugin, 0, len(plugins))
	for _, p := range plugins {
		if d := p.Domain(); d != nil {
			out = append(out, d)
		}
	}
	return out
}

// ResolvePluginForTrigger returns the plugin namespace that declares the given
// trigger type, or empty string if no installed plugin matches.
func ResolvePluginForTrigger(plugins []*InstalledPlugin, triggerType string) string {
	for _, p := range plugins {
		if p.FindTriggerDefinition(triggerType) != nil {
			return p.Manifest.Namespace
		}
	}
	return ""
}

// ResolvePluginForAction returns the plugin namespace that declares the given
// action name, or empty string if no installed plugin matches (e.g. built-in "Shell").
func ResolvePluginForAction(plugins []*InstalledPlugin, actionName string) string {
	for _, p := range plugins {
		if p.FindActionDefinition(actionName) != nil {
			return p.Manifest.Namespace
		}
	}
	return ""
}

// CheckConflicts detects trigger/action name collisions across installed plugins.
// Returns warnings for any names claimed by multiple plugins.
func CheckConflicts(plugins []*InstalledPlugin) []string {
	triggerOwners := make(map[string][]string) // trigger name → []namespace
	actionOwners := make(map[string][]string)  // action name → []namespace

	for _, p := range plugins {
		for _, t := range p.Manifest.Triggers {
			triggerOwners[t.Name] = append(triggerOwners[t.Name], p.Manifest.Namespace)
		}
		for _, a := range p.Manifest.Actions {
			actionOwners[a.Name] = append(actionOwners[a.Name], p.Manifest.Namespace)
		}
	}

	var warnings []string
	for name, owners := range triggerOwners {
		if len(owners) > 1 {
			warnings = append(warnings, fmt.Sprintf("trigger %q declared by multiple plugins: %s", name, strings.Join(owners, ", ")))
		}
	}
	for name, owners := range actionOwners {
		if len(owners) > 1 {
			warnings = append(warnings, fmt.Sprintf("action %q declared by multiple plugins: %s", name, strings.Join(owners, ", ")))
		}
	}
	return warnings
}

// FindByIdentifier searches a list of plugins by ID prefix, name, or namespace.
func FindByIdentifier(plugins []*InstalledPlugin, identifier string) *InstalledPlugin {
	// Exact match on ID, name, or namespace
	for _, p := range plugins {
		if p.ID == identifier || strings.EqualFold(p.Manifest.Name, identifier) || strings.EqualFold(p.Manifest.Namespace, identifier) {
			return p
		}
	}
	// ID prefix match
	for _, p := range plugins {
		if len(identifier) >= 3 && len(p.ID) >= len(identifier) && p.ID[:len(identifier)] == identifier {
			return p
		}
	}
	return nil
}
