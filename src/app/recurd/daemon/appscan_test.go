package daemon

import (
	"os"
	"path/filepath"
	"testing"

	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
)

const scanTestRecurfile = `Habit:
  on:
    - type: FileModified
      options:
        path: "/tmp"
  do:
    - shell: "echo hi"
`

// installApp writes a recurfile named rfName under ~/.config/recur/app/<name>/.
func installApp(t *testing.T, home, name, rfName string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "recur", "app", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, rfName), []byte(scanTestRecurfile), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanAppFolderRegistersInstalledApps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	installApp(t, home, "habits", "habits.yaml")

	d := &Daemon{registry: newRegistry(), config: configyaml.DefaultConfig()}
	d.scanAppFolder()

	if len(d.registry.recurfiles) != 1 {
		t.Fatalf("expected 1 registered recurfile after scan, got %d", len(d.registry.recurfiles))
	}

	// Idempotent: an app already registered is not registered again.
	d.scanAppFolder()
	if len(d.registry.recurfiles) != 1 {
		t.Errorf("scan not idempotent: got %d recurfiles", len(d.registry.recurfiles))
	}
}

func TestScanAppFolderSkipsNonAppDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// A directory with no root recurfile is not an app.
	junk := filepath.Join(home, ".config", "recur", "app", "junk")
	if err := os.MkdirAll(filepath.Join(junk, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{registry: newRegistry(), config: configyaml.DefaultConfig()}
	d.scanAppFolder()

	if len(d.registry.recurfiles) != 0 {
		t.Errorf("expected no registrations, got %d", len(d.registry.recurfiles))
	}
}

func TestScanAppFolderNoAppDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// AppDir() creates the dir but leaves it empty; scan must be a no-op.
	d := &Daemon{registry: newRegistry(), config: configyaml.DefaultConfig()}
	d.scanAppFolder()

	if len(d.registry.recurfiles) != 0 {
		t.Errorf("expected no registrations for empty app dir, got %d", len(d.registry.recurfiles))
	}
}
