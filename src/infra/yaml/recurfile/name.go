package recurfileyaml

import "strings"

// IsRecurfileName reports whether basename names a recurfile.
// Match is case-insensitive on the entire basename. Accepted forms:
//
//	recurfile             (no extension; YAML assumed)
//	recurfile.yaml
//	recurfile.yml
//
// Dot-prefix variants (.recurfile, .recurfile.yaml, .recurfile.yml) are
// not accepted.
func IsRecurfileName(basename string) bool {
	n := strings.ToLower(basename)
	return n == "recurfile" || n == "recurfile.yaml" || n == "recurfile.yml"
}
