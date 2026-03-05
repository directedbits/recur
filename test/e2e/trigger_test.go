package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileCreatedTriggerFiresAction(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t, "fileevents")
	defer cleanup()

	// Set up directories
	wfDir := t.TempDir()
	watchDir := t.TempDir()
	resultFile := filepath.Join(wfDir, "result.txt")

	// Create a recurfile that watches watchDir and writes to resultFile on FileCreated
	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(fmt.Sprintf(`
Build:
  on:
    - type: FileCreated
      options:
        path: "%s"
  do:
    - Shell: "echo triggered > %s"
`, watchDir, resultFile)), 0644)

	// Register
	out, stderr, code := runBin(t, home, watchBin, "register", wfPath)
	if code != 0 {
		t.Fatalf("register exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}

	// Give the trigger engine time to start watching
	time.Sleep(500 * time.Millisecond)

	// Create a file in the watched directory
	testFile := filepath.Join(watchDir, "newfile.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// Wait for the action to execute
	deadline := time.After(10 * time.Second)
	for {
		data, err := os.ReadFile(resultFile)
		if err == nil && len(data) > 0 {
			if got := string(data); got != "" {
				if !strings.Contains(got, "triggered") {
					t.Errorf("result file = %q, want 'triggered'", got)
				}
				break
			}
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for trigger to fire action")
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestFileModifiedTriggerFiresAction(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t, "fileevents")
	defer cleanup()

	wfDir := t.TempDir()
	watchDir := t.TempDir()
	resultFile := filepath.Join(wfDir, "result.txt")

	// Create a file BEFORE registering so the watcher knows about it
	existingFile := filepath.Join(watchDir, "existing.txt")
	os.WriteFile(existingFile, []byte("initial"), 0644)

	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(fmt.Sprintf(`
Monitor:
  on:
    - type: FileModified
      options:
        path: "%s"
  do:
    - Shell: "echo modified > %s"
`, watchDir, resultFile)), 0644)

	// Register
	out, stderr, code := runBin(t, home, watchBin, "register", wfPath)
	if code != 0 {
		t.Fatalf("register exit code = %d, stderr: %s\nstdout: %s", code, stderr, out)
	}

	// Wait for watcher to start
	time.Sleep(500 * time.Millisecond)

	// First create a new file so fsbroker registers it in its watchmap
	// (on some platforms, pre-existing files may not be in the watchmap by inode)
	tempFile := filepath.Join(watchDir, "temp.txt")
	os.WriteFile(tempFile, []byte("setup"), 0644)
	time.Sleep(1 * time.Second)

	// Now modify the temp file — this should produce a Write event
	f, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	f.Write([]byte(" changed"))
	f.Sync()
	f.Close()

	deadline := time.After(10 * time.Second)
	for {
		data, err := os.ReadFile(resultFile)
		if err == nil && len(data) > 0 {
			if !strings.Contains(string(data), "modified") {
				t.Errorf("result file = %q, want 'modified'", string(data))
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for modified trigger to fire")
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestSuspendedTriggerDoesNotFire(t *testing.T) {
	watchBin, home, cleanup := startDaemonForTest(t, "fileevents")
	defer cleanup()

	wfDir := t.TempDir()
	watchDir := t.TempDir()
	resultFile := filepath.Join(wfDir, "result.txt")

	wfPath := filepath.Join(wfDir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte(fmt.Sprintf(`
Build:
  on:
    - type: FileCreated
      options:
        path: "%s"
  do:
    - Shell: "echo should-not-appear > %s"
`, watchDir, resultFile)), 0644)

	// Register
	runBin(t, home, watchBin, "register", wfPath)
	time.Sleep(500 * time.Millisecond)

	// Get trigger ID and suspend it
	out, _, _ := runBin(t, home, watchBin, "list", "triggers", "--json")
	var triggers []map[string]any
	if err := json.Unmarshal([]byte(out), &triggers); err != nil {
		t.Fatalf("parse triggers: %v", err)
	}
	if len(triggers) == 0 {
		t.Fatal("no triggers registered")
	}
	triggerID := triggers[0]["id"].(string)

	_, stderr, code := runBin(t, home, watchBin, "suspend", "trigger", triggerID)
	if code != 0 {
		t.Fatalf("suspend exit code = %d, stderr: %s", code, stderr)
	}

	time.Sleep(200 * time.Millisecond)

	// Create a file — action should NOT fire
	os.WriteFile(filepath.Join(watchDir, "test.txt"), []byte("hello"), 0644)
	time.Sleep(2 * time.Second)

	if _, err := os.Stat(resultFile); err == nil {
		data, _ := os.ReadFile(resultFile)
		t.Errorf("result file should not exist while trigger is suspended, contains: %s", string(data))
	}
}

