package buildinfo

import "testing"

func TestVersionPrefersInjected(t *testing.T) {
	orig := injected
	t.Cleanup(func() { injected = orig })

	injected = "v1.2.3"
	if got := Version("fallback"); got != "v1.2.3" {
		t.Fatalf("Version() = %q, want injected %q", got, "v1.2.3")
	}
}

func TestVersionFallsBackWhenNoInjectedOrModuleVersion(t *testing.T) {
	orig := injected
	t.Cleanup(func() { injected = orig })

	// `go test` binaries report a Main.Version of "" or "(devel)", so with no
	// injected value Version must return the caller's fallback.
	injected = ""
	if got := Version("0.1.0-alpha"); got != "0.1.0-alpha" {
		t.Fatalf("Version() = %q, want fallback %q", got, "0.1.0-alpha")
	}
}
