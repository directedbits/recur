package recurfile

import (
	pkgconfig "github.com/directedbits/recur/pkg/config"
)

// Resolve mutates the parsed RawFile in place: merges aliases, expands alias
// prefixes in option keys and type/name fields, merges group+trigger options,
// and resolves which actions apply to each trigger. After this function, all
// names are fully qualified (aliases resolved), options are merged, and each
// trigger carries its resolved actions.
//
// No lock needed — operates only on the RawFile value.
func Resolve(f *RawFile) {
	for gi := range f.Groups {
		g := &f.Groups[gi]

		// Build merged alias map for this group (file + group, group wins)
		aliases := MergeAliases(f.Aliases, g.Aliases)

		// Expand alias prefixes in group option keys
		g.Options = ExpandOptionAliases(g.Options, aliases)

		// Resolve group-level action types
		for ai := range g.Actions {
			g.Actions[ai].Type = ExpandAlias(g.Actions[ai].Type, aliases)
		}

		for ti := range g.Triggers {
			t := &g.Triggers[ti]

			// Expand trigger option keys before merging
			t.Options = ExpandOptionAliases(t.Options, aliases)

			// Merge group options into trigger options (trigger takes precedence)
			t.Options = pkgconfig.OverlayMaps(g.Options, t.Options)

			// Resolve trigger type alias — t.Type becomes the qualified name
			t.Type = ExpandAlias(t.Type, aliases)

			// Resolve which actions apply to this trigger.
			// If trigger has no actions, copy group-level actions.
			if len(t.Actions) == 0 && len(g.Actions) > 0 {
				resolved := make([]RawAction, len(g.Actions))
				copy(resolved, g.Actions)
				t.Actions = resolved
			}

			// Resolve trigger-level action type aliases and merge group
			// options into each action's options (action takes precedence).
			// Mirrors the trigger inheritance path above.
			for ai := range t.Actions {
				t.Actions[ai].Type = ExpandAlias(t.Actions[ai].Type, aliases)
				t.Actions[ai].Options = pkgconfig.OverlayMaps(g.Options, t.Actions[ai].Options)
			}
		}
	}
}
