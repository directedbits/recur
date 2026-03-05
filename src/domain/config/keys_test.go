package config

import (
	"testing"

	pkgconfig "github.com/directedbits/recur/pkg/config"
)

// testDefaults returns a fixed Config used as the "default" layer in tests.
// Hard-coded here instead of pulling from infra so the domain package has no
// upward dependency.
func testDefaults() *Config {
	s := func(v string) *string { return &v }
	i := func(v int) *int { return &v }
	return &Config{
		DefaultShell:    s("sh -c"),
		ErrorThreshold:  i(5),
		ConcurrencyMode: s("queue"),
		MaxQueueSize:    i(100),
		Debounce:        s("300ms"),
		ShutdownTimeout: s("30s"),
		LogLevel:        s(""),
		SocketAddress:   s(""),
		AllowedHosts:    s(""),
	}
}

// newTestStore creates an in-memory Store with defaults on the "default" layer.
func newTestStore() *pkgconfig.Store[Config] {
	store := pkgconfig.NewStore[Config]("default", "file")
	store.Set("default", *testDefaults())
	return store
}

func TestGetByKey_DaemonKeys(t *testing.T) {
	store := newTestStore()
	defaults := testDefaults()

	tests := []struct {
		key      string
		wantInt  int
		wantStr  string
		isInt    bool
		isNilPtr bool
	}{
		{"default_shell", 0, *defaults.DefaultShell, false, false},
		{"error_threshold", 5, "", true, false},
		{"concurrency_mode", 0, "queue", false, false},
		{"max_queue_size", 100, "", true, false},
		{"debounce", 0, "300ms", false, false},
		{"shutdown_timeout", 0, "30s", false, false},
	}

	for _, tt := range tests {
		val, err := GetByKey(store, tt.key)
		if err != nil {
			t.Errorf("GetByKey(%q) error: %v", tt.key, err)
			continue
		}
		if tt.isInt {
			intVal, ok := val.(int)
			if !ok {
				t.Errorf("GetByKey(%q) = %T, want int", tt.key, val)
				continue
			}
			if intVal != tt.wantInt {
				t.Errorf("GetByKey(%q) = %d, want %d", tt.key, intVal, tt.wantInt)
			}
		} else {
			strVal, ok := val.(string)
			if !ok {
				t.Errorf("GetByKey(%q) = %T, want string", tt.key, val)
				continue
			}
			if strVal != tt.wantStr {
				t.Errorf("GetByKey(%q) = %q, want %q", tt.key, strVal, tt.wantStr)
			}
		}
	}
}

func TestGetByKey_ThresholdFallbackViaAllKeys(t *testing.T) {
	store := newTestStore()

	// trigger_error_threshold and action_error_threshold are nil in defaults.
	// AllKeys computes effective values using the fallback to error_threshold.
	all := AllKeys(store)

	var trigVal, actVal any
	for _, kv := range all {
		switch kv.Key {
		case "trigger_error_threshold":
			trigVal = kv.Value
		case "action_error_threshold":
			actVal = kv.Value
		}
	}

	if trigVal != 5 {
		t.Errorf("trigger_error_threshold effective = %v, want 5 (from error_threshold)", trigVal)
	}
	if actVal != 5 {
		t.Errorf("action_error_threshold effective = %v, want 5 (from error_threshold)", actVal)
	}
}

func TestGetByKey_ExplicitOverride(t *testing.T) {
	store := newTestStore()

	// Set trigger_error_threshold to 3 on the file layer
	if err := SetByKey(store, "file", "trigger_error_threshold", "3"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}

	val, err := GetByKey(store, "trigger_error_threshold")
	if err != nil {
		t.Fatalf("GetByKey error: %v", err)
	}
	intVal, ok := val.(int)
	if !ok {
		t.Fatalf("trigger_error_threshold = %T, want int", val)
	}
	if intVal != 3 {
		t.Errorf("trigger_error_threshold effective = %v, want 3", intVal)
	}
}

