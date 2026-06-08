package recurfile

import (
	"fmt"
	"strings"
)

// Merge produces the YAML text resulting from appending or merging a fragment
// recurfile into existing recurfile text.
//
//   - If existing is empty or whitespace-only, the fragment becomes the file.
//   - If the fragment introduces a group whose name already exists in existing,
//     the new triggers and actions are merged into the existing group's YAML in
//     place. Comments and indentation in the existing text are preserved.
//   - Otherwise, the fragment is appended verbatim with a blank-line separator.
//
// Both inputs must be valid recurfile YAML; parsing errors are returned with
// context.
func Merge(existing, fragment []byte) ([]byte, error) {
	existingStr := string(existing)
	if strings.TrimSpace(existingStr) == "" {
		return fragment, nil
	}

	existingFile, err := parseForMerge(existing)
	if err != nil {
		return nil, fmt.Errorf("existing recurfile is invalid: %w", err)
	}

	fragmentFile, err := parseForMerge(fragment)
	if err != nil {
		return nil, fmt.Errorf("fragment is invalid: %w", err)
	}

	existingGroups := make(map[string]bool, len(existingFile.Groups))
	for _, g := range existingFile.Groups {
		existingGroups[g.Name] = true
	}

	for _, fg := range fragmentFile.Groups {
		if existingGroups[fg.Name] {
			merged, err := MergeGroupIntoYAML(existingStr, fg)
			if err != nil {
				return nil, err
			}
			return []byte(merged), nil
		}
	}

	if !strings.HasSuffix(existingStr, "\n") {
		existingStr += "\n"
	}
	return []byte(existingStr + "\n" + string(fragment)), nil
}

// parseForMerge is a hook the infra parser sets at init so the domain merge
// logic can validate inputs without importing infra/recurfile.
var parseForMerge func([]byte) (*RawFile, error)

// RegisterParser is called by the infra parser package to register its Parse
// function with the domain merge logic. Calling more than once overwrites
// the previous registration (the last package wins; in practice only one
// parser exists).
func RegisterParser(parse func([]byte) (*RawFile, error)) {
	parseForMerge = parse
}

// MergeGroupIntoYAML inserts new triggers and actions from fg into the
// existing YAML text for the group with the same name. Operates on the raw
// YAML text to preserve comments and formatting that a roundtrip through
// the YAML parser would lose.
func MergeGroupIntoYAML(existingYAML string, fg RawGroup) (string, error) {
	lines := strings.Split(existingYAML, "\n")
	groupHeader := fg.Name + ":"

	groupIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == groupHeader {
			groupIdx = i
			break
		}
	}
	if groupIdx == -1 {
		if !strings.HasSuffix(existingYAML, "\n") {
			existingYAML += "\n"
		}
		stub := RenderGroupContent(fg)
		return existingYAML + "\n" + fg.Name + ":\n" + stub, nil
	}

	// Find the end of this group (next top-level key or EOF).
	groupEnd := len(lines)
	for i := groupIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
			groupEnd = i
			break
		}
	}

	var insertLines []string

	if len(fg.Triggers) > 0 {
		hasTriggersSection := false
		triggersEnd := -1
		for i := groupIdx + 1; i < groupEnd; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "on:" {
				hasTriggersSection = true
			}
			if hasTriggersSection && trimmed == "do:" {
				triggersEnd = i
				break
			}
		}
		if !hasTriggersSection {
			triggersEnd = groupEnd
		}

		for _, t := range fg.Triggers {
			insertLines = append(insertLines, "    - type: "+t.Type)
			if len(t.Options) > 0 {
				insertLines = append(insertLines, "      options:")
				for k, v := range t.Options {
					insertLines = append(insertLines, fmt.Sprintf("        %s: %q", k, fmt.Sprint(v)))
				}
			}
		}

		if hasTriggersSection && triggersEnd > 0 {
			before := lines[:triggersEnd]
			after := lines[triggersEnd:]
			lines = append(append(before, insertLines...), after...)
			groupEnd += len(insertLines)
		} else if hasTriggersSection {
			before := lines[:groupEnd]
			after := lines[groupEnd:]
			lines = append(append(before, insertLines...), after...)
			groupEnd += len(insertLines)
		} else {
			onLines := append([]string{"  on:"}, insertLines...)
			before := lines[:groupIdx+1]
			after := lines[groupIdx+1:]
			lines = append(append(before, onLines...), after...)
			groupEnd += len(onLines)
		}
	}

	if len(fg.Actions) > 0 {
		hasDoSection := false
		for i := groupIdx + 1; i < groupEnd; i++ {
			if strings.TrimSpace(lines[i]) == "do:" {
				hasDoSection = true
				break
			}
		}

		var actionLines []string
		for _, a := range fg.Actions {
			if sh, ok := a.Options["_shorthand"]; ok {
				actionLines = append(actionLines, fmt.Sprintf("    - %s: %q", a.Type, fmt.Sprint(sh)))
			} else {
				actionLines = append(actionLines, "    - type: "+a.Type)
				if len(a.Options) > 0 {
					actionLines = append(actionLines, "      options:")
					for k, v := range a.Options {
						actionLines = append(actionLines, fmt.Sprintf("        %s: %q", k, fmt.Sprint(v)))
					}
				}
			}
		}

		if hasDoSection {
			doEnd := groupEnd
			for i := groupIdx + 1; i < groupEnd; i++ {
				if strings.TrimSpace(lines[i]) == "do:" {
					for j := i + 1; j < groupEnd; j++ {
						trimmed := strings.TrimSpace(lines[j])
						if trimmed != "" && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, " ") {
							doEnd = j
							break
						}
					}
					break
				}
			}
			before := lines[:doEnd]
			after := lines[doEnd:]
			lines = append(append(before, actionLines...), after...)
		} else {
			doLines := append([]string{"  do:"}, actionLines...)
			before := lines[:groupEnd]
			after := lines[groupEnd:]
			lines = append(append(before, doLines...), after...)
		}
	}

	if len(fg.Triggers) == 0 && len(fg.Actions) == 0 {
		return existingYAML, nil
	}

	return strings.Join(lines, "\n"), nil
}

// RenderGroupContent renders the inner content (triggers + do) of a group as
// minimal YAML, suitable for use as a stub when a brand-new group is being
// appended to a recurfile.
func RenderGroupContent(g RawGroup) string {
	var b strings.Builder
	if len(g.Triggers) > 0 {
		b.WriteString("  on:\n")
		for _, t := range g.Triggers {
			b.WriteString("    - type: " + t.Type + "\n")
		}
	}
	if len(g.Actions) > 0 {
		b.WriteString("  do:\n")
		for _, a := range g.Actions {
			b.WriteString("    - type: " + a.Type + "\n")
		}
	}
	return b.String()
}
