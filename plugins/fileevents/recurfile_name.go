package main

import "strings"

// Mirror of src/infra/recurfile/name.go:IsRecurfileName — keep in sync.
// Recurfile naming is stable; if it changes, update both locations.
//
// Plugins are treated as third-party code and do not import from the main
// module's internal packages, so this small helper is duplicated rather
// than imported.
func isRecurfileName(basename string) bool {
	n := strings.ToLower(basename)
	return n == "recurfile" || n == "recurfile.yaml" || n == "recurfile.yml"
}
