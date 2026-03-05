package configyaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	defaultsos "github.com/directedbits/recur/src/infra/os/defaults"
)

func tempConfigPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "config.yaml")
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if *cfg.DefaultShell != defaultsos.DefaultShell {
		t.Errorf("DefaultShell = %q, want %q", *cfg.DefaultShell, defaultsos.DefaultShell)
	}
	if *cfg.ErrorThreshold != 5 {
		t.Errorf("ErrorThreshold = %d, want %d", *cfg.ErrorThreshold, 5)
	}
	if cfg.TriggerErrorThreshold != nil {
		t.Errorf("TriggerErrorThreshold = %v, want nil", cfg.TriggerErrorThreshold)
	}
	if cfg.ActionErrorThreshold != nil {
		t.Errorf("ActionErrorThreshold = %v, want nil", cfg.ActionErrorThreshold)
	}
	if *cfg.ConcurrencyMode != "queue" {
		t.Errorf("ConcurrencyMode = %q, want %q", *cfg.ConcurrencyMode, "queue")
	}
	if *cfg.MaxQueueSize != 100 {
		t.Errorf("MaxQueueSize = %d, want %d", *cfg.MaxQueueSize, 100)
	}
	if *cfg.Debounce != "300ms" {
		t.Errorf("Debounce = %q, want %q", *cfg.Debounce, "300ms")
	}
	if *cfg.ShutdownTimeout != "30s" {
		t.Errorf("ShutdownTimeout = %q, want %q", *cfg.ShutdownTimeout, "30s")
	}
}

func TestDefaultConfig_SocketAddress(t *testing.T) {
	cfg := DefaultConfig()
	if *cfg.SocketAddress != defaultsos.DefaultSocketAddress {
		t.Errorf("SocketAddress = %q, want %q", *cfg.SocketAddress, defaultsos.DefaultSocketAddress)
	}
}

func TestNewStoreMissingFile(t *testing.T) {
	store, err := NewStore("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("NewStore returned error for missing file: %v", err)
	}
	cfg := store.Get()
	if *cfg.ErrorThreshold != 5 {
		t.Errorf("expected defaults, got ErrorThreshold = %d", *cfg.ErrorThreshold)
	}
}

func TestNewStoreInvalidYAML(t *testing.T) {
	path := tempConfigPath(t)
	os.WriteFile(path, []byte("error_threshold:\n  - bad\n  nested: [unclosed"), 0644)

	_, err := NewStore(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestSaveAndReload(t *testing.T) {
	path := tempConfigPath(t)
	cfg := DefaultConfig()
	cfg.ErrorThreshold = intPtr(10)
	cfg.DefaultShell = strPtr("bash -c")

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	loaded := store.Get()

	if *loaded.ErrorThreshold != 10 {
		t.Errorf("ErrorThreshold = %d, want 10", *loaded.ErrorThreshold)
	}
	if *loaded.DefaultShell != "bash -c" {
		t.Errorf("DefaultShell = %q, want %q", *loaded.DefaultShell, "bash -c")
	}
	// Unset fields should retain defaults via overlay
	if *loaded.ConcurrencyMode != "queue" {
		t.Errorf("ConcurrencyMode = %q, want %q", *loaded.ConcurrencyMode, "queue")
	}
}

func TestSaveAtomicCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "nested", "config.yaml")

	cfg := DefaultConfig()
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed to create nested dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestIsHostAllowed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AllowedHosts = strPtr("github.com, gitlab.com")

	if !cfg.IsHostAllowed("github.com") {
		t.Error("github.com should be allowed")
	}
	if !cfg.IsHostAllowed("gitlab.com") {
		t.Error("gitlab.com should be allowed")
	}
	if !cfg.IsHostAllowed("GitHub.com") {
		t.Error("case-insensitive match should work")
	}
	if cfg.IsHostAllowed("evil.com") {
		t.Error("evil.com should not be allowed")
	}
}

