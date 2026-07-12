// Package buildinfo resolves the running binary's version across build methods.
//
// Release builds inject the tag via the linker
// (-ldflags "-X github.com/directedbits/recur/src/infra/buildinfo.injected=vX.Y.Z").
// Binaries produced by `go install <module>@vX.Y.Z` carry no ldflags, so the
// version is read from the module build info instead. Local `go build` produces
// neither, so a caller-supplied fallback is used.
package buildinfo

import "runtime/debug"

// injected is set at release build time via -ldflags -X. Leave it empty here;
// the linker overwrites it. Do not rename without updating the release workflow.
var injected string

// Version resolves the build version, preferring the release-injected value,
// then the main module version recorded by `go install`, then fallback.
func Version(fallback string) string {
	if injected != "" {
		return injected
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		// Main.Version is the module version for `go install <mod>@vX.Y.Z`
		// builds, and "(devel)" (or empty) for local `go build`.
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return fallback
}
