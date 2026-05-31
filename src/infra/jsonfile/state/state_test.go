package statejsonfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadNonexistent(t *testing.T) {
	f, err := Load("/tmp/nonexistent/state.json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(f.Recurfiles) != 0 {
		t.Errorf("expected empty recurfiles, got %d", len(f.Recurfiles))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "state.json")

	original := &File{
		Recurfiles: []RecurfileState{
			{
				ID:       "abc123def456",
				FilePath: "/home/user/project/recur.yaml",
				Triggers: []EntityState{
					{ID: "trig01", Status: "active"},
					{ID: "trig02", Status: "suspended", ErrorCount: 3},
				},
				Actions: []EntityState{
					{ID: "act01", Status: "active"},
				},
			},
		},
	}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Recurfiles) != 1 {
		t.Fatalf("expected 1 recurfile, got %d", len(loaded.Recurfiles))
	}

	wf := loaded.Recurfiles[0]
	if wf.ID != "abc123def456" {
		t.Errorf("ID = %q, want %q", wf.ID, "abc123def456")
	}
	if wf.FilePath != "/home/user/project/recur.yaml" {
		t.Errorf("FilePath = %q", wf.FilePath)
	}
	if len(wf.Triggers) != 2 {
		t.Fatalf("triggers = %d, want 2", len(wf.Triggers))
	}
	if wf.Triggers[1].Status != "suspended" {
		t.Errorf("trigger status = %q, want suspended", wf.Triggers[1].Status)
	}
	if wf.Triggers[1].ErrorCount != 3 {
		t.Errorf("error count = %d, want 3", wf.Triggers[1].ErrorCount)
	}
}

func TestSaveAtomicity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	f := &File{
		Recurfiles: []RecurfileState{
			{ID: "test1", FilePath: "/test"},
		},
	}

	if err := Save(f, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify no temp file remains
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful save")
	}

	// Verify file exists and is valid JSON
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.Recurfiles) != 1 {
		t.Errorf("expected 1 recurfile, got %d", len(loaded.Recurfiles))
	}
}

func TestSaveCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "state.json")

	f := &File{}
	if err := Save(f, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to exist")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte("not valid json"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	f := &File{}
	if err := Save(f, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.Recurfiles) != 0 {
		t.Errorf("expected nil or empty recurfiles, got %d", len(loaded.Recurfiles))
	}
}

func TestSaveOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Save first version
	f1 := &File{
		Recurfiles: []RecurfileState{
			{ID: "old", FilePath: "/old"},
		},
	}
	Save(f1, path)

	// Overwrite with new version
	f2 := &File{
		Recurfiles: []RecurfileState{
			{ID: "new", FilePath: "/new"},
		},
	}
	Save(f2, path)

	loaded, _ := Load(path)
	if len(loaded.Recurfiles) != 1 {
		t.Fatalf("expected 1 recurfile, got %d", len(loaded.Recurfiles))
	}
	if loaded.Recurfiles[0].ID != "new" {
		t.Errorf("ID = %q, want %q", loaded.Recurfiles[0].ID, "new")
	}
}

func TestLastActivityPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := &File{
		Recurfiles: []RecurfileState{
			{
				ID:       "wf1",
				FilePath: "/test/recur.yaml",
				Triggers: []EntityState{
					{ID: "t1", Status: "active", LastActivity: "2026-03-17T20:00:00Z"},
				},
				Actions: []EntityState{
					{ID: "a1", Status: "active", LastActivity: "2026-03-17T20:01:00Z"},
				},
			},
		},
	}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Recurfiles[0].Triggers[0].LastActivity != "2026-03-17T20:00:00Z" {
		t.Errorf("trigger LastActivity = %q, want 2026-03-17T20:00:00Z",
			loaded.Recurfiles[0].Triggers[0].LastActivity)
	}
	if loaded.Recurfiles[0].Actions[0].LastActivity != "2026-03-17T20:01:00Z" {
		t.Errorf("action LastActivity = %q, want 2026-03-17T20:01:00Z",
			loaded.Recurfiles[0].Actions[0].LastActivity)
	}
}

func TestLastActivityOmittedWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := &File{
		Recurfiles: []RecurfileState{
			{
				ID:       "wf1",
				FilePath: "/test/recur.yaml",
				Triggers: []EntityState{
					{ID: "t1", Status: "active"},
				},
			},
		},
	}

	Save(original, path)

	data, _ := os.ReadFile(path)
	json := string(data)
	if contains(json, "last_activity") {
		t.Error("last_activity should be omitted when empty")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMultipleRecurfiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	f := &File{
		Recurfiles: []RecurfileState{
			{
				ID: "wf1", FilePath: "/project1/recur.yaml",
				Triggers: []EntityState{{ID: "t1", Status: "active"}},
				Actions:  []EntityState{{ID: "a1", Status: "active"}},
			},
			{
				ID: "wf2", FilePath: "/project2/recur.yaml",
				Triggers: []EntityState{{ID: "t2", Status: "suspended"}},
				Actions:  []EntityState{{ID: "a2", Status: "error", ErrorCount: 5}},
			},
		},
	}

	Save(f, path)
	loaded, _ := Load(path)

	if len(loaded.Recurfiles) != 2 {
		t.Fatalf("expected 2 recurfiles, got %d", len(loaded.Recurfiles))
	}
	if loaded.Recurfiles[1].Actions[0].ErrorCount != 5 {
		t.Errorf("error count = %d, want 5", loaded.Recurfiles[1].Actions[0].ErrorCount)
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join(".config", "recur", "state", "state.json")) {
		t.Errorf("DefaultPath = %q, want suffix .config/recur/state/state.json", path)
	}
}

