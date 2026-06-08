package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
)

func testPlugins() []*pluginfs.InstalledPlugin {
	return []*pluginfs.InstalledPlugin{
		{
			Manifest: &manifestyaml.Manifest{
				Namespace: "core.timer",
				Triggers:  []manifestyaml.TriggerDef{{Name: "Cron"}, {Name: "Interval"}},
			},
		},
		{
			Manifest: &manifestyaml.Manifest{
				Namespace: "core.fileevents",
				Triggers:  []manifestyaml.TriggerDef{{Name: "FileCreated"}, {Name: "FileChanged"}},
			},
		},
		{
			Manifest: &manifestyaml.Manifest{
				Namespace: "core.notify",
				Actions:   []manifestyaml.ActionDef{{Name: "Notify"}},
			},
		},
	}
}

func TestValidateKnownTypes_AllKnown(t *testing.T) {
	err := validateKnownTypes(
		[]string{"Cron", "FileCreated"},
		[]string{"Notify", "shell"},
		testPlugins(),
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateKnownTypes_CaseInsensitive(t *testing.T) {
	err := validateKnownTypes([]string{"cron"}, []string{"notify"}, testPlugins())
	if err != nil {
		t.Fatalf("expected case-insensitive match, got: %v", err)
	}
}

func TestValidateKnownTypes_UnknownTriggerWithSuggestion(t *testing.T) {
	err := validateKnownTypes([]string{"Crom"}, nil, testPlugins())
	if err == nil {
		t.Fatal("expected error for unknown trigger")
	}
	if !strings.Contains(err.Error(), "unknown trigger type \"Crom\"") {
		t.Errorf("missing unknown-trigger message: %v", err)
	}
	if !strings.Contains(err.Error(), "Cron") {
		t.Errorf("expected 'Cron' suggestion in: %v", err)
	}
}

func TestValidateKnownTypes_UnknownActionNoSuggestion(t *testing.T) {
	err := validateKnownTypes(nil, []string{"xyzzy"}, testPlugins())
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if !strings.Contains(err.Error(), "unknown action type \"xyzzy\"") {
		t.Errorf("missing unknown-action message: %v", err)
	}
	if strings.Contains(err.Error(), "did you mean") {
		t.Errorf("did not expect suggestions for 'xyzzy', got: %v", err)
	}
}

func TestValidateKnownTypes_PluginNameInsteadOfTrigger(t *testing.T) {
	// User types plugin name "timer" instead of the actual trigger name
	err := validateKnownTypes([]string{"timer"}, nil, testPlugins())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown trigger type \"timer\"") {
		t.Errorf("missing error message: %v", err)
	}
}

func TestValidateKnownTypes_MultipleErrors(t *testing.T) {
	err := validateKnownTypes([]string{"BadTrig"}, []string{"BadAct"}, testPlugins())
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BadTrig") || !strings.Contains(msg, "BadAct") {
		t.Errorf("expected both unknowns reported: %v", err)
	}
}