func TestIsHostAllowed_Empty(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.IsHostAllowed("github.com") {
		t.Error("empty allowed_hosts should deny all")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := tempConfigPath(t)
	cfg := DefaultConfig()
	cfg.ErrorThreshold = intPtr(10)
	cfg.DefaultShell = strPtr("bash -c")
	v := 3
	cfg.TriggerErrorThreshold = &v
	cfg.Plugins = map[string]map[string]any{
		"com.example.filesystem": {"poll_interval": 5, "follow_symlinks": true},
	}

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	loaded := store.Get()

	if *loaded.ErrorThreshold != 10 {
		t.Errorf("ErrorThreshold = %d, want 10", *loaded.ErrorThreshold)
	}
	if loaded.TriggerErrorThreshold == nil || *loaded.TriggerErrorThreshold != 3 {
		t.Errorf("TriggerErrorThreshold = %v, want 3", loaded.TriggerErrorThreshold)
	}
	if loaded.Plugins["com.example.filesystem"]["poll_interval"] != 5 {
		t.Errorf("plugin poll_interval = %v, want 5", loaded.Plugins["com.example.filesystem"]["poll_interval"])
	}
}

func TestToJSON(t *testing.T) {
	v := map[string]any{"key": "value", "num": 42}
	result, err := ToJSON(v)
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	if !strings.Contains(result, `"key": "value"`) {
		t.Errorf("ToJSON output missing expected content: %s", result)
	}
}

func TestNewStore_MissingFile(t *testing.T) {
	store, err := NewStore("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("NewStore returned error for missing file: %v", err)
	}

	cfg := store.Get()
	if *cfg.ErrorThreshold != 5 {
		t.Errorf("ErrorThreshold = %d, want default 5", *cfg.ErrorThreshold)
	}
	if *cfg.ConcurrencyMode != "queue" {
		t.Errorf("ConcurrencyMode = %q, want default 'queue'", *cfg.ConcurrencyMode)
	}
}

func TestNewStore_WithFile(t *testing.T) {
	path := tempConfigPath(t)
	os.WriteFile(path, []byte("error_threshold: 10\nlog_level: debug\n"), 0644)

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	cfg := store.Get()
	if *cfg.ErrorThreshold != 10 {
		t.Errorf("ErrorThreshold = %d, want 10", *cfg.ErrorThreshold)
	}
	if *cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want 'debug'", *cfg.LogLevel)
	}
	// Defaults still apply for unset fields
	if *cfg.ConcurrencyMode != "queue" {
		t.Errorf("ConcurrencyMode = %q, want default 'queue'", *cfg.ConcurrencyMode)
	}
}

func TestNewStore_SourceTracking(t *testing.T) {
	path := tempConfigPath(t)
	os.WriteFile(path, []byte("error_threshold: 10\n"), 0644)

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// error_threshold should show "file" as the source
	layers := store.Inspect("ErrorThreshold")
	found := false
	for _, l := range layers {
		if l.Layer == "file" && l.Defined {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ErrorThreshold to be defined in 'file' layer")
	}

	// ConcurrencyMode should show "default" as the source
	layers = store.Inspect("ConcurrencyMode")
	found = false
	for _, l := range layers {
		if l.Layer == "default" && l.Defined {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ConcurrencyMode to be defined in 'default' layer")
	}
}

func TestInterpolatePluginEnvVars(t *testing.T) {
	t.Setenv("MQTT_PASS", "secret123")

	path := tempConfigPath(t)
	os.WriteFile(path, []byte(`plugins:
  core.mqtt:
    password: "${MQTT_PASS}"
    broker: "tcp://localhost:1883"
`), 0644)

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	cfg := store.Get()
	if cfg.Plugins["core.mqtt"]["password"] != "secret123" {
		t.Errorf("password = %q, want %q", cfg.Plugins["core.mqtt"]["password"], "secret123")
	}
	if cfg.Plugins["core.mqtt"]["broker"] != "tcp://localhost:1883" {
		t.Errorf("broker should be unchanged, got %q", cfg.Plugins["core.mqtt"]["broker"])
	}
}

func TestInterpolatePluginEnvVars_Unset(t *testing.T) {
	os.Unsetenv("UNSET_VAR_FOR_TEST")

	path := tempConfigPath(t)
	os.WriteFile(path, []byte(`plugins:
  core.mqtt:
    password: "${UNSET_VAR_FOR_TEST}"
`), 0644)

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	cfg := store.Get()
	if cfg.Plugins["core.mqtt"]["password"] != "${UNSET_VAR_FOR_TEST}" {
		t.Errorf("unset var should remain literal, got %q", cfg.Plugins["core.mqtt"]["password"])
	}
}

func TestInterpolatePluginEnvVars_NoPlugins(t *testing.T) {
	cfg := &Config{}
	interpolatePluginEnvVars(cfg)
	if cfg.Plugins != nil {
		t.Error("should be nil when no plugins")
	}
}
