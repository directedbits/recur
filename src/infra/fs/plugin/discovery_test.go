package pluginfs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const validManifest = `
name: filesystem
namespace: com.example.filesystem
version: "1.0.0"
description: "File system event triggers"
triggers:
  - name: FileCreated
    options:
      - name: recursive
        type: bool
        default: false
    context:
      - name: FilePath
        type: string
actions:
  - name: Shell
    options:
      - name: command
        type: string
        shorthand: true
`

const validManifest2 = `
name: notify
namespace: com.example.notify
version: "0.1.0"
actions:
  - name: Notify
    options:
      - name: message
        type: string
        shorthand: true
`

func setupPluginDir(t *testing.T, plugins map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, manifest := range plugins {
		pluginDir := filepath.Join(dir, name)
		os.MkdirAll(pluginDir, 0755)
		os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(manifest), 0644)
	}
	return dir
}

func TestDiscoverInEmpty(t *testing.T) {
	dir := t.TempDir()
	plugins, errs := DiscoverIn(dir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestDiscoverInNonexistent(t *testing.T) {
	plugins, errs := DiscoverIn("/nonexistent/path")
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for nonexistent dir, got %d", len(errs))
	}
}

func TestDiscoverInSinglePlugin(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
	})

	plugins, errs := DiscoverIn(dir)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Manifest.Name != "filesystem" {
		t.Errorf("name = %q, want %q", plugins[0].Manifest.Name, "filesystem")
	}
	if plugins[0].Manifest.Namespace != "com.example.filesystem" {
		t.Errorf("namespace = %q, want %q", plugins[0].Manifest.Namespace, "com.example.filesystem")
	}
	if plugins[0].ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestDiscoverInMultiplePlugins(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
		"notify":     validManifest2,
	})

	plugins, errs := DiscoverIn(dir)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	// Should be sorted by name
	if plugins[0].Manifest.Name != "filesystem" {
		t.Errorf("first plugin = %q, want %q", plugins[0].Manifest.Name, "filesystem")
	}
	if plugins[1].Manifest.Name != "notify" {
		t.Errorf("second plugin = %q, want %q", plugins[1].Manifest.Name, "notify")
	}
}

func TestDiscoverInSkipsInvalid(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"good":    validManifest,
		"bad":     ":::invalid yaml",
		"nofield": "name: x\nversion: '1.0'\n", // missing namespace and triggers/actions
	})

	plugins, errs := DiscoverIn(dir)
	if len(plugins) != 1 {
		t.Errorf("expected 1 valid plugin, got %d", len(plugins))
	}
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestDiscoverInSkipsFiles(t *testing.T) {
	dir := t.TempDir()
	// Create a file (not a directory) in the plugins dir
	os.WriteFile(filepath.Join(dir, "not-a-plugin.txt"), []byte("hello"), 0644)
	// Create a valid plugin dir
	pluginDir := filepath.Join(dir, "filesystem")
	os.MkdirAll(pluginDir, 0755)
	os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(validManifest), 0644)

	plugins, errs := DiscoverIn(dir)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
}

func TestLoadPlugin(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
	})

	p, err := LoadPlugin(filepath.Join(dir, "filesystem"))
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}
	if p.Manifest.Name != "filesystem" {
		t.Errorf("name = %q, want %q", p.Manifest.Name, "filesystem")
	}
	if p.Dir != filepath.Join(dir, "filesystem") {
		t.Errorf("dir = %q, want %q", p.Dir, filepath.Join(dir, "filesystem"))
	}
}

func TestLoadPluginMissingManifest(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "empty")
	os.MkdirAll(pluginDir, 0755)

	_, err := LoadPlugin(pluginDir)
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestFindByIdentifier(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
		"notify":     validManifest2,
	})
	plugins, _ := DiscoverIn(dir)

	// By name
	p := FindByIdentifier(plugins, "filesystem")
	if p == nil || p.Manifest.Name != "filesystem" {
		t.Error("FindByIdentifier by name failed")
	}

	// By namespace
	p = FindByIdentifier(plugins, "com.example.notify")
	if p == nil || p.Manifest.Name != "notify" {
		t.Error("FindByIdentifier by namespace failed")
	}

	// By ID
	p = FindByIdentifier(plugins, plugins[0].ID)
	if p == nil {
		t.Error("FindByIdentifier by full ID failed")
	}

	// By ID prefix (at least 3 chars)
	if len(plugins[0].ID) >= 3 {
		p = FindByIdentifier(plugins, plugins[0].ID[:3])
		if p == nil {
			t.Error("FindByIdentifier by ID prefix failed")
		}
	}

	// Not found
	p = FindByIdentifier(plugins, "nonexistent")
	if p != nil {
		t.Error("expected nil for nonexistent identifier")
	}
}

