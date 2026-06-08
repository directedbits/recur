package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDaemonStartAndStop(t *testing.T) {
	home := t.TempDir()
	pidPath := filepath.Join(home, ".config", "recur", "run", "recur.pid")

	// Start daemon
	out, stderr, code := runBin(t, home, recurBinary, "start")
	if code != 0 {
		t.Fatalf("start exit code = %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(out, "Daemon started") {
		t.Errorf("start output = %q, want 'Daemon started'", strings.TrimSpace(out))
	}

	// Give daemon time to write PID file
	time.Sleep(200 * time.Millisecond)

	// Verify PID file exists
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("PID file not created: %v", err)
	}

	// Status should report running
	out, _, code = runBin(t, home, recurBinary, "status")
	if code != 0 {
		t.Fatalf("status exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("status output = %q, want 'running'", strings.TrimSpace(out))
	}

	// Stop daemon
	out, stderr, code = runBin(t, home, recurBinary, "stop")
	if code != 0 {
		t.Fatalf("stop exit code = %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(out, "Daemon stopped") {
		t.Errorf("stop output = %q, want 'Daemon stopped'", strings.TrimSpace(out))
	}

	// Wait a moment for cleanup
	time.Sleep(200 * time.Millisecond)

	// Status should report not running (exit code 1)
	_, _, code = runBin(t, home, recurBinary, "status")
	if code != 1 {
		t.Errorf("status exit code = %d, want 1 (not running)", code)
	}
}

func TestDaemonStartAlreadyRunning(t *testing.T) {
	home := t.TempDir()

	// Start daemon
	_, _, code := runBin(t, home, recurBinary, "start")
	if code != 0 {
		t.Fatalf("first start exit code = %d", code)
	}
	time.Sleep(200 * time.Millisecond)

	// Try to start again — should error
	_, _, code = runBin(t, home, recurBinary, "start")
	if code == 0 {
		t.Fatal("expected non-zero exit code for double start")
	}

	// Clean up
	runBin(t, home, recurBinary, "stop")
	time.Sleep(200 * time.Millisecond)
}

func TestDaemonStopNotRunning(t *testing.T) {
	home := t.TempDir()

	_, _, code := runBin(t, home, recurBinary, "stop")
	if code == 0 {
		t.Fatal("expected non-zero exit code for stop when not running")
	}
}

func TestDaemonStatusJSON(t *testing.T) {
	home := t.TempDir()

	// Not running — JSON
	out, _, _ := runBin(t, home, recurBinary, "status", "--json")
	var status map[string]any
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatalf("status JSON parse error: %v\noutput: %s", err, out)
	}
	if status["running"] != false {
		t.Errorf("status.running = %v, want false", status["running"])
	}
}

func TestDaemonStartWithConfigFile(t *testing.T) {
	home := t.TempDir()

	// Create a custom config file
	customConfig := filepath.Join(home, "custom-config.yaml")
	os.WriteFile(customConfig, []byte("error_threshold: 42\n"), 0644)

	// Start with --file
	_, _, code := runBin(t, home, recurBinary, "start", "--file", customConfig)
	if code != 0 {
		t.Fatalf("start --file exit code = %d", code)
	}
	time.Sleep(200 * time.Millisecond)

	// Clean up
	runBin(t, home, recurBinary, "stop")
	time.Sleep(200 * time.Millisecond)
}
