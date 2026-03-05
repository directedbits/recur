package displayterminal

import (
	"encoding/json"
	"fmt"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
)

// EntityStatus formats a proto EntityStatus enum as a lowercase word
// suitable for tabular display.
func EntityStatus(s recurv1.EntityStatus) string {
	switch s {
	case recurv1.EntityStatus_ENTITY_STATUS_ACTIVE:
		return "active"
	case recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED:
		return "suspended"
	case recurv1.EntityStatus_ENTITY_STATUS_ERROR:
		return "error"
	default:
		return "unknown"
	}
}

// StatusLabel returns a parenthesized suffix for non-active statuses, or an
// empty string when active. Used to annotate list rows.
func StatusLabel(s recurv1.EntityStatus) string {
	switch s {
	case recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED:
		return " (suspended)"
	case recurv1.EntityStatus_ENTITY_STATUS_ERROR:
		return " (error)"
	default:
		return ""
	}
}

// SafeID truncates an ID to at most 8 characters for compact display.
func SafeID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// JoinNames concatenates names with ", " separators.
func JoinNames(names []string) string {
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

// PrintJSON marshals v with two-space indent and writes it to stdout with a
// trailing newline. Returns the marshalling error if any.
func PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// InspectEntityResponse dispatches to the correct per-entity display function
// based on resp.EntityType. Returns an error for unknown types.
func InspectEntityResponse(resp *recurv1.InspectEntityResponse, jsonFlag, verbose bool) error {
	switch resp.EntityType {
	case "trigger":
		return Trigger(resp.Trigger, jsonFlag, verbose)
	case "action":
		return Action(resp.Action, jsonFlag, verbose)
	case "group":
		return Group(resp.Group, jsonFlag)
	case "recurfile":
		return Recurfile(resp.Recurfile, jsonFlag)
	case "plugin":
		return Plugin(resp.Plugin, jsonFlag, verbose)
	default:
		return fmt.Errorf("unknown entity type: %s", resp.EntityType)
	}
}

func Trigger(t *recurv1.TriggerDetail, jsonFlag, verbose bool) error {
	if jsonFlag {
		return PrintJSON(t)
	}

	fmt.Printf("ID:        %s\n", t.Id)
	fmt.Printf("Name:      %s\n", t.Name)
	fmt.Printf("Group:     %s\n", t.Group)
	fmt.Printf("Plugin:    %s\n", t.Plugin)
	fmt.Printf("Status:    %s\n", EntityStatus(t.Status))
	fmt.Printf("Recurfile: %s\n", t.Recurfile)
	if t.LastFired != nil {
		fmt.Printf("Fired:     %s\n", t.LastFired.AsTime().Local().Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Errors:    %d\n", t.ErrorCount)
	if len(t.Options) > 0 {
		fmt.Println("Options:")
		for _, o := range t.Options {
			fmt.Printf("  %s = %s\n", o.Name, o.Value)
		}
	}
	if len(t.Context) > 0 {
		fmt.Println("Context Variables:")
		for _, c := range t.Context {
			fmt.Printf("  %s (%s) — %s\n", c.Name, c.Type, c.Description)
		}
	}
	if len(t.ActionIds) > 0 {
		fmt.Println("Actions:")
		for _, id := range t.ActionIds {
			fmt.Printf("  %s\n", SafeID(id))
		}
	}
	if verbose && t.Plugin != "" {
		if ip := findInstalledPlugin("", "", t.Plugin); ip != nil {
			fmt.Printf("Plugin Dir:  %s\n", ip.Dir)
		}
	}
	return nil
}

func Action(a *recurv1.ActionDetail, jsonFlag, verbose bool) error {
	if jsonFlag {
		return PrintJSON(a)
	}

	fmt.Printf("ID:        %s\n", a.Id)
	fmt.Printf("Name:      %s\n", a.Name)
	fmt.Printf("Group:     %s\n", a.Group)
	fmt.Printf("Plugin:    %s\n", a.Plugin)
	fmt.Printf("Status:    %s\n", EntityStatus(a.Status))
	fmt.Printf("Recurfile: %s\n", a.Recurfile)
	if a.TriggerId != "" {
		fmt.Printf("Trigger:   %s\n", SafeID(a.TriggerId))
	}
	if a.LastExecuted != nil {
		fmt.Printf("Executed:  %s\n", a.LastExecuted.AsTime().Local().Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Errors:    %d\n", a.ErrorCount)
	if len(a.Options) > 0 {
		fmt.Println("Options:")
		for _, o := range a.Options {
			fmt.Printf("  %s = %s\n", o.Name, o.Value)
		}
	}
	if verbose && a.Plugin != "" {
		if ip := findInstalledPlugin("", "", a.Plugin); ip != nil {
			fmt.Printf("Plugin Dir:  %s\n", ip.Dir)
		}
	}
	return nil
}

func Group(g *recurv1.GroupDetail, jsonFlag bool) error {
	if jsonFlag {
		return PrintJSON(g)
	}

	fmt.Printf("ID:         %s\n", g.Id)
	fmt.Printf("Name:       %s\n", g.Name)
	fmt.Printf("Recurfiles: %s\n", JoinNames(g.Recurfiles))
	if len(g.Options) > 0 {
		fmt.Println("Options:")
		for _, o := range g.Options {
			fmt.Printf("  %s = %s\n", o.Name, o.Value)
		}
	}
	if len(g.Aliases) > 0 {
		fmt.Println("Aliases:")
		for k, v := range g.Aliases {
			fmt.Printf("  %s = %s\n", k, v)
		}
	}
	if len(g.Triggers) > 0 {
		fmt.Println("Triggers:")
		for _, t := range g.Triggers {
			fmt.Printf("  %s %s%s\n", SafeID(t.Id), t.Name, StatusLabel(t.Status))
		}
	}
	if len(g.Actions) > 0 {
		fmt.Println("Actions:")
		for _, a := range g.Actions {
			fmt.Printf("  %s %s%s\n", SafeID(a.Id), a.Name, StatusLabel(a.Status))
		}
	}
	return nil
}

func Plugin(p *recurv1.PluginDetail, jsonFlag, verbose bool) error {
	if jsonFlag {
		return PrintJSON(p)
	}

	fmt.Printf("ID:          %s\n", p.Id)
	fmt.Printf("Name:        %s\n", p.Name)
	fmt.Printf("Namespace:   %s\n", p.Namespace)
	fmt.Printf("Version:     %s\n", p.Version)
	fmt.Printf("Status:      %s\n", EntityStatus(p.Status))
	if p.Description != "" {
		fmt.Printf("Description: %s\n", p.Description)
	}
	if len(p.Dependencies) > 0 {
		fmt.Printf("Dependencies: %s\n", JoinNames(p.Dependencies))
	}
	if len(p.Triggers) > 0 {
		fmt.Println("Triggers:")
		for _, t := range p.Triggers {
			fmt.Printf("  %s\n", t.Name)
		}
	}
	if len(p.Actions) > 0 {
		fmt.Println("Actions:")
		for _, a := range p.Actions {
			fmt.Printf("  %s\n", a.Name)
		}
	}
	if len(p.Configuration) > 0 {
		fmt.Println("Configuration:")
		for _, c := range p.Configuration {
			fmt.Printf("  %s (%s)", c.Key, c.Type)
			if c.DefaultValue != "" && c.DefaultValue != "<nil>" {
				fmt.Printf(" default=%s", c.DefaultValue)
			}
			fmt.Println()
		}
	}
	if verbose {
		if ip := findInstalledPlugin(p.Id, p.Name, p.Namespace); ip != nil {
			fmt.Printf("Install Dir: %s\n", ip.Dir)
			fmt.Printf("Binary:      %s\n", ip.BinaryPath())
		}
	}
	return nil
}

func Recurfile(w *recurv1.RecurfileDetail, jsonFlag bool) error {
	if jsonFlag {
		return PrintJSON(w)
	}

	fmt.Printf("ID:   %s\n", w.Id)
	fmt.Printf("Path: %s\n", w.Path)
	if len(w.Groups) > 0 {
		fmt.Println("Groups:")
		for _, g := range w.Groups {
			fmt.Printf("  %s %s (triggers=%d, actions=%d)\n", SafeID(g.Id), g.Name, g.TriggerCount, g.ActionCount)
		}
	}
	if len(w.Triggers) > 0 {
		fmt.Println("Triggers:")
		for _, t := range w.Triggers {
			fmt.Printf("  %s %s [%s]%s\n", SafeID(t.Id), t.Name, t.Group, StatusLabel(t.Status))
		}
	}
	if len(w.Actions) > 0 {
		fmt.Println("Actions:")
		for _, a := range w.Actions {
			fmt.Printf("  %s %s [%s]%s\n", SafeID(a.Id), a.Name, a.Group, StatusLabel(a.Status))
		}
	}
	return nil
}

// findInstalledPlugin looks up a plugin on disk by ID, name, or namespace.
// Used for verbose display only.
func findInstalledPlugin(id, name, namespace string) *pluginfs.InstalledPlugin {
	plugins, _ := pluginfs.Discover()
	for _, identifier := range []string{id, name, namespace} {
		if identifier == "" {
			continue
		}
		if p := pluginfs.FindByIdentifier(plugins, identifier); p != nil {
			return p
		}
	}
	return nil
}
