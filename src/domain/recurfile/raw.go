package recurfile

import "github.com/directedbits/recur/src/domain/secret"

// Raw* types are the structural representation of a parsed recurfile, before
// it is resolved (alias expansion, option merging, action inheritance) and
// before it is built into domain entities (Group/Trigger/Action with IDs,
// status, and runtime state).
//
// The `Raw` prefix disambiguates these from the entity types and signals that
// they are inert YAML-shaped data. infra/recurfile is the parser that emits
// them; domain/recurfile owns the resolution and entity-construction logic
// that consumes them.

// RawFile represents a parsed recurfile.
type RawFile struct {
	Path    string
	Aliases map[string]string
	Secrets []secret.SecretDef
	Groups  []RawGroup
}

// RawGroup represents a named trigger group within a recurfile.
type RawGroup struct {
	Name     string
	Aliases  map[string]string
	Options  map[string]any
	Triggers []RawTrigger
	Actions  []RawAction // group-level default actions (from "do")
}

// RawTrigger represents a trigger entry within a group.
type RawTrigger struct {
	Type    string
	Name    string // optional user-defined label
	Options map[string]any
	Actions []RawAction // trigger-level actions (from "do"), overrides group default
}

// RawAction represents an action entry, either detailed or shorthand form.
type RawAction struct {
	Type    string
	Name    string // optional user-defined label
	Options map[string]any
}
