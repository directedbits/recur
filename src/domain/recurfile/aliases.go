package recurfile

import "strings"

// MergeAliases merges file-level and group-level aliases. Group aliases
// override file aliases when both define the same key.
func MergeAliases(fileAliases, groupAliases map[string]string) map[string]string {
	if len(fileAliases) == 0 && len(groupAliases) == 0 {
		return nil
	}
	result := make(map[string]string, len(fileAliases)+len(groupAliases))
	for k, v := range fileAliases {
		result[k] = v
	}
	for k, v := range groupAliases {
		result[k] = v
	}
	return result
}

// ExpandAlias expands the prefix before the first "." in name if it matches an
// alias key. If there is no dot, the entire name is checked against the alias
// map. Returns the original name when no alias applies.
func ExpandAlias(name string, aliases map[string]string) string {
	if len(aliases) == 0 {
		return name
	}

	dotIdx := strings.IndexByte(name, '.')
	if dotIdx < 0 {
		if expanded, ok := aliases[name]; ok {
			return expanded
		}
		return name
	}

	prefix := name[:dotIdx]
	if expanded, ok := aliases[prefix]; ok {
		return expanded + name[dotIdx:]
	}
	return name
}

// ExpandOptionAliases returns a new map with alias prefixes expanded in option
// keys. Values are copied as-is.
func ExpandOptionAliases(opts map[string]any, aliases map[string]string) map[string]any {
	if len(opts) == 0 || len(aliases) == 0 {
		return opts
	}
	result := make(map[string]any, len(opts))
	for k, v := range opts {
		result[ExpandAlias(k, aliases)] = v
	}
	return result
}
