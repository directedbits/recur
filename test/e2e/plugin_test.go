package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTestPluginDir creates a plugin directory with a minimal manifest.
func writeTestPluginDir(t *testing.T, name, namespace string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	os.MkdirAll(dir, 0755)
	manifest := `name: ` + name + `
namespace: ` + namespace + `
version: "0.1.0"
description: "Test plugin"

triggers:
  - name: TestTrigger
    description: "A test trigger"
    options:
      - name: interval
        type: string
        default: "5s"
        description: "Trigger interval"
    context:
      - name: Timestamp
        type: string
        description: "Event timestamp"
`
	os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0644)
	return dir
}

func TestInstallAndUninstallPlugin(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	pluginDir := writeTestPluginDir(t, "testplugin", "test.plugin")

	// Install
	out, stderr, code := runBin(t, home, watchBin, "install", pluginDir)
	if code != 0 {
		t.Fatalf("install exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}
	if !strings.Contains(out, "Installed") {
		t.Errorf("install output should contain 'Installed', got: %s", out)
	}
	if !strings.Contains(out, "test.plugin") {
		t.Errorf("install output should contain namespace, got: %s", out)
	}

	// Verify plugin directory exists on disk
	installedDir := filepath.Join(home, ".config", "recur", "plugins", "testplugin")
	if _, err := os.Stat(installedDir); err != nil {
		t.Fatalf("plugin not persisted to disk at %s: %v", installedDir, err)
	}
	if _, err := os.Stat(filepath.Join(installedDir, "manifest.yaml")); err != nil {
		t.Error("manifest.yaml not found in installed plugin directory")
	}

	// List plugins — should show the installed plugin
	out, _, code = runBin(t, home, watchBin, "list", "plugins")
	if code != 0 {
		t.Fatalf("list plugins exit code = %d", code)
	}
	if !strings.Contains(out, "testplugin") {
		t.Errorf("list plugins should contain 'testplugin', got: %s", out)
	}

	// Inspect plugin by namespace
	out, _, code = runBin(t, home, watchBin, "inspect", "plugin", "test.plugin")
	if code != 0 {
		t.Fatalf("inspect plugin exit code = %d", code)
	}
	if !strings.Contains(out, "test.plugin") {
		t.Errorf("inspect plugin should contain namespace, got: %s", out)
	}
	if !strings.Contains(out, "TestTrigger") {
		t.Errorf("inspect plugin should list triggers, got: %s", out)
	}

	// Uninstall by namespace
	out, stderr, code = runBin(t, home, watchBin, "uninstall", "test.plugin")
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}
	if !strings.Contains(out, "uninstalled") {
		t.Errorf("uninstall output should contain 'uninstalled', got: %s", out)
	}

	// Verify removed from disk
	if _, err := os.Stat(installedDir); !os.IsNotExist(err) {
		t.Error("plugin directory should be removed from disk after uninstall")
	}

	// List plugins — should be empty
	out, _, code = runBin(t, home, watchBin, "list", "plugins")
	if code != 0 {
		t.Fatalf("list plugins exit code = %d", code)
	}
	if strings.Contains(out, "testplugin") {
		t.Error("plugin should be removed after uninstall")
	}
}

func TestInstallPluginJSON(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	pluginDir := writeTestPluginDir(t, "jsonplugin", "test.jsonplugin")

	out, _, code := runBin(t, home, watchBin, "install", pluginDir, "--json")
	if code != 0 {
		t.Fatalf("install --json exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("install --json invalid JSON: %v\noutput: %s", err, out)
	}
	if result["namespace"] != "test.jsonplugin" {
		t.Errorf("namespace = %v, want test.jsonplugin", result["namespace"])
	}
}

func TestInstallDuplicatePlugin(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	pluginDir := writeTestPluginDir(t, "dupplugin", "test.duplicate")

	// First install should succeed
	_, _, code := runBin(t, home, watchBin, "install", pluginDir)
	if code != 0 {
		t.Fatalf("first install exit code = %d", code)
	}

	// Second install should fail (directory already exists)
	_, stderr, code := runBin(t, home, watchBin, "install", pluginDir)
	if code == 0 {
		t.Error("duplicate install should fail")
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("error should mention 'already exists', got: %s", stderr)
	}
}

func TestInstallInvalidPath(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	_, stderr, code := runBin(t, home, watchBin, "install", "/nonexistent/path")
	if code == 0 {
		t.Error("install with invalid path should fail")
	}
	if !strings.Contains(stderr, "install failed") {
		t.Errorf("error should mention failure, got: %s", stderr)
	}
}

func TestUninstallNotFound(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	_, stderr, code := runBin(t, home, watchBin, "uninstall", "nonexistent.plugin")
	if code == 0 {
		t.Error("uninstall nonexistent plugin should fail")
	}
	if !strings.Contains(stderr, "uninstall failed") {
		t.Errorf("error should mention failure, got: %s", stderr)
	}
}

func TestInstallPluginWithLink(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	pluginDir := writeTestPluginDir(t, "linkplugin", "test.linked")

	// Install with --link
	out, stderr, code := runBin(t, home, watchBin, "install", pluginDir, "--link")
	if code != 0 {
		t.Fatalf("install --link exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}
	if !strings.Contains(out, "linked") {
		t.Errorf("install --link output should contain 'linked', got: %s", out)
	}

	// Verify it's a symlink
	installedDir := filepath.Join(home, ".config", "recur", "plugins", "linkplugin")
	info, err := os.Lstat(installedDir)
	if err != nil {
		t.Fatalf("installed symlink not found: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular directory")
	}

	// Plugin should be functional
	out, _, code = runBin(t, home, watchBin, "list", "plugins")
	if code != 0 {
		t.Fatalf("list plugins exit code = %d", code)
	}
	if !strings.Contains(out, "linkplugin") {
		t.Errorf("list plugins should show linked plugin, got: %s", out)
	}

	// Uninstall should remove the symlink but not the source
	runBin(t, home, watchBin, "uninstall", "test.linked")

	if _, err := os.Lstat(installedDir); !os.IsNotExist(err) {
		t.Error("symlink should be removed after uninstall")
	}
	if _, err := os.Stat(pluginDir); err != nil {
		t.Error("source directory should still exist after uninstall")
	}
}

func TestInstallPluginSurvivesRestart(t *testing.T) {
	home := t.TempDir()

	// Start daemon
	_, stderr, code := runBin(t, home, recurBinary, "start")
	if code != 0 {
		t.Fatalf("start exit code = %d, stderr: %s", code, stderr)
	}
	time.Sleep(300 * time.Millisecond)

	// Install a plugin
	pluginDir := writeTestPluginDir(t, "persistplugin", "test.persist")
	out, stderr, code := runBin(t, home, recurBinary, "install", pluginDir)
	if code != 0 {
		t.Fatalf("install exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}

	// Stop daemon
	runBin(t, home, recurBinary, "stop")
	time.Sleep(300 * time.Millisecond)

	// Start daemon again
	_, stderr, code = runBin(t, home, recurBinary, "start")
	if code != 0 {
		t.Fatalf("restart exit code = %d, stderr: %s", code, stderr)
	}
	time.Sleep(300 * time.Millisecond)
	defer func() {
		runBin(t, home, recurBinary, "stop")
		time.Sleep(200 * time.Millisecond)
	}()

	// Plugin should be discovered on restart
	out, _, code = runBin(t, home, recurBinary, "list", "plugins")
	if code != 0 {
		t.Fatalf("list plugins exit code = %d", code)
	}
	if !strings.Contains(out, "persistplugin") {
		t.Errorf("plugin should survive daemon restart, got: %s", out)
	}
}