func TestFindTriggerDef(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{"filesystem": validManifest})
	p, _ := LoadPlugin(filepath.Join(dir, "filesystem"))

	def := p.FindTriggerDefinition("FileCreated")
	if def == nil {
		t.Fatal("expected to find FileCreated trigger def")
	}
	if def.Name != "FileCreated" {
		t.Errorf("name = %q, want %q", def.Name, "FileCreated")
	}

	// Case insensitive
	if p.FindTriggerDefinition("filecreated") == nil {
		t.Error("expected case-insensitive match")
	}

	if p.FindTriggerDefinition("NonExistent") != nil {
		t.Error("expected nil for non-existent trigger")
	}
}

func TestFindActionDef(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{"filesystem": validManifest})
	p, _ := LoadPlugin(filepath.Join(dir, "filesystem"))

	def := p.FindActionDefinition("Shell")
	if def == nil {
		t.Fatal("expected to find Shell action def")
	}
	if def.Name != "Shell" {
		t.Errorf("name = %q, want %q", def.Name, "Shell")
	}

	if p.FindActionDefinition("NonExistent") != nil {
		t.Error("expected nil for non-existent action")
	}
}

func TestFindShorthandOption(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
		"notify":     validManifest2,
	})

	p1, _ := LoadPlugin(filepath.Join(dir, "filesystem"))
	got, isFallback := p1.FindShorthandOption("Shell")
	if got != "command" {
		t.Errorf("Shell shorthand = %q, want %q", got, "command")
	}
	if isFallback {
		t.Error("Shell shorthand should not be a fallback")
	}

	p2, _ := LoadPlugin(filepath.Join(dir, "notify"))
	got, isFallback = p2.FindShorthandOption("Notify")
	if got != "message" {
		t.Errorf("Notify shorthand = %q, want %q", got, "message")
	}
	if isFallback {
		t.Error("Notify shorthand should not be a fallback")
	}

	// No shorthand for non-existent action
	got, _ = p1.FindShorthandOption("NonExistent")
	if got != "" {
		t.Errorf("expected empty for non-existent action, got %q", got)
	}
}

const noShorthandManifest = `
name: noshorthand
namespace: com.example.noshorthand
version: "0.1.0"
actions:
  - name: DoStuff
    options:
      - name: first_opt
        type: string
      - name: second_opt
        type: string
`

func TestFindShorthandOption_FallbackToFirst(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"noshorthand": noShorthandManifest,
	})

	p, _ := LoadPlugin(filepath.Join(dir, "noshorthand"))
	got, isFallback := p.FindShorthandOption("DoStuff")
	if got != "first_opt" {
		t.Errorf("expected fallback to first option 'first_opt', got %q", got)
	}
	if !isFallback {
		t.Error("expected isFallback=true when no shorthand declared")
	}
}

func TestResolvePluginForTrigger(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
		"notify":     validManifest2,
	})
	plugins, _ := DiscoverIn(dir)

	ns := ResolvePluginForTrigger(plugins, "FileCreated")
	if ns != "com.example.filesystem" {
		t.Errorf("got %q, want %q", ns, "com.example.filesystem")
	}

	ns = ResolvePluginForTrigger(plugins, "NonExistent")
	if ns != "" {
		t.Errorf("expected empty for non-existent trigger, got %q", ns)
	}
}

func TestResolvePluginForAction(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
		"notify":     validManifest2,
	})
	plugins, _ := DiscoverIn(dir)

	ns := ResolvePluginForAction(plugins, "Notify")
	if ns != "com.example.notify" {
		t.Errorf("got %q, want %q", ns, "com.example.notify")
	}

	ns = ResolvePluginForAction(plugins, "NonExistent")
	if ns != "" {
		t.Errorf("expected empty for non-existent action, got %q", ns)
	}
}

