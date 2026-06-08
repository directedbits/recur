package triggerengine

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	domainaction "github.com/directedbits/recur/src/domain/action"
	domaintrigger "github.com/directedbits/recur/src/domain/trigger"
	executorsubprocess "github.com/directedbits/recur/src/infra/subprocess/executor"
)

// mockLookup implements TriggerLookup for testing.
type mockLookup struct {
	triggers map[string]*domaintrigger.Trigger
	actions  map[string][]*domainaction.Action // keyed by trigger ID
}

func (m *mockLookup) GetTrigger(id string) *domaintrigger.Trigger {
	return m.triggers[id]
}

func (m *mockLookup) GetActionsForTrigger(triggerID string) []*domainaction.Action {
	return m.actions[triggerID]
}

func TestNewEngine(t *testing.T) {
	lookup := &mockLookup{}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if len(e.active) != 0 {
		t.Errorf("expected empty watchers map, got %d", len(e.active))
	}
}

func TestActivateNoPath(t *testing.T) {
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)

	tr := &domaintrigger.Trigger{
		ID:      "trigger-no-path",
		Type:    "FileCreated",
		Options: map[string]any{},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	err := e.Activate(tr)
	if err == nil {
		t.Fatal("expected error for trigger with no path")
	}
}

func TestActivateWithExplicitPath(t *testing.T) {
	dir := t.TempDir()
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)

	tr := &domaintrigger.Trigger{
		ID:      "trigger-explicit",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	err := e.Activate(tr)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	if _, ok := e.active[tr.ID]; !ok {
		t.Error("expected watcher to be registered")
	}
}

func TestActivateWithRecurfilePath(t *testing.T) {
	dir := t.TempDir()
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)

	tr := &domaintrigger.Trigger{
		ID:            "trigger-wfpath",
		Type:          "FileModified",
		Options:       map[string]any{},
		RecurfilePath: filepath.Join(dir, "Recurfile.yaml"),
		Status:        domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	err := e.Activate(tr)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	if _, ok := e.active[tr.ID]; !ok {
		t.Error("expected trigger to be active")
	}
}

func TestActivateDuplicate(t *testing.T) {
	dir := t.TempDir()
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)

	tr := &domaintrigger.Trigger{
		ID:      "trigger-dup",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	if err := e.Activate(tr); err != nil {
		t.Fatalf("first activate: %v", err)
	}
	defer e.StopAll()

	// Second activate should be a no-op
	if err := e.Activate(tr); err != nil {
		t.Fatalf("second activate: %v", err)
	}
}

func TestDeactivate(t *testing.T) {
	dir := t.TempDir()
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)

	tr := &domaintrigger.Trigger{
		ID:      "trigger-deact",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	e.Activate(tr)
	e.Deactivate(tr.ID)

	if _, ok := e.active[tr.ID]; ok {
		t.Error("expected watcher to be removed after deactivate")
	}
}

func TestDeactivateNonexistent(t *testing.T) {
	lookup := &mockLookup{}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)

	// Should not panic
	e.Deactivate("nonexistent")
}

func TestStopAll(t *testing.T) {
	dir := t.TempDir()
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)

	for i := 0; i < 3; i++ {
		d := filepath.Join(dir, string(rune('a'+i)))
		os.Mkdir(d, 0755)
		tr := &domaintrigger.Trigger{
			ID:      "trigger-stop-" + string(rune('a'+i)),
			Type:    "FileCreated",
			Options: map[string]any{"path": d},
			Status:  domaintrigger.StatusActive,
		}
		lookup.triggers[tr.ID] = tr
		e.Activate(tr)
	}

	if len(e.active) != 3 {
		t.Fatalf("expected 3 watchers, got %d", len(e.active))
	}

	e.StopAll()

	if len(e.active) != 0 {
		t.Errorf("expected 0 watchers after StopAll, got %d", len(e.active))
	}
}

func TestActivateAfterStopAll(t *testing.T) {
	dir := t.TempDir()
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {}, testFileEventsFactory)
	e.StopAll()

	tr := &domaintrigger.Trigger{
		ID:      "trigger-after-stop",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	err := e.Activate(tr)
	if err == nil {
		t.Fatal("expected error activating after StopAll")
	}
}

func TestActivateUnknownTriggerType(t *testing.T) {
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
	}, testFileEventsFactory)

	tr := &domaintrigger.Trigger{
		ID:      "trigger-unknown",
		Type:    "SomethingUnknown",
		Options: map[string]any{"path": t.TempDir()},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	err := e.Activate(tr)
	if err == nil {
		e.StopAll()
		t.Fatal("expected error for unknown trigger type")
	}
}

