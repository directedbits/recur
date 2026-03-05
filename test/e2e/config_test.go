package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigGetDefaults(t *testing.T) {
	home := t.TempDir()
	out, _, code := runRecur(t, home, "config", "get")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "error_threshold") {
		t.Error("output missing error_threshold")
	}
	if !strings.Contains(out, "default_shell") {
		t.Error("output missing default_shell")
	}
	if !strings.Contains(out, "= 5") {
		t.Error("output missing default error_threshold value of 5")
	}
}

func TestConfigGetSpecificKey(t *testing.T) {
	home := t.TempDir()
	out, _, code := runRecur(t, home, "config", "get", "error_threshold")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "error_threshold = 5") {
		t.Errorf("output = %q, want error_threshold = 5", strings.TrimSpace(out))
	}
}

func TestConfigGetUnknownKey(t *testing.T) {
	home := t.TempDir()
	_, _, code := runRecur(t, home, "config", "get", "nonexistent")

	if code == 0 {
		t.Fatal("expected non-zero exit code for unknown key")
	}
}

func TestConfigSetAndGet(t *testing.T) {
	home := t.TempDir()

	// Set
	_, _, code := runRecur(t, home, "config", "set", "error_threshold", "10")
	if code != 0 {
		t.Fatalf("set exit code = %d, want 0", code)
	}

	// Verify file was created
	configPath := filepath.Join(home, ".config", "recur", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Get it back
	out, _, code := runRecur(t, home, "config", "get", "error_threshold")
	if code != 0 {
		t.Fatalf("get exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "error_threshold = 10") {
		t.Errorf("output = %q, want error_threshold = 10", strings.TrimSpace(out))
	}
}

func TestConfigSetInvalidValue(t *testing.T) {
	home := t.TempDir()
	_, _, code := runRecur(t, home, "config", "set", "error_threshold", "not_a_number")

	if code == 0 {
		t.Fatal("expected non-zero exit code for invalid value")
	}
}

func TestConfigSetInvalidConcurrencyMode(t *testing.T) {
	home := t.TempDir()
	_, _, code := runRecur(t, home, "config", "set", "concurrency_mode", "invalid")

	if code == 0 {
		t.Fatal("expected non-zero exit code for invalid concurrency_mode")
	}
}

func TestConfigSetUnknownKey(t *testing.T) {
	home := t.TempDir()
	_, _, code := runRecur(t, home, "config", "set", "unknown_key", "value")

	if code == 0 {
		t.Fatal("expected non-zero exit code for unknown key")
	}
}

func TestConfigDeleteRevertsToDefault(t *testing.T) {
	home := t.TempDir()

	// Set to non-default
	runRecur(t, home, "config", "set", "error_threshold", "10")

	// Delete
	out, _, code := runRecur(t, home, "config", "delete", "error_threshold")
	if code != 0 {
		t.Fatalf("delete exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "reverted to default") {
		t.Errorf("output = %q, want 'reverted to default'", strings.TrimSpace(out))
	}

	// Verify default
	out, _, code = runRecur(t, home, "config", "get", "error_threshold")
	if code != 0 {
		t.Fatalf("get exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "error_threshold = 5") {
		t.Errorf("output = %q, want error_threshold = 5", strings.TrimSpace(out))
	}
}

func TestConfigDeleteIdempotent(t *testing.T) {
	home := t.TempDir()

	// Delete without ever setting — should succeed
	_, _, code := runRecur(t, home, "config", "delete", "error_threshold")
	if code != 0 {
		t.Fatalf("delete exit code = %d, want 0 (idempotent)", code)
	}
}

func TestConfigGetJSON(t *testing.T) {
	home := t.TempDir()

	// Get all as JSON
	out, _, code := runRecur(t, home, "config", "get", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}

	if result["error_threshold"] != float64(5) {
		t.Errorf("error_threshold = %v, want 5", result["error_threshold"])
	}
}

func TestConfigGetSpecificKeyJSON(t *testing.T) {
	home := t.TempDir()
	out, _, code := runRecur(t, home, "config", "get", "error_threshold", "--json")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "5" {
		t.Errorf("output = %q, want %q", strings.TrimSpace(out), "5")
	}
}

func TestConfigPluginSetGetDelete(t *testing.T) {
	home := t.TempDir()

	// Set plugin config
	_, _, code := runRecur(t, home, "config", "set", "plugins.com.recur.filesystem.poll_interval", "10")
	if code != 0 {
		t.Fatalf("set exit code = %d, want 0", code)
	}

	// Get it back
	out, _, code := runRecur(t, home, "config", "get", "plugins.com.recur.filesystem.poll_interval")
	if code != 0 {
		t.Fatalf("get exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "= 10") {
		t.Errorf("output = %q, want value 10", strings.TrimSpace(out))
	}

	// Should appear in get all
	out, _, _ = runRecur(t, home, "config", "get")
	if !strings.Contains(out, "plugins.com.recur.filesystem.poll_interval") {
		t.Error("plugin config missing from get all output")
	}

	// Delete
	out, _, code = runRecur(t, home, "config", "delete", "plugins.com.recur.filesystem.poll_interval")
	if code != 0 {
		t.Fatalf("delete exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "deleted") {
		t.Errorf("output = %q, want 'deleted'", strings.TrimSpace(out))
	}

	// Verify gone
	_, _, code = runRecur(t, home, "config", "get", "plugins.com.recur.filesystem.poll_interval")
	if code == 0 {
		t.Fatal("expected non-zero exit code for deleted plugin config")
	}
}

func TestConfigQuietFlag(t *testing.T) {
	home := t.TempDir()

	out, _, code := runRecur(t, home, "config", "set", "error_threshold", "10", "--quiet")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("quiet mode should produce no output, got %q", out)
	}
}

func TestConfigMultipleSetsThenGetAll(t *testing.T) {
	home := t.TempDir()

	runRecur(t, home, "config", "set", "error_threshold", "10")
	runRecur(t, home, "config", "set", "default_shell", "bash -c")
	runRecur(t, home, "config", "set", "concurrency_mode", "parallel")

	out, _, code := runRecur(t, home, "config", "get")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "= 10") {
		t.Error("output missing updated error_threshold")
	}
	if !strings.Contains(out, "bash -c") {
		t.Error("output missing updated default_shell")
	}
	if !strings.Contains(out, "parallel") {
		t.Error("output missing updated concurrency_mode")
	}
}
