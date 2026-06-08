package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterAndDeregister(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	// Create a recurfile
	wfDir := t.TempDir()
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "echo action completed"
`), 0644)

	// Register
	out, stderr, code := runBin(t, home, watchBin, "register", wfPath)
	if code != 0 {
		t.Fatalf("register exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}
	if !strings.Contains(out, "Registered") {
		t.Errorf("register output = %q, want 'Registered'", strings.TrimSpace(out))
	}

	// List recurfiles
	out, _, code = runBin(t, home, watchBin, "list", "recurfiles")
	if code != 0 {
		t.Fatalf("list recurfiles exit code = %d", code)
	}
	if !strings.Contains(out, wfPath) {
		t.Errorf("list recurfiles should contain path %q, got: %s", wfPath, out)
	}

	// List triggers
	out, _, code = runBin(t, home, watchBin, "list", "triggers")
	if code != 0 {
		t.Fatalf("list triggers exit code = %d", code)
	}
	if !strings.Contains(out, "FileModified") {
		t.Errorf("list triggers should contain 'FileModified', got: %s", out)
	}

	// List actions
	out, _, code = runBin(t, home, watchBin, "list", "actions")
	if code != 0 {
		t.Fatalf("list actions exit code = %d", code)
	}
	if !strings.Contains(out, "Shell") {
		t.Errorf("list actions should contain 'Shell', got: %s", out)
	}

	// Deregister
	out, _, code = runBin(t, home, watchBin, "deregister", wfPath)
	if code != 0 {
		t.Fatalf("deregister exit code = %d", code)
	}
	if !strings.Contains(out, "Deregistered") {
		t.Errorf("deregister output = %q, want 'Deregistered'", strings.TrimSpace(out))
	}

	// Verify empty
	out, _, _ = runBin(t, home, watchBin, "list", "recurfiles")
	if strings.Contains(out, wfPath) {
		t.Error("recurfile should be deregistered")
	}
}

func TestRegisterDefaultDiscovery(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	// Create recurfile in a working directory
	wfDir := t.TempDir()
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(`
Test:
  on:
    - type: FileCreated
  do:
    - Shell: "echo test"
`), 0644)

	// Register from the directory containing the recurfile (no explicit path)
	out, stderr, code := runBinInDir(t, home, wfDir, watchBin, "register")
	if code != 0 {
		t.Fatalf("register (auto-discover) exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}
	if !strings.Contains(out, "Registered") {
		t.Errorf("expected 'Registered' in output, got: %s", out)
	}
}

func TestVerifyRecurfile(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	wfDir := t.TempDir()
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "echo action completed"
`), 0644)

	out, _, code := runBin(t, home, watchBin, "verify", wfPath)
	if code != 0 {
		t.Fatalf("verify exit code = %d", code)
	}
	if !strings.Contains(out, "Valid") && !strings.Contains(out, "valid") {
		t.Errorf("verify output should indicate valid, got: %s", out)
	}
}

func TestSuspendAndResumeTriggerE2E(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	// Register a recurfile
	wfDir := t.TempDir()
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "echo action completed"
`), 0644)

	runBin(t, home, watchBin, "register", wfPath)

	// Get trigger ID from list --json
	out, _, _ := runBin(t, home, watchBin, "list", "triggers", "--json")
	var triggers []map[string]any
	if err := json.Unmarshal([]byte(out), &triggers); err != nil {
		t.Fatalf("parse triggers JSON: %v\noutput: %s", err, out)
	}
	if len(triggers) == 0 {
		t.Fatal("expected at least 1 trigger")
	}
	triggerID := triggers[0]["id"].(string)

	// Suspend
	out, stderr, code := runBin(t, home, watchBin, "suspend", "trigger", triggerID)
	if code != 0 {
		t.Fatalf("suspend exit code = %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(out), "suspended") {
		t.Errorf("suspend output should contain 'suspended', got: %s", out)
	}

	// Resume
	out, stderr, code = runBin(t, home, watchBin, "resume", "trigger", triggerID)
	if code != 0 {
		t.Fatalf("resume exit code = %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(out), "resumed") {
		t.Errorf("resume output should contain 'resumed', got: %s", out)
	}
}

func TestInspectEntities(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	wfDir := t.TempDir()
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "echo action completed"
`), 0644)

	runBin(t, home, watchBin, "register", wfPath)

	// Inspect trigger --json
	out, _, _ := runBin(t, home, watchBin, "list", "triggers", "--json")
	var triggers []map[string]any
	json.Unmarshal([]byte(out), &triggers)
	triggerID := triggers[0]["id"].(string)

	out, _, code := runBin(t, home, watchBin, "inspect", "trigger", triggerID, "--json")
	if code != 0 {
		t.Fatalf("inspect trigger exit code = %d", code)
	}
	var trigger map[string]any
	if err := json.Unmarshal([]byte(out), &trigger); err != nil {
		t.Fatalf("parse inspect JSON: %v\noutput: %s", err, out)
	}
	if trigger["id"] != triggerID {
		t.Errorf("inspect trigger id = %v, want %s", trigger["id"], triggerID)
	}
}

func TestListJSON(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	wfDir := t.TempDir()
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "echo action completed"
`), 0644)

	runBin(t, home, watchBin, "register", wfPath)

	// All list commands should produce valid JSON
	for _, sub := range []string{"triggers", "actions", "groups", "recurfiles"} {
		out, _, code := runBin(t, home, watchBin, "list", sub, "--json")
		if code != 0 {
			t.Errorf("list %s --json exit code = %d", sub, code)
			continue
		}
		var result []any
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Errorf("list %s --json invalid JSON: %v\noutput: %s", sub, err, out)
		}
	}
}

func TestTestActionE2E(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	wfDir := t.TempDir()
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "echo hello from test={{.Test}}"
`), 0644)

	runBin(t, home, watchBin, "register", wfPath)

	// Get action ID
	out, _, _ := runBin(t, home, watchBin, "list", "actions", "--json")
	var actions []map[string]any
	json.Unmarshal([]byte(out), &actions)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}
	actionID := actions[0]["id"].(string)

	// Test action
	out, stderr, code := runBin(t, home, watchBin, "test", "action", actionID)
	if code != 0 {
		t.Fatalf("test action exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}
	if !strings.Contains(out, "hello from test=true") {
		t.Errorf("output should contain 'hello from test=true', got: %s", out)
	}
}

func TestTestActionWithSetVarsE2E(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t)
	defer cleanup()

	wfDir := t.TempDir()
	resultFile := filepath.Join(wfDir, "result.txt")
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(fmt.Sprintf(`
Log:
  on:
    - type: FileModified
  do:
    - Shell: "echo action completed > %s"
`, resultFile)), 0644)

	runBin(t, home, watchBin, "register", wfPath)

	out, _, _ := runBin(t, home, watchBin, "list", "actions", "--json")
	var actions []map[string]any
	json.Unmarshal([]byte(out), &actions)
	actionID := actions[0]["id"].(string)

	// Execute the action — it writes to a file
	_, stderr, code := runBin(t, home, watchBin, "test", "action", actionID)
	if code != 0 {
		t.Fatalf("test action exit code = %d, stderr: %s", code, stderr)
	}

	// Verify the file was created
	data, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatalf("result file not created: %v", err)
	}
	if !strings.Contains(string(data), "action completed") {
		t.Errorf("result file = %q, want 'action completed'", string(data))
	}
}