func TestCheckConflicts_NoConflicts(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
		"notify":     validManifest2,
	})
	plugins, _ := DiscoverIn(dir)
	warnings := CheckConflicts(plugins)
	if len(warnings) != 0 {
		t.Errorf("expected no conflicts, got: %v", warnings)
	}
}

const conflictManifest = `
name: altfilesystem
namespace: com.example.altfilesystem
version: "0.1.0"
triggers:
  - name: FileCreated
    options:
      - name: path
        type: string
actions:
  - name: Shell
    options:
      - name: command
        type: string
        shorthand: true
`

func TestCheckConflicts_Detects(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem":    validManifest,
		"altfilesystem": conflictManifest,
	})
	plugins, _ := DiscoverIn(dir)
	warnings := CheckConflicts(plugins)
	if len(warnings) == 0 {
		t.Fatal("expected conflict warnings, got none")
	}

	foundTrigger := false
	foundAction := false
	for _, w := range warnings {
		if strings.Contains(w, "trigger") && strings.Contains(w, "FileCreated") {
			foundTrigger = true
		}
		if strings.Contains(w, "action") && strings.Contains(w, "Shell") {
			foundAction = true
		}
	}
	if !foundTrigger {
		t.Error("expected trigger conflict warning for FileCreated")
	}
	if !foundAction {
		t.Error("expected action conflict warning for Shell")
	}
}

func TestBinaryPath(t *testing.T) {
	dir := setupPluginDir(t, map[string]string{
		"filesystem": validManifest,
	})
	plugins, _ := DiscoverIn(dir)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	binPath := plugins[0].BinaryPath()
	if !strings.HasSuffix(binPath, "filesystem") && !strings.HasSuffix(binPath, "filesystem.exe") {
		t.Errorf("BinaryPath = %q, want suffix 'filesystem' or 'filesystem.exe'", binPath)
	}
	if !filepath.IsAbs(binPath) {
		t.Errorf("BinaryPath should be absolute, got %q", binPath)
	}
}

func TestBinaryPathExeFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exe fallback only applies on non-Windows")
	}

	dir := setupPluginDir(t, map[string]string{
		"timer": `
name: timer
namespace: core.timer
version: "0.1.0"
triggers:
  - name: Cron
    options:
      - name: expression
        type: string
`,
	})

	plugins, _ := DiscoverIn(dir)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	// Create only a .exe binary (simulating Windows-built plugin in WSL)
	exePath := filepath.Join(dir, "timer", "timer.exe")
	if err := os.WriteFile(exePath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	binPath := plugins[0].BinaryPath()
	if binPath != exePath {
		t.Errorf("BinaryPath = %q, want %q (exe fallback)", binPath, exePath)
	}
}

func TestBinaryPathPrefersPlatformNative(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test is for non-Windows platforms")
	}

	dir := setupPluginDir(t, map[string]string{
		"timer": `
name: timer
namespace: core.timer
version: "0.1.0"
triggers:
  - name: Cron
    options:
      - name: expression
        type: string
`,
	})

	// Create both the plain binary and the .exe variant
	plainPath := filepath.Join(dir, "timer", "timer")
	exePath := filepath.Join(dir, "timer", "timer.exe")
	os.WriteFile(plainPath, []byte("#!/bin/sh\n"), 0755)
	os.WriteFile(exePath, []byte("#!/bin/sh\n"), 0755)

	plugins, _ := DiscoverIn(dir)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	binPath := plugins[0].BinaryPath()
	if binPath != plainPath {
		t.Errorf("BinaryPath = %q, want %q (should prefer native binary)", binPath, plainPath)
	}
}

func TestGenerateIDDeterministic(t *testing.T) {
	id1 := generateID("com.example.filesystem")
	id2 := generateID("com.example.filesystem")
	if id1 != id2 {
		t.Errorf("IDs not deterministic: %q != %q", id1, id2)
	}

	id3 := generateID("com.example.notify")
	if id1 == id3 {
		t.Error("different namespaces should produce different IDs")
	}
}