func TestFileCreatedEvent(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var executedActions []string

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:      "trigger-filecreated-test",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-test-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		executedActions = append(executedActions, a.ID)
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Give the watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(dir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// Wait for the event to be processed
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(executedActions)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for file created event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(executedActions) == 0 {
		t.Error("expected at least one action to be executed")
	}
	if executedActions[0] != "action-test-1" {
		t.Errorf("executed action = %q, want %q", executedActions[0], "action-test-1")
	}
}

func TestFileModifiedEvent(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var modifyExecuted bool

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:      "trigger-filemodified-test",
		Type:    "FileModified",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-mod-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		modifyExecuted = true
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(200 * time.Millisecond)

	// Create the file first so fsbroker registers it in its watchmap
	testFile := filepath.Join(dir, "existing.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)

	// Wait for fsbroker to process the Create event and register the file
	time.Sleep(1 * time.Second)

	// Now modify the file — this should produce a Write event
	f, err := os.OpenFile(testFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	f.Write([]byte(" appended"))
	f.Sync()
	f.Close()

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		done := modifyExecuted
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for file modified event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestSuspendedTriggerSkipped(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var executed bool

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:      "trigger-suspended-test",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-susp-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		executed = true
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	// Suspend the trigger before creating a file
	tr.Status = domaintrigger.StatusSuspended

	// Create a file
	os.WriteFile(filepath.Join(dir, "should-be-ignored.txt"), []byte("hello"), 0644)

	// Wait a bit — the action should NOT fire
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if executed {
		t.Error("action should not have executed for suspended trigger")
	}
}

func TestSuspendedActionSkipped(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var executedIDs []string

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:      "trigger-actsusp-test",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	activeAct := &domainaction.Action{
		ID:     "action-active",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	suspendedAct := &domainaction.Action{
		ID:     "action-suspended",
		Type:   "Shell",
		Status: domainaction.StatusSuspended,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{activeAct, suspendedAct}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		executedIDs = append(executedIDs, a.ID)
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(executedIDs)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Wait a bit more for any additional actions
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Only the active action should have executed
	for _, id := range executedIDs {
		if id == "action-suspended" {
			t.Error("suspended action should not have been executed")
		}
	}
}

func TestEventContextVariables(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var capturedCtx *executorsubprocess.Context

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:      "trigger-ctx-test1",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-ctx-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		capturedCtx = execCtx
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	testFile := filepath.Join(dir, "context-test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		done := capturedCtx != nil
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if capturedCtx.Test {
		t.Error("Test should be false for real events")
	}
	if capturedCtx.Set["TriggerType"] != "FileCreated" {
		t.Errorf("TriggerType = %q, want %q", capturedCtx.Set["TriggerType"], "FileCreated")
	}
	if capturedCtx.Set["FilePath"] == "" {
		t.Error("FilePath should not be empty")
	}
	if capturedCtx.Set["TriggeredOn"] == "" {
		t.Error("TriggeredOn should not be empty")
	}
	if capturedCtx.Set["IsDirectory"] != "false" {
		t.Errorf("IsDirectory = %q, want %q", capturedCtx.Set["IsDirectory"], "false")
	}
}

func TestFilterMatchesGlob(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var capturedPaths []string

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:   "trigger-filter-test",
		Type: "FileCreated",
		Options: map[string]any{
			"path":   dir,
			"filter": []any{"*.txt"},
		},
		Status: domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-filter-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		capturedPaths = append(capturedPaths, execCtx.Set["FilePath"])
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	// Create a .log file (should NOT match *.txt filter)
	os.WriteFile(filepath.Join(dir, "ignored.log"), []byte("x"), 0644)
	// Create a .txt file (should match)
	os.WriteFile(filepath.Join(dir, "matched.txt"), []byte("x"), 0644)

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(capturedPaths)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for filtered file event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Wait a bit more for any additional events
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, p := range capturedPaths {
		if filepath.Base(p) == "ignored.log" {
			t.Error("filter should have excluded ignored.log")
		}
	}
}

func TestFilterNoPatterns_MatchesAll(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var executed bool

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:   "trigger-nofilter-test",
		Type: "FileCreated",
		Options: map[string]any{
			"path":   dir,
			"filter": []any{}, // empty filter = match all
		},
		Status: domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-nofilter-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		executed = true
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	os.WriteFile(filepath.Join(dir, "anything.xyz"), []byte("x"), 0644)

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		done := executed
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout — empty filter should match all files")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestFilterMultiplePatterns(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var capturedPaths []string

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:   "trigger-multifilter",
		Type: "FileCreated",
		Options: map[string]any{
			"path":   dir,
			"filter": []any{"*.go", "*.md"},
		},
		Status: domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-multifilter-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		capturedPaths = append(capturedPaths, execCtx.Set["FilePath"])
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("x"), 0644) // should NOT match

	// Wait for at least 2 events
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(capturedPaths)
		mu.Unlock()
		if count >= 2 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timeout — got %d events, want at least 2", len(capturedPaths))
			mu.Unlock()
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, p := range capturedPaths {
		base := filepath.Base(p)
		if base == "data.json" {
			t.Error("filter should have excluded data.json")
		}
	}
}

func TestFilterInvalidPattern(t *testing.T) {
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
	}, testFileEventsFactory)
	_ = e

	tr := &domaintrigger.Trigger{
		ID:   "trigger-badfilter",
		Type: "FileCreated",
		Options: map[string]any{
			"path":   t.TempDir(),
			"filter": []any{"[invalid"},
		},
		Status: domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	err := e.Activate(tr)
	if err == nil {
		e.StopAll()
		t.Fatal("expected error for invalid filter pattern")
	}
}

func TestEntityTypeFile_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var capturedPaths []string

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:   "trigger-entity-file",
		Type: "FileCreated",
		Options: map[string]any{
			"path":        dir,
			"entity_type": "file",
		},
		Status: domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-entity-file",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		capturedPaths = append(capturedPaths, execCtx.Set["FilePath"])
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	// Create a subdirectory (should be skipped)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	// Create a file (should match)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(capturedPaths)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for file event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, p := range capturedPaths {
		if filepath.Base(p) == "subdir" {
			t.Error("entity_type=file should have excluded directory creation")
		}
	}
}

func TestEntityTypeDirectory_SkipsFiles(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var capturedPaths []string

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:   "trigger-entity-dir",
		Type: "FileCreated",
		Options: map[string]any{
			"path":        dir,
			"entity_type": "directory",
		},
		Status: domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-entity-dir",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		capturedPaths = append(capturedPaths, execCtx.Set["FilePath"])
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	// Create a file first (should be skipped)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)
	// Create a subdirectory (should match)
	os.Mkdir(filepath.Join(dir, "newdir"), 0755)

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(capturedPaths)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for directory event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, p := range capturedPaths {
		if filepath.Base(p) == "file.txt" {
			t.Error("entity_type=directory should have excluded file creation")
		}
	}
}

func TestEntityTypeAll_MatchesBoth(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var capturedPaths []string

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:   "trigger-entity-all",
		Type: "FileCreated",
		Options: map[string]any{
			"path":        dir,
			"entity_type": "all",
		},
		Status: domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-entity-all",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		capturedPaths = append(capturedPaths, execCtx.Set["FilePath"])
		mu.Unlock()
	}, testFileEventsFactory)

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)
	os.Mkdir(filepath.Join(dir, "newdir"), 0755)

	// Wait for at least 2 events
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		count := len(capturedPaths)
		mu.Unlock()
		if count >= 2 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timeout — got %d events, want at least 2", len(capturedPaths))
			mu.Unlock()
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestOnFiredCallback(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	var firedCalled bool
	var actionExecuted bool

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
		actions:  map[string][]*domainaction.Action{},
	}

	tr := &domaintrigger.Trigger{
		ID:      "trigger-onfired-test",
		Type:    "FileCreated",
		Options: map[string]any{"path": dir},
		Status:  domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	act := &domainaction.Action{
		ID:     "action-onfired-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}
	lookup.actions[tr.ID] = []*domainaction.Action{act}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		actionExecuted = true
		mu.Unlock()
	}, testFileEventsFactory)

	e.SetOnFired(func() {
		mu.Lock()
		firedCalled = true
		mu.Unlock()
	})

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	time.Sleep(100 * time.Millisecond)

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		done := actionExecuted
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Give onFired a moment to execute (it runs after actions)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !firedCalled {
		t.Error("onFired callback was not called after trigger fired")
	}
}

func TestEntityTypeInvalid(t *testing.T) {
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{},
	}
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
	}, testFileEventsFactory)

	tr := &domaintrigger.Trigger{
		ID:   "trigger-entity-bad",
		Type: "FileCreated",
		Options: map[string]any{
			"path":        t.TempDir(),
			"entity_type": "symlink",
		},
		Status: domaintrigger.StatusActive,
	}
	lookup.triggers[tr.ID] = tr

	err := e.Activate(tr)
	if err == nil {
		e.StopAll()
		t.Fatal("expected error for invalid entity_type")
	}
}