func TestResolveAddTarget_Local(t *testing.T) {
	path, err := resolveAddTarget(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
	if filepath.Base(path) != "Recurfile.yaml" {
		t.Errorf("expected recur.yaml, got %q", filepath.Base(path))
	}
}

func TestResolveAddTarget_User(t *testing.T) {
	path, err := resolveAddTarget(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(path, filepath.Join(".config", "recur")) {
		t.Errorf("expected path to contain .config/recur, got %q", path)
	}
	if filepath.Base(path) != "Recurfile.yaml" {
		t.Errorf("expected recur.yaml, got %q", filepath.Base(path))
	}
}

func TestResolveGroupName_Default(t *testing.T) {
	tests := []struct {
		args      []string
		userScope bool
		expected  string
	}{
		{nil, false, "Local"},
		{nil, true, "User"},
		{[]string{"DeviceConnected"}, false, "Local"},  // single arg is trigger type, not group name
		{[]string{"DeviceConnected"}, true, "User"},     // single arg is trigger type, not group name
		{[]string{"MyGroup", "Cron"}, false, "MyGroup"}, // two args: first is group name
		{[]string{"Custom", "Cron"}, true, "Custom"},
	}

	for _, tt := range tests {
		got := resolveGroupName(tt.args, tt.userScope)
		if got != tt.expected {
			t.Errorf("resolveGroupName(%v, %v) = %q, want %q", tt.args, tt.userScope, got, tt.expected)
		}
	}
}

func TestCollectTriggers(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		triggersFlag string
		expected     []string
	}{
		{"no args or flag", nil, "", nil},
		{"single arg is trigger type", []string{"Cron"}, "", []string{"Cron"}},
		{"two args, second is trigger", []string{"group", "Cron"}, "", []string{"Cron"}},
		{"flag only", nil, "Cron,FileChanged", []string{"Cron", "FileChanged"}},
		{"two args plus flag", []string{"group", "Cron"}, "FileChanged", []string{"Cron", "FileChanged"}},
		{"single arg plus flag", []string{"Cron"}, "FileChanged", []string{"Cron", "FileChanged"}},
		{"flag with spaces", nil, " Cron , FileChanged ", []string{"Cron", "FileChanged"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectTriggers(tt.args, tt.triggersFlag)
			if len(got) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"one", []string{"one"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , ", []string{"a", "b"}},
	}

	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitCSV(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestMergeFragment_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "Recurfile.yaml")

	fragment := "MyGroup:\n  on:\n    - type: Cron\n  do:\n    - type: Shell\n"

	result, err := mergeFragment(path, fragment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != fragment {
		t.Errorf("expected fragment as-is for new file, got:\n%s", result)
	}
}

func TestMergeFragment_AppendNewGroup(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "Recurfile.yaml")

	existing := "Existing:\n  on:\n    - type: Cron\n  do:\n    - type: Shell\n"
	os.WriteFile(path, []byte(existing), 0644)

	fragment := "NewGroup:\n  on:\n    - type: FileChanged\n  do:\n    - type: Shell\n"

	result, err := mergeFragment(path, fragment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Existing:") {
		t.Error("expected existing group to be preserved")
	}
	if !strings.Contains(result, "NewGroup:") {
		t.Error("expected new group to be appended")
	}
}

func TestMergeFragment_MergeExistingGroup(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "Recurfile.yaml")

	existing := "MyGroup:\n  on:\n    - type: Cron\n  do:\n    - type: Shell\n"
	os.WriteFile(path, []byte(existing), 0644)

	fragment := "MyGroup:\n  on:\n    - type: FileChanged\n"

	result, err := mergeFragment(path, fragment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Cron") {
		t.Error("expected original Cron trigger to be preserved")
	}
	if !strings.Contains(result, "FileChanged") {
		t.Error("expected new FileChanged trigger to be merged")
	}
}

func TestMergeFragment_EmptyExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "Recurfile.yaml")

	os.WriteFile(path, []byte(""), 0644)

	fragment := "MyGroup:\n  on:\n    - type: Cron\n  do:\n    - type: Shell\n"

	result, err := mergeFragment(path, fragment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != fragment {
		t.Errorf("expected fragment for empty file, got:\n%s", result)
	}
}

func TestRunAdd_FlagParsing(t *testing.T) {
	cmd := newAddCmd()
	// Verify all flags exist
	flags := []string{"local", "user", "triggers", "actions", "edit", "stub"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag %q to be defined", f)
		}
	}
}

func TestRunAdd_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "Recurfile.yaml")

	// Simulate what runAdd does without editor/daemon
	fragment := "TestGroup:\n  on:\n    - type: Cron\n  do:\n    - type: Shell\n"

	merged, err := mergeFragment(targetPath, fragment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.WriteFile(targetPath, []byte(merged), 0644); err != nil {
		t.Fatalf("could not write file: %v", err)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("could not read file: %v", err)
	}

	if !strings.Contains(string(data), "TestGroup:") {
		t.Error("expected TestGroup in written file")
	}
}
