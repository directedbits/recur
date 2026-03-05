package recurfile

import "fmt"

// Validate returns warnings for a resolved RawFile. Should be called after
// Resolve so inherited group actions are visible on each trigger.
func Validate(f *RawFile) []string {
	var warnings []string
	for _, g := range f.Groups {
		for _, t := range g.Triggers {
			if len(t.Actions) == 0 {
				warnings = append(warnings, fmt.Sprintf("group %q, trigger %q: no actions defined", g.Name, t.Type))
			}
		}
	}
	return warnings
}