func TestRecover_NoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte(`{"recurfiles":[]}`), 0644)

	if err := Recover(path); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}
	// File should be unchanged
	data, _ := os.ReadFile(path)
	if string(data) != `{"recurfiles":[]}` {
		t.Errorf("data changed: %q", string(data))
	}
}

func TestRecover_PromotesOrphanedTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte("old"), 0644)

	// Create orphaned temp
	tmpPath := filepath.Join(dir, "state-20260320T120000.000.json.tmp")
	os.WriteFile(tmpPath, []byte("recovered"), 0644)

	if err := Recover(path); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "recovered" {
		t.Errorf("data = %q, want %q", string(data), "recovered")
	}
}

func TestRecover_NonexistentDir(t *testing.T) {
	err := Recover("/tmp/nonexistent-state-dir-99999/state.json")
	if err != nil {
		t.Fatalf("Recover should not error on missing dir: %v", err)
	}
}

func TestLoadReadPermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte(`{"recurfiles":[]}`), 0644)
	os.Chmod(path, 0000)
	t.Cleanup(func() { os.Chmod(path, 0644) })

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
}

func TestLaunchArgsSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := &File{
		LaunchArgs: &LaunchArgs{
			ConfigPath:    "/home/user/.config/recur/config.yaml",
			SocketAddress: "/tmp/recur.sock",
			LogLevel:      "debug",
			Foreground:    false,
		},
		Recurfiles: []RecurfileState{
			{ID: "wf1", FilePath: "/test/recur.yaml"},
		},
	}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.LaunchArgs == nil {
		t.Fatal("expected LaunchArgs, got nil")
	}
	if loaded.LaunchArgs.ConfigPath != "/home/user/.config/recur/config.yaml" {
		t.Errorf("ConfigPath = %q", loaded.LaunchArgs.ConfigPath)
	}
	if loaded.LaunchArgs.SocketAddress != "/tmp/recur.sock" {
		t.Errorf("SocketAddress = %q", loaded.LaunchArgs.SocketAddress)
	}
	if loaded.LaunchArgs.LogLevel != "debug" {
		t.Errorf("LogLevel = %q", loaded.LaunchArgs.LogLevel)
	}
	if loaded.LaunchArgs.Foreground {
		t.Error("Foreground should be false")
	}

	// Verify recurfiles are still intact
	if len(loaded.Recurfiles) != 1 || loaded.Recurfiles[0].ID != "wf1" {
		t.Errorf("recurfiles not preserved: %+v", loaded.Recurfiles)
	}
}

func TestLaunchArgsOmittedWhenNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	f := &File{
		Recurfiles: []RecurfileState{
			{ID: "wf1", FilePath: "/test/recur.yaml"},
		},
	}

	Save(f, path)

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "launch_args") {
		t.Error("launch_args should be omitted when nil")
	}
}

func TestLoadLaunchArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Save state with launch args
	f := &File{
		LaunchArgs: &LaunchArgs{
			ConfigPath: "/etc/recur/config.yaml",
			LogLevel:   "warn",
		},
	}
	Save(f, path)

	args, err := LoadLaunchArgs(path)
	if err != nil {
		t.Fatalf("LoadLaunchArgs failed: %v", err)
	}
	if args == nil {
		t.Fatal("expected args, got nil")
	}
	if args.ConfigPath != "/etc/recur/config.yaml" {
		t.Errorf("ConfigPath = %q", args.ConfigPath)
	}
	if args.LogLevel != "warn" {
		t.Errorf("LogLevel = %q", args.LogLevel)
	}
}

func TestLoadLaunchArgsNonexistent(t *testing.T) {
	args, err := LoadLaunchArgs("/tmp/nonexistent/state.json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if args != nil {
		t.Errorf("expected nil args for nonexistent file, got: %+v", args)
	}
}

func TestLoadLaunchArgsNilWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Save state without launch args
	f := &File{Recurfiles: []RecurfileState{{ID: "wf1", FilePath: "/test"}}}
	Save(f, path)

	args, err := LoadLaunchArgs(path)
	if err != nil {
		t.Fatalf("LoadLaunchArgs failed: %v", err)
	}
	if args != nil {
		t.Errorf("expected nil args, got: %+v", args)
	}
}

func TestLaunchArgsForeground(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	f := &File{
		LaunchArgs: &LaunchArgs{
			ConfigPath: "/test/config.yaml",
			Foreground: true,
		},
	}
	Save(f, path)

	loaded, _ := Load(path)
	if !loaded.LaunchArgs.Foreground {
		t.Error("Foreground should be true")
	}
}

func TestLaunchArgsBackwardCompat(t *testing.T) {
	// Simulate loading a state file that was written before LaunchArgs existed
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte(`{"recurfiles":[{"id":"wf1","file_path":"/old"}]}`), 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.LaunchArgs != nil {
		t.Errorf("expected nil LaunchArgs for old state file, got: %+v", loaded.LaunchArgs)
	}
	if len(loaded.Recurfiles) != 1 {
		t.Fatalf("expected 1 recurfile, got %d", len(loaded.Recurfiles))
	}
}
