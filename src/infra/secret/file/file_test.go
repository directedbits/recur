package secretfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/directedbits/recur/src/domain/secret"
)

func TestResolve_Found(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(path, []byte("  hunter2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := New()
	got, err := r.Resolve(secret.SecretDef{Ref: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("got %q, want %q (whitespace should be trimmed)", got, "hunter2")
	}
}

func TestResolve_NotFound(t *testing.T) {
	r := New()
	_, err := r.Resolve(secret.SecretDef{Ref: "/does/not/exist"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