func TestGetByKey_OverriddenThresholds(t *testing.T) {
	store := newTestStore()

	if err := SetByKey(store, "file", "trigger_error_threshold", "3"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}

	val, err := GetByKey(store, "trigger_error_threshold")
	if err != nil {
		t.Fatalf("GetByKey error: %v", err)
	}
	intVal, ok := val.(int)
	if !ok || intVal != 3 {
		t.Errorf("trigger_error_threshold = %v, want 3", val)
	}

	// action_error_threshold still not set — verify via AllKeys which handles fallback
	all := AllKeys(store)
	var actVal any
	for _, kv := range all {
		if kv.Key == "action_error_threshold" {
			actVal = kv.Value
		}
	}
	if actVal != 5 {
		t.Errorf("action_error_threshold effective (via AllKeys) = %v, want 5 (fallback)", actVal)
	}
}

func TestGetByKey_UnknownKey(t *testing.T) {
	store := newTestStore()
	_, err := GetByKey(store, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestSetByKey_DaemonKeys(t *testing.T) {
	store := newTestStore()

	if err := SetByKey(store, "file", "error_threshold", "10"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg := store.Get()
	if *cfg.ErrorThreshold != 10 {
		t.Errorf("ErrorThreshold = %d, want 10", *cfg.ErrorThreshold)
	}

	if err := SetByKey(store, "file", "default_shell", "zsh -c"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg = store.Get()
	if *cfg.DefaultShell != "zsh -c" {
		t.Errorf("DefaultShell = %q, want %q", *cfg.DefaultShell, "zsh -c")
	}

	if err := SetByKey(store, "file", "concurrency_mode", "parallel"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg = store.Get()
	if *cfg.ConcurrencyMode != "parallel" {
		t.Errorf("ConcurrencyMode = %q, want %q", *cfg.ConcurrencyMode, "parallel")
	}
}

func TestSetByKey_InvalidConcurrencyMode(t *testing.T) {
	store := newTestStore()
	err := SetByKey(store, "file", "concurrency_mode", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid concurrency_mode, got nil")
	}
}

func TestSetByKey_InvalidIntValue(t *testing.T) {
	store := newTestStore()
	err := SetByKey(store, "file", "error_threshold", "not_a_number")
	if err == nil {
		t.Fatal("expected error for non-integer value, got nil")
	}
}

func TestSetByKey_UnknownKey(t *testing.T) {
	store := newTestStore()
	err := SetByKey(store, "file", "nonexistent", "value")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestSetByKey_TriggerErrorThreshold(t *testing.T) {
	store := newTestStore()
	if err := SetByKey(store, "file", "trigger_error_threshold", "3"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg := store.Get()
	if cfg.TriggerErrorThreshold == nil || *cfg.TriggerErrorThreshold != 3 {
		t.Errorf("TriggerErrorThreshold = %v, want 3", cfg.TriggerErrorThreshold)
	}
}

func TestDeleteByKey_DaemonKeys(t *testing.T) {
	store := newTestStore()

	// Set error_threshold on file layer, then delete it
	SetByKey(store, "file", "error_threshold", "10")
	if err := DeleteByKey(store, "file", "error_threshold"); err != nil {
		t.Fatalf("DeleteByKey error: %v", err)
	}
	cfg := store.Get()
	// Should fall back to default layer value
	if *cfg.ErrorThreshold != 5 {
		t.Errorf("ErrorThreshold = %d, want default 5", *cfg.ErrorThreshold)
	}

	// Set trigger_error_threshold on file layer, then delete it
	SetByKey(store, "file", "trigger_error_threshold", "3")
	if err := DeleteByKey(store, "file", "trigger_error_threshold"); err != nil {
		t.Fatalf("DeleteByKey error: %v", err)
	}
	cfg = store.Get()
	if cfg.TriggerErrorThreshold != nil {
		t.Errorf("TriggerErrorThreshold = %v, want nil", cfg.TriggerErrorThreshold)
	}
}

func TestDeleteByKey_UnknownKey(t *testing.T) {
	store := newTestStore()
	err := DeleteByKey(store, "file", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestAllKeys(t *testing.T) {
	store := newTestStore()
	all := AllKeys(store)

	if len(all) != 11 {
		t.Errorf("AllKeys returned %d items, want 11", len(all))
	}

	// Verify ordering and first/last
	if all[0].Key != "default_shell" {
		t.Errorf("first key = %q, want %q", all[0].Key, "default_shell")
	}
	if all[7].Key != "shutdown_timeout" {
		t.Errorf("key at index 7 = %q, want %q", all[7].Key, "shutdown_timeout")
	}
	if all[8].Key != "log_level" {
		t.Errorf("key at index 8 = %q, want %q", all[8].Key, "log_level")
	}
}

func TestAllKeysWithPlugins(t *testing.T) {
	store := newTestStore()

	// Set plugin config on the file layer
	SetByKey(store, "file", "plugins.com.example.filesystem.poll_interval", "10")

	all := AllKeys(store)
	if len(all) != 12 {
		t.Errorf("AllKeys returned %d items, want 12", len(all))
	}

	last := all[len(all)-1]
	if last.Key != "plugins.com.example.filesystem.poll_interval" {
		t.Errorf("last key = %q, want plugin key", last.Key)
	}
}

func TestPluginConfigGetSetDelete(t *testing.T) {
	store := newTestStore()

	// Set
	if err := SetByKey(store, "file", "plugins.com.example.filesystem.poll_interval", "10"); err != nil {
		t.Fatalf("SetByKey plugin config error: %v", err)
	}

	// Get
	val, err := GetByKey(store, "plugins.com.example.filesystem.poll_interval")
	if err != nil {
		t.Fatalf("GetByKey plugin config error: %v", err)
	}
	if val != "10" {
		t.Errorf("plugin config value = %v, want %q", val, "10")
	}

	// Delete
	if err := DeleteByKey(store, "file", "plugins.com.example.filesystem.poll_interval"); err != nil {
		t.Fatalf("DeleteByKey plugin config error: %v", err)
	}

	// Verify deleted
	_, err = GetByKey(store, "plugins.com.example.filesystem.poll_interval")
	if err == nil {
		t.Fatal("expected error after deleting plugin config, got nil")
	}

	// Namespace should be cleaned up
	cfg := store.Get()
	if len(cfg.Plugins) != 0 {
		t.Errorf("Plugins map not empty after deleting last key: %v", cfg.Plugins)
	}
}

func TestPluginConfigDeleteIdempotent(t *testing.T) {
	store := newTestStore()

	// Delete non-existent plugin config should not error
	if err := DeleteByKey(store, "file", "plugins.com.example.foo.bar"); err != nil {
		t.Fatalf("DeleteByKey non-existent plugin config should not error: %v", err)
	}
}

func TestPluginConfigGetMissing(t *testing.T) {
	store := newTestStore()

	_, err := GetByKey(store, "plugins.com.example.foo.bar")
	if err == nil {
		t.Fatal("expected error for missing plugin config, got nil")
	}
}

func TestPluginConfigInvalidKey(t *testing.T) {
	store := newTestStore()

	// No field part
	err := SetByKey(store, "file", "plugins.nofieldpart", "value")
	if err == nil {
		t.Fatal("expected error for invalid plugin key, got nil")
	}
}

func TestDeleteByKey_AllDaemonKeys(t *testing.T) {
	store := newTestStore()
	defaults := testDefaults()

	// Set various keys on the file layer
	SetByKey(store, "file", "default_shell", "zsh -c")
	SetByKey(store, "file", "concurrency_mode", "parallel")
	SetByKey(store, "file", "debounce", "1s")
	SetByKey(store, "file", "shutdown_timeout", "10s")
	SetByKey(store, "file", "socket_address", "localhost:9090")
	SetByKey(store, "file", "allowed_hosts", "example.com")
	SetByKey(store, "file", "max_queue_size", "50")

	keys := []string{"default_shell", "concurrency_mode", "debounce", "shutdown_timeout", "socket_address", "allowed_hosts", "max_queue_size"}
	for _, key := range keys {
		if err := DeleteByKey(store, "file", key); err != nil {
			t.Fatalf("DeleteByKey(%s) error: %v", key, err)
		}
	}

	cfg := store.Get()
	if *cfg.DefaultShell != *defaults.DefaultShell {
		t.Errorf("DefaultShell = %q, want %q", *cfg.DefaultShell, *defaults.DefaultShell)
	}
	if *cfg.MaxQueueSize != *defaults.MaxQueueSize {
		t.Errorf("MaxQueueSize = %d, want %d", *cfg.MaxQueueSize, *defaults.MaxQueueSize)
	}
	if *cfg.SocketAddress != *defaults.SocketAddress {
		t.Errorf("SocketAddress = %q, want %q", *cfg.SocketAddress, *defaults.SocketAddress)
	}
}

func TestGetByKey_SocketAddress(t *testing.T) {
	store := newTestStore()
	SetByKey(store, "file", "socket_address", "localhost:9090")

	val, err := GetByKey(store, "socket_address")
	if err != nil {
		t.Fatalf("GetByKey error: %v", err)
	}
	strVal, ok := val.(string)
	if !ok || strVal != "localhost:9090" {
		t.Errorf("socket_address = %v, want %q", val, "localhost:9090")
	}
}

func TestGetByKey_AllowedHosts(t *testing.T) {
	store := newTestStore()
	SetByKey(store, "file", "allowed_hosts", "github.com")

	val, err := GetByKey(store, "allowed_hosts")
	if err != nil {
		t.Fatalf("GetByKey error: %v", err)
	}
	strVal, ok := val.(string)
	if !ok || strVal != "github.com" {
		t.Errorf("allowed_hosts = %v, want %q", val, "github.com")
	}
}

func TestSetByKey_SocketAddressAndAllowedHosts(t *testing.T) {
	store := newTestStore()
	if err := SetByKey(store, "file", "socket_address", "localhost:9999"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg := store.Get()
	if *cfg.SocketAddress != "localhost:9999" {
		t.Errorf("SocketAddress = %q", *cfg.SocketAddress)
	}
	if err := SetByKey(store, "file", "allowed_hosts", "a.com,b.com"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg = store.Get()
	if *cfg.AllowedHosts != "a.com,b.com" {
		t.Errorf("AllowedHosts = %q", *cfg.AllowedHosts)
	}
}

func TestSetByKey_Debounce(t *testing.T) {
	store := newTestStore()
	if err := SetByKey(store, "file", "debounce", "500ms"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg := store.Get()
	if *cfg.Debounce != "500ms" {
		t.Errorf("Debounce = %q", *cfg.Debounce)
	}
}

func TestSetByKey_ShutdownTimeout(t *testing.T) {
	store := newTestStore()
	if err := SetByKey(store, "file", "shutdown_timeout", "60s"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg := store.Get()
	if *cfg.ShutdownTimeout != "60s" {
		t.Errorf("ShutdownTimeout = %q", *cfg.ShutdownTimeout)
	}
}

func TestSetByKey_MaxQueueSize(t *testing.T) {
	store := newTestStore()
	if err := SetByKey(store, "file", "max_queue_size", "200"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg := store.Get()
	if *cfg.MaxQueueSize != 200 {
		t.Errorf("MaxQueueSize = %d", *cfg.MaxQueueSize)
	}
}

func TestSetByKey_ActionErrorThreshold(t *testing.T) {
	store := newTestStore()
	if err := SetByKey(store, "file", "action_error_threshold", "7"); err != nil {
		t.Fatalf("SetByKey error: %v", err)
	}
	cfg := store.Get()
	if cfg.ActionErrorThreshold == nil || *cfg.ActionErrorThreshold != 7 {
		t.Errorf("ActionErrorThreshold = %v", cfg.ActionErrorThreshold)
	}
}

func TestGetByKey_RegularKey(t *testing.T) {
	store := newTestStore()
	val, err := GetByKey(store, "default_shell")
	if err != nil {
		t.Fatalf("GetByKey error: %v", err)
	}
	strVal, ok := val.(string)
	if !ok || strVal == "" {
		t.Fatalf("default_shell = %T %v, want string", val, val)
	}
	defaults := testDefaults()
	if strVal != *defaults.DefaultShell {
		t.Errorf("val = %q, want %q", strVal, *defaults.DefaultShell)
	}
}

func TestGetByKey_UnknownKeyError(t *testing.T) {
	store := newTestStore()
	_, err := GetByKey(store, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestPluginConfigSetMultipleKeys(t *testing.T) {
	store := newTestStore()
	SetByKey(store, "file", "plugins.com.recur.mqtt.broker", "localhost:1883")
	SetByKey(store, "file", "plugins.com.recur.mqtt.username", "admin")

	val, err := GetByKey(store, "plugins.com.recur.mqtt.broker")
	if err != nil {
		t.Fatalf("GetByKey error: %v", err)
	}
	if val != "localhost:1883" {
		t.Errorf("broker = %v", val)
	}

	// Delete one key, other should remain
	DeleteByKey(store, "file", "plugins.com.recur.mqtt.broker")
	cfg := store.Get()
	if _, ok := cfg.Plugins["com.recur.mqtt"]; !ok {
		t.Error("namespace should still exist after deleting one key")
	}
	_, err = GetByKey(store, "plugins.com.recur.mqtt.username")
	if err != nil {
		t.Errorf("username should still be accessible: %v", err)
	}
}

func TestSetByKey_LogLevel_Valid(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		store := newTestStore()
		if err := SetByKey(store, "file", "log_level", level); err != nil {
			t.Errorf("SetByKey log_level=%q error: %v", level, err)
		}
		cfg := store.Get()
		if *cfg.LogLevel != level {
			t.Errorf("LogLevel = %q, want %q", *cfg.LogLevel, level)
		}
	}
}

func TestSetByKey_LogLevel_Invalid(t *testing.T) {
	store := newTestStore()
	err := SetByKey(store, "file", "log_level", "trace")
	if err == nil {
		t.Fatal("expected error for invalid log_level, got nil")
	}
}

func TestGetByKey_LogLevel(t *testing.T) {
	store := newTestStore()
	val, err := GetByKey(store, "log_level")
	if err != nil {
		t.Fatalf("GetByKey error: %v", err)
	}
	strVal, ok := val.(string)
	if !ok {
		t.Fatalf("log_level = %T, want string", val)
	}
	if strVal != "" {
		t.Errorf("log_level = %q, want empty string", strVal)
	}
}

func TestDeleteByKey_LogLevel(t *testing.T) {
	store := newTestStore()
	SetByKey(store, "file", "log_level", "debug")
	if err := DeleteByKey(store, "file", "log_level"); err != nil {
		t.Fatalf("DeleteByKey error: %v", err)
	}
	cfg := store.Get()
	if *cfg.LogLevel != "" {
		t.Errorf("LogLevel = %q, want empty", *cfg.LogLevel)
	}
}

func TestPluginConfigGetInvalidFormat(t *testing.T) {
	store := newTestStore()
	// Just "plugins." with no namespace
	_, err := GetByKey(store, "plugins.")
	if err == nil {
		t.Fatal("expected error for empty plugin key")
	}
}

func TestPluginConfigSetInvalidFormat(t *testing.T) {
	store := newTestStore()
	err := SetByKey(store, "file", "plugins.", "value")
	if err == nil {
		t.Fatal("expected error for empty plugin key")
	}
}

func TestPluginConfigDeleteInvalidFormat(t *testing.T) {
	store := newTestStore()
	err := DeleteByKey(store, "file", "plugins.")
	if err == nil {
		t.Fatal("expected error for empty plugin key")
	}
}

func TestSplitPluginKey(t *testing.T) {
	tests := []struct {
		input string
		ns    string
		field string
		err   bool
	}{
		{"plugins.com.example.filesystem.poll_interval", "com.example.filesystem", "poll_interval", false},
		{"plugins.simple.key", "simple", "key", false},
		{"nofield", "", "", true},
		{"", "", "", true},
		{"plugins.", "", "", true},
	}

	for _, tt := range tests {
		ns, field, err := SplitPluginKey(tt.input)
		if tt.err && err == nil {
			t.Errorf("SplitPluginKey(%q) expected error, got nil", tt.input)
			continue
		}
		if !tt.err && err != nil {
			t.Errorf("SplitPluginKey(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if ns != tt.ns || field != tt.field {
			t.Errorf("SplitPluginKey(%q) = (%q, %q), want (%q, %q)", tt.input, ns, field, tt.ns, tt.field)
		}
	}
}
