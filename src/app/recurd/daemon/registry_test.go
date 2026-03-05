package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pkgconfig "github.com/directedbits/recur/pkg/config"
	domainrf "github.com/directedbits/recur/src/domain/recurfile"
	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
)

func testRecurfileWithSecrets() *recurfileyaml.RawFile {
	f := testRecurfile()
	f.Secrets = []recurfileyaml.SecretDef{
		{Name: "db_password", Source: "env", Ref: "DB_PASSWORD"},
		{Name: "api_key", Source: "env", Ref: "API_KEY"},
	}
	return f
}

func testRecurfile() *recurfileyaml.RawFile {
	return &recurfileyaml.RawFile{
		Path: "/tmp/test/recur.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Build",
				Triggers: []recurfileyaml.RawTrigger{
					{
						Type:    "FileModified",
						Options: map[string]any{"path": "/src"},
						Actions: []recurfileyaml.RawAction{
							{Type: "Shell", Options: map[string]any{"command": "make build"}},
						},
					},
				},
			},
		},
	}
}

func testRecurfileMultiGroup() *recurfileyaml.RawFile {
	return &recurfileyaml.RawFile{
		Path: "/tmp/test/multi.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Lint",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "FileModified"},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell", Options: map[string]any{"command": "make lint"}},
				},
			},
			{
				Name: "Test",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "FileCreated"},
					{Type: "FileModified"},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell", Options: map[string]any{"command": "make test"}},
				},
			},
		},
	}
}

func TestRegistryRegisterRecurfile(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()

	result, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("registerRecurfile failed: %v", err)
	}

	if result.RecurfileID == "" {
		t.Error("expected non-empty recurfile ID")
	}
	if result.TriggerCount != 1 {
		t.Errorf("trigger count = %d, want 1", result.TriggerCount)
	}
	if result.ActionCount != 1 {
		t.Errorf("action count = %d, want 1", result.ActionCount)
	}

	// Verify stored entities
	if len(reg.recurfiles) != 1 {
		t.Errorf("recurfiles count = %d, want 1", len(reg.recurfiles))
	}
	if len(reg.groups) != 1 {
		t.Errorf("groups count = %d, want 1", len(reg.groups))
	}
	if len(reg.triggers) != 1 {
		t.Errorf("triggers count = %d, want 1", len(reg.triggers))
	}
	if len(reg.actions) != 1 {
		t.Errorf("actions count = %d, want 1", len(reg.actions))
	}
}

func TestRegistryRegisterMultipleGroups(t *testing.T) {
	reg := newRegistry()
	f := testRecurfileMultiGroup()

	result, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("registerRecurfile failed: %v", err)
	}

	if result.TriggerCount != 3 {
		t.Errorf("trigger count = %d, want 3", result.TriggerCount)
	}
	if result.ActionCount != 3 {
		t.Errorf("action count = %d, want 3", result.ActionCount)
	}
	if len(reg.groups) != 2 {
		t.Errorf("groups count = %d, want 2", len(reg.groups))
	}
}

func TestRegistryReloadRecurfile(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()

	result1, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if result1.Reloaded {
		t.Error("first registration should not be a reload")
	}

	// Register same path again — should succeed as reload
	result2, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("second register (reload) failed: %v", err)
	}
	if !result2.Reloaded {
		t.Error("second registration should be a reload")
	}
	if result2.RecurfileID != result1.RecurfileID {
		t.Error("recurfile ID should be stable across reload")
	}
	if len(result2.OldTriggerIDs) != 1 {
		t.Errorf("OldTriggerIDs count = %d, want 1", len(result2.OldTriggerIDs))
	}

	// Verify entity counts are correct (not doubled)
	if len(reg.recurfiles) != 1 {
		t.Errorf("recurfiles count = %d, want 1", len(reg.recurfiles))
	}
	if len(reg.groups) != 1 {
		t.Errorf("groups count = %d, want 1", len(reg.groups))
	}
	if len(reg.triggers) != 1 {
		t.Errorf("triggers count = %d, want 1", len(reg.triggers))
	}
	if len(reg.actions) != 1 {
		t.Errorf("actions count = %d, want 1", len(reg.actions))
	}
}

func TestRegistryReloadDetectsEquivalentPaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("Recurfile.yaml", []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Register via relative path, then via absolute path — should reload.
	reg := newRegistry()
	f1 := testRecurfile()
	f1.Path = "Recurfile.yaml"
	result1, err := reg.registerRecurfile(f1, nil, nil, nil)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	f2 := testRecurfile()
	f2.Path = filepath.Join(dir, "Recurfile.yaml")
	result2, err := reg.registerRecurfile(f2, nil, nil, nil)
	if err != nil {
		t.Fatalf("second register failed: %v", err)
	}

	if !result2.Reloaded {
		t.Error("expected second registration to be a reload (same file, different path forms)")
	}
	if result2.RecurfileID != result1.RecurfileID {
		t.Errorf("recurfile ID should be stable: got %q, want %q", result2.RecurfileID, result1.RecurfileID)
	}
	if len(reg.recurfiles) != 1 {
		t.Errorf("recurfiles count = %d, want 1 (no duplicate)", len(reg.recurfiles))
	}
}

func TestRegistryReloadWithChangedContent(t *testing.T) {
	reg := newRegistry()
	f1 := testRecurfile()

	_, err := reg.registerRecurfile(f1, nil, nil, nil)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	// Register same path with different content (more triggers)
	f2 := testRecurfileMultiGroup()
	f2.Path = f1.Path // same path

	result, err := reg.registerRecurfile(f2, nil, nil, nil)
	if err != nil {
		t.Fatalf("reload with changed content failed: %v", err)
	}
	if !result.Reloaded {
		t.Error("expected reload")
	}
	if result.TriggerCount != 3 {
		t.Errorf("trigger count = %d, want 3", result.TriggerCount)
	}
	if result.ActionCount != 3 {
		t.Errorf("action count = %d, want 3", result.ActionCount)
	}

	// Verify old entities were cleaned up
	if len(reg.triggers) != 3 {
		t.Errorf("triggers count = %d, want 3", len(reg.triggers))
	}
	if len(reg.actions) != 3 {
		t.Errorf("actions count = %d, want 3", len(reg.actions))
	}
}

func TestRegistryDeregisterRecurfile(t *testing.T) {
	reg := newRegistry()
	f := testRecurfileMultiGroup()

	result, _ := reg.registerRecurfile(f, nil, nil, nil)

	wf, triggersRemoved, actionsRemoved, err := reg.deregisterRecurfile(result.RecurfileID)
	if err != nil {
		t.Fatalf("deregisterRecurfile failed: %v", err)
	}

	if wf.FilePath != f.Path {
		t.Errorf("path = %q, want %q", wf.FilePath, f.Path)
	}
	if triggersRemoved != 3 {
		t.Errorf("triggers removed = %d, want 3", triggersRemoved)
	}
	if actionsRemoved != 3 {
		t.Errorf("actions removed = %d, want 3", actionsRemoved)
	}

	// Verify all entities removed
	if len(reg.recurfiles) != 0 {
		t.Errorf("recurfiles remaining = %d", len(reg.recurfiles))
	}
	if len(reg.groups) != 0 {
		t.Errorf("groups remaining = %d", len(reg.groups))
	}
	if len(reg.triggers) != 0 {
		t.Errorf("triggers remaining = %d", len(reg.triggers))
	}
	if len(reg.actions) != 0 {
		t.Errorf("actions remaining = %d", len(reg.actions))
	}
}

func TestRegistryDeregisterByPath(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()

	reg.registerRecurfile(f, nil, nil, nil)

	wf, _, _, err := reg.deregisterRecurfile(f.Path)
	if err != nil {
		t.Fatalf("deregister by path failed: %v", err)
	}
	if wf.FilePath != f.Path {
		t.Errorf("path = %q, want %q", wf.FilePath, f.Path)
	}
}

func TestRegistryAppliesPluginManifestDefaults(t *testing.T) {
	// A plugin declares debounce=0 in its manifestyaml. Recurfile doesn't set
	// debounce. Daemon default of 300ms is also passed in. The plugin
	// manifest should override the daemon default (less specific) but the
	// recurfile would still win if it had set one.
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/plugin-defaults.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "X",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "WidgetEvent"},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell", Options: map[string]any{"command": ":"}},
				},
			},
		},
	}
	plugins := []*pluginfs.InstalledPlugin{
		{
			ID:  "p1",
			Dir: "/tmp/test/plugin",
			Manifest: &manifestyaml.Manifest{
				Name:      "widget",
				Namespace: "test.widget",
				Version:   "0.0.1",
				Triggers: []manifestyaml.TriggerDef{
					{Name: "WidgetEvent", Defaults: map[string]any{"debounce": "0"}},
				},
			},
		},
	}
	daemonDefaults := map[string]any{"debounce": "300ms"}

	if _, err := reg.registerRecurfile(f, plugins, daemonDefaults, nil); err != nil {
		t.Fatalf("registerRecurfile: %v", err)
	}
	triggers := reg.listTriggers()
	if len(triggers) != 1 {
		t.Fatalf("triggers count = %d, want 1", len(triggers))
	}
	if triggers[0].Debounce != 0 {
		t.Errorf("Debounce = %v, want 0 (plugin manifest should override daemon default)", triggers[0].Debounce)
	}
}

func TestRegistryDaemonPluginOverrideBeatsManifest(t *testing.T) {
	// User sets plugins.test.widget.trigger_defaults.debounce in daemon
	// config — that should beat the plugin manifest's own default.
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/plugin-user-override.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "X",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "WidgetEvent"},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell", Options: map[string]any{"command": ":"}},
				},
			},
		},
	}
	plugins := []*pluginfs.InstalledPlugin{
		{
			ID:  "p1",
			Dir: "/tmp/test/plugin",
			Manifest: &manifestyaml.Manifest{
				Name:      "widget",
				Namespace: "test.widget",
				Version:   "0.0.1",
				Triggers: []manifestyaml.TriggerDef{
					{Name: "WidgetEvent", Defaults: map[string]any{"debounce": "0"}},
				},
			},
		},
	}
	pluginOverrides := map[string]map[string]any{
		"test.widget": {"debounce": "75ms"},
	}

	if _, err := reg.registerRecurfile(f, plugins, nil, pluginOverrides); err != nil {
		t.Fatalf("registerRecurfile: %v", err)
	}
	tr := reg.listTriggers()[0]
	if tr.Debounce != 75*time.Millisecond {
		t.Errorf("Debounce = %v, want 75ms (daemon plugin override should beat manifest)", tr.Debounce)
	}
}

func TestRegistryRecurfileBeatsDaemonPluginOverride(t *testing.T) {
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/recurfile-beats-plugin-override.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "X",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "WidgetEvent", Options: map[string]any{"debounce": "10ms"}},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell", Options: map[string]any{"command": ":"}},
				},
			},
		},
	}
	plugins := []*pluginfs.InstalledPlugin{
		{
			ID:  "p1",
			Dir: "/tmp/test/plugin",
			Manifest: &manifestyaml.Manifest{
				Name:      "widget",
				Namespace: "test.widget",
				Version:   "0.0.1",
				Triggers:  []manifestyaml.TriggerDef{{Name: "WidgetEvent"}},
			},
		},
	}
	pluginOverrides := map[string]map[string]any{
		"test.widget": {"debounce": "75ms"},
	}

	if _, err := reg.registerRecurfile(f, plugins, nil, pluginOverrides); err != nil {
		t.Fatalf("registerRecurfile: %v", err)
	}
	tr := reg.listTriggers()[0]
	if tr.Debounce != 10*time.Millisecond {
		t.Errorf("Debounce = %v, want 10ms (recurfile must win over daemon plugin override)", tr.Debounce)
	}
}

func TestRegistryRecurfileOverridesPluginDefaults(t *testing.T) {
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/recurfile-overrides-pluginfs.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "X",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "WidgetEvent", Options: map[string]any{"debounce": "50ms"}},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell", Options: map[string]any{"command": ":"}},
				},
			},
		},
	}
	plugins := []*pluginfs.InstalledPlugin{
		{
			ID:  "p1",
			Dir: "/tmp/test/plugin",
			Manifest: &manifestyaml.Manifest{
				Name:      "widget",
				Namespace: "test.widget",
				Version:   "0.0.1",
				Triggers: []manifestyaml.TriggerDef{
					{Name: "WidgetEvent", Defaults: map[string]any{"debounce": "0"}},
				},
			},
		},
	}

	if _, err := reg.registerRecurfile(f, plugins, nil, nil); err != nil {
		t.Fatalf("registerRecurfile: %v", err)
	}
	tr := reg.listTriggers()[0]
	if tr.Debounce != 50*time.Millisecond {
		t.Errorf("Debounce = %v, want 50ms (recurfile should win over plugin default)", tr.Debounce)
	}
}

func TestRegistryDeregisterNotFound(t *testing.T) {
	reg := newRegistry()

	_, _, _, err := reg.deregisterRecurfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent recurfile")
	}
}

func TestRegistryFindRecurfile(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	result, _ := reg.registerRecurfile(f, nil, nil, nil)

	// By ID
	wf := reg.findRecurfile(result.RecurfileID)
	if wf == nil {
		t.Fatal("findRecurfile by ID returned nil")
	}

	// By path
	wf = reg.findRecurfile(f.Path)
	if wf == nil {
		t.Fatal("findRecurfile by path returned nil")
	}

	// By ID prefix (first 4 chars)
	wf = reg.findRecurfile(result.RecurfileID[:4])
	if wf == nil {
		t.Fatal("findRecurfile by ID prefix returned nil")
	}

	// Not found
	wf = reg.findRecurfile("nonexistent")
	if wf != nil {
		t.Error("expected nil for nonexistent recurfile")
	}
}

func TestRegistryFindTrigger(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	triggers := reg.listTriggers()
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}

	tr := reg.findTrigger(triggers[0].ID)
	if tr == nil {
		t.Fatal("findTrigger by ID returned nil")
	}
	if tr.Type != "FileModified" {
		t.Errorf("type = %q, want %q", tr.Type, "FileModified")
	}

	// ID prefix
	tr = reg.findTrigger(triggers[0].ID[:4])
	if tr == nil {
		t.Fatal("findTrigger by prefix returned nil")
	}
}

func TestRegistryFindAction(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	actions := reg.listActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	a := reg.findAction(actions[0].ID)
	if a == nil {
		t.Fatal("findAction by ID returned nil")
	}
	if a.Type != "Shell" {
		t.Errorf("name = %q, want %q", a.Type, "Shell")
	}
}

func TestRegistryFindGroup(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	// By name
	g := reg.findGroup("Build")
	if g == nil {
		t.Fatal("findGroup by name returned nil")
	}

	// By ID
	g2 := reg.findGroup(g.ID)
	if g2 == nil {
		t.Fatal("findGroup by ID returned nil")
	}
	if g2.Name != "Build" {
		t.Errorf("name = %q, want %q", g2.Name, "Build")
	}
}

func TestRegistryNoActionsWarning(t *testing.T) {
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/noactions.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Empty",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "FileCreated"},
				},
			},
		},
	}

	result, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("registerRecurfile failed: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning about missing actions")
	}
}

func TestRegistryGroupLevelActions(t *testing.T) {
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/groupactions.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Build",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "FileModified"},
					{Type: "FileCreated"},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell", Options: map[string]any{"command": "make build"}},
				},
			},
		},
	}

	result, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("registerRecurfile failed: %v", err)
	}
	// 2 triggers, each gets the 1 group-level action = 2 actions total
	if result.ActionCount != 2 {
		t.Errorf("action count = %d, want 2", result.ActionCount)
	}
}

func TestRegistryGetActionsForTrigger(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	triggers := reg.listTriggers()
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}

	actions := reg.GetActionsForTrigger(triggers[0].ID)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "Shell" {
		t.Errorf("action name = %q, want %q", actions[0].Type, "Shell")
	}
}

func TestRegistryGetActionsForTriggerFiltersByTrigger(t *testing.T) {
	// Two triggers in the same group, each with its own per-trigger action.
	// GetActionsForTrigger must return only the actions belonging to the
	// queried trigger, not every action in the group.
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/multi-trigger.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Wildcards",
				Triggers: []recurfileyaml.RawTrigger{
					{
						Type:    "MessageReceived",
						Options: map[string]any{"topic": "home/+/temperature"},
						Actions: []recurfileyaml.RawAction{
							{Type: "Shell", Options: map[string]any{"command": "echo SINGLE"}},
						},
					},
					{
						Type:    "MessageReceived",
						Options: map[string]any{"topic": "logs/#"},
						Actions: []recurfileyaml.RawAction{
							{Type: "Shell", Options: map[string]any{"command": "echo MULTI"}},
						},
					},
				},
			},
		},
	}
	if _, err := reg.registerRecurfile(f, nil, nil, nil); err != nil {
		t.Fatalf("registerRecurfile failed: %v", err)
	}

	triggers := reg.listTriggers()
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}

	for _, tr := range triggers {
		actions := reg.GetActionsForTrigger(tr.ID)
		if len(actions) != 1 {
			t.Errorf("trigger %s: got %d actions, want 1", tr.ID, len(actions))
		}
		for _, a := range actions {
			if a.TriggerID != tr.ID {
				t.Errorf("trigger %s: action %s has TriggerID %s", tr.ID, a.ID, a.TriggerID)
			}
		}
	}
}

func TestRegistryGetActionsForTriggerNotFound(t *testing.T) {
	reg := newRegistry()
	actions := reg.GetActionsForTrigger("nonexistent")
	if actions != nil {
		t.Errorf("expected nil for nonexistent trigger, got %v", actions)
	}
}

func TestRegistryLookupActionByPrefix(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	actions := reg.listActions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	// By full ID
	a := reg.findAction(actions[0].ID)
	if a == nil {
		t.Fatal("findAction by full ID returned nil")
	}

	// By ID prefix (first 4 chars)
	a = reg.findAction(actions[0].ID[:4])
	if a == nil {
		t.Fatal("findAction by prefix returned nil")
	}

	// Not found
	a = reg.findAction("nonexistent")
	if a != nil {
		t.Error("expected nil for nonexistent action")
	}
}

func TestRegistryLookupGroupByPrefix(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	groups := reg.listGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	// By name
	g := reg.findGroup("Build")
	if g == nil {
		t.Fatal("findGroup by name returned nil")
	}

	// By ID prefix
	g = reg.findGroup(g.ID[:4])
	if g == nil {
		t.Fatal("findGroup by prefix returned nil")
	}

	// Not found
	g = reg.findGroup("nonexistent")
	if g != nil {
		t.Error("expected nil for nonexistent group")
	}
}

func TestRegistrySuspendAndResumeAction(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	actions := reg.listActions()
	actionID := actions[0].ID

	// Suspend
	id, name, alreadySuspended, err := reg.suspendAction(actionID)
	if err != nil {
		t.Fatalf("suspendAction failed: %v", err)
	}
	if id != actionID {
		t.Errorf("id = %q, want %q", id, actionID)
	}
	if name != "Shell" {
		t.Errorf("name = %q", name)
	}
	if alreadySuspended {
		t.Error("should not be already suspended")
	}

	// Suspend again
	_, _, alreadySuspended, _ = reg.suspendAction(actionID)
	if !alreadySuspended {
		t.Error("should be already suspended")
	}

	// Resume
	_, _, alreadyActive, err := reg.resumeAction(actionID)
	if err != nil {
		t.Fatalf("resumeAction failed: %v", err)
	}
	if alreadyActive {
		t.Error("should not be already active")
	}

	// Resume again
	_, _, alreadyActive, _ = reg.resumeAction(actionID)
	if !alreadyActive {
		t.Error("should be already active")
	}
}

func TestRegistrySuspendActionNotFound(t *testing.T) {
	reg := newRegistry()
	_, _, _, err := reg.suspendAction("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent action")
	}
}

func TestRegistryResumeActionNotFound(t *testing.T) {
	reg := newRegistry()
	_, _, _, err := reg.resumeAction("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent action")
	}
}

func TestRegistryResumeTriggerNotFound(t *testing.T) {
	reg := newRegistry()
	_, _, _, err := reg.resumeTrigger("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent trigger")
	}
}

func TestRegistrySnapshot(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	snaps := reg.snapshot()
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].FilePath != f.Path {
		t.Errorf("FilePath = %q, want %q", snaps[0].FilePath, f.Path)
	}
	if len(snaps[0].Triggers) != 1 {
		t.Errorf("trigger snapshots = %d, want 1", len(snaps[0].Triggers))
	}
	if len(snaps[0].Actions) != 1 {
		t.Errorf("action snapshots = %d, want 1", len(snaps[0].Actions))
	}
	// Default status should be "active"
	if snaps[0].Triggers[0].Status != "active" {
		t.Errorf("trigger status = %q, want %q", snaps[0].Triggers[0].Status, "active")
	}
}

func TestRegistrySnapshotEmpty(t *testing.T) {
	reg := newRegistry()
	snaps := reg.snapshot()
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snaps))
	}
}

func TestRegistryRestoreEntityStates(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	triggers := reg.listTriggers()
	actions := reg.listActions()
	triggerID := triggers[0].ID
	actionID := actions[0].ID

	// Get recurfile ID
	wfs := reg.listRecurfiles()
	wfID := wfs[0].ID

	triggerStates := map[string]entitySnapshot{
		triggerID: {
			ID:           triggerID,
			Status:       "suspended",
			ErrorCount:   3,
			LastActivity: "2025-01-15T10:30:00Z",
		},
	}
	actionStates := map[string]entitySnapshot{
		actionID: {
			ID:           actionID,
			Status:       "suspended",
			ErrorCount:   1,
			LastActivity: "2025-01-15T11:00:00Z",
		},
	}

	reg.restoreEntityStates(wfID, triggerStates, actionStates)

	// Verify trigger state
	tr := reg.findTrigger(triggerID)
	if string(tr.Status) != "suspended" {
		t.Errorf("trigger status = %q, want %q", tr.Status, "suspended")
	}
	if tr.ErrorCount != 3 {
		t.Errorf("trigger ErrorCount = %d, want 3", tr.ErrorCount)
	}
	if tr.LastFired.IsZero() {
		t.Error("trigger LastFired should not be zero")
	}

	// Verify action state
	a := reg.findAction(actionID)
	if string(a.Status) != "suspended" {
		t.Errorf("action status = %q, want %q", a.Status, "suspended")
	}
	if a.ErrorCount != 1 {
		t.Errorf("action ErrorCount = %d, want 1", a.ErrorCount)
	}
	if a.LastExecuted.IsZero() {
		t.Error("action LastExecuted should not be zero")
	}
}

func TestRegistryRestoreEntityStatesNoMatch(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	// Restore with wrong recurfile ID — should not change anything
	reg.restoreEntityStates("wrong-wf-id", map[string]entitySnapshot{}, map[string]entitySnapshot{})

	triggers := reg.listTriggers()
	if string(triggers[0].Status) != "active" {
		t.Errorf("trigger status should remain active, got %q", triggers[0].Status)
	}
}

// --- Feature 2: Group Option Inheritance ---

func TestGroupOptionsInheritedWhenTriggerHasNone(t *testing.T) {
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/inherit.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name:    "Build",
				Options: map[string]any{"path": "/src", "recursive": true},
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "FileModified"},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell", Options: map[string]any{"command": "make"}},
				},
			},
		},
	}

	result, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("registerRecurfile failed: %v", err)
	}
	if result.TriggerCount != 1 {
		t.Fatalf("expected 1 trigger, got %d", result.TriggerCount)
	}

	triggers := reg.listTriggers()
	opts := triggers[0].Options
	if opts["path"] != "/src" {
		t.Errorf("expected inherited path=/src, got %v", opts["path"])
	}
	if opts["recursive"] != true {
		t.Errorf("expected inherited recursive=true, got %v", opts["recursive"])
	}
}

func TestTriggerOptionsOverrideGroupOptions(t *testing.T) {
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/override.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name:    "Build",
				Options: map[string]any{"path": "/src", "recursive": true},
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "FileModified", Options: map[string]any{"path": "/custom"}},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell"},
				},
			},
		},
	}

	reg.registerRecurfile(f, nil, nil, nil)
	triggers := reg.listTriggers()
	opts := triggers[0].Options

	if opts["path"] != "/custom" {
		t.Errorf("expected trigger path=/custom to override group, got %v", opts["path"])
	}
	if opts["recursive"] != true {
		t.Errorf("expected inherited recursive=true, got %v", opts["recursive"])
	}
}

func TestOverlayMapsDifferentKeys(t *testing.T) {
	base := map[string]any{"a": 1}
	override := map[string]any{"b": 2}
	result := pkgconfig.OverlayMaps(base, override)
	if result["a"] != 1 || result["b"] != 2 {
		t.Errorf("expected merged {a:1, b:2}, got %v", result)
	}
}

func TestNoGroupOptionsTriggerOptionsUsed(t *testing.T) {
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/nogroup.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Build",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "FileModified", Options: map[string]any{"path": "/only"}},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell"},
				},
			},
		},
	}

	reg.registerRecurfile(f, nil, nil, nil)
	triggers := reg.listTriggers()
	opts := triggers[0].Options

	if opts["path"] != "/only" {
		t.Errorf("expected trigger-only path=/only, got %v", opts["path"])
	}
}

// --- Feature 3: Alias Resolution in Registration ---

func TestAliasExpansionInRegistration(t *testing.T) {
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path:    "/tmp/test/aliases.yaml",
		Aliases: map[string]string{"fs": "com.recur.filesystem"},
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Watch",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "fs.FileCreated", Options: map[string]any{"path": "/src"}},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell"},
				},
			},
		},
	}

	reg.registerRecurfile(f, nil, nil, nil)
	triggers := reg.listTriggers()

	if triggers[0].Type != "com.recur.filesystem.FileCreated" {
		t.Errorf("expected expanded type, got %q", triggers[0].Type)
	}
}

func TestAliasExpansionStableIDs(t *testing.T) {
	// Registering the same recurfile twice should produce stable IDs
	// because IDs use the qualified (alias-resolved) names deterministically.
	f := &recurfileyaml.RawFile{
		Path:    "/tmp/test/stable.yaml",
		Aliases: map[string]string{"fs": "com.recur.filesystem"},
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Watch",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "fs.FileCreated"},
				},
				Actions: []recurfileyaml.RawAction{
					{Type: "Shell"},
				},
			},
		},
	}

	reg1 := newRegistry()
	r1, _ := reg1.registerRecurfile(f, nil, nil, nil)
	t1 := reg1.listTriggers()[0].ID

	reg2 := newRegistry()
	r2, _ := reg2.registerRecurfile(f, nil, nil, nil)
	t2 := reg2.listTriggers()[0].ID

	if r1.RecurfileID != r2.RecurfileID {
		t.Error("recurfile IDs should be stable")
	}
	if t1 != t2 {
		t.Error("trigger IDs should be stable across registrations")
	}
}

func TestRegistryEntityIDDeterministic(t *testing.T) {
	id1 := domainrf.EntityID("trigger", "seed123")
	id2 := domainrf.EntityID("trigger", "seed123")
	if id1 != id2 {
		t.Errorf("IDs not deterministic: %q != %q", id1, id2)
	}

	id3 := domainrf.EntityID("action", "seed123")
	if id1 == id3 {
		t.Error("different entity types should produce different IDs")
	}

	if len(id1) != 12 {
		t.Errorf("ID length = %d, want 12", len(id1))
	}
}

// --- resolveEntity tests ---

func TestResolveEntity_NoFilter(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	triggers := reg.listTriggers()
	refs := reg.resolveEntity(triggers[0].ID)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].EntityType != "trigger" {
		t.Errorf("type = %q, want trigger", refs[0].EntityType)
	}
}

func TestResolveEntity_WithTypeFilter(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	triggers := reg.listTriggers()

	// Match with correct type
	refs := reg.resolveEntity(triggers[0].ID, "trigger")
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}

	// No match with wrong type
	refs = reg.resolveEntity(triggers[0].ID, "action")
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for wrong type, got %d", len(refs))
	}
}

func TestResolveEntity_MultipleTypes(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	triggers := reg.listTriggers()
	actions := reg.listActions()

	// Filter for trigger+action types: should find both
	refs := reg.resolveEntity(triggers[0].ID, "trigger", "action")
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref for trigger ID with trigger+action filter, got %d", len(refs))
	}
	if refs[0].EntityType != "trigger" {
		t.Errorf("type = %q, want trigger", refs[0].EntityType)
	}

	refs = reg.resolveEntity(actions[0].ID, "trigger", "action")
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref for action ID with trigger+action filter, got %d", len(refs))
	}
	if refs[0].EntityType != "action" {
		t.Errorf("type = %q, want action", refs[0].EntityType)
	}
}

func TestResolveEntity_NotFound(t *testing.T) {
	reg := newRegistry()
	refs := reg.resolveEntity("nonexistent")
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}

func TestResolveEntity_AmbiguousByName(t *testing.T) {
	// Create a recurfile where a trigger and action share the same name
	reg := newRegistry()
	f := &recurfileyaml.RawFile{
		Path: "/tmp/test/ambiguous.yaml",
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "Build",
				Triggers: []recurfileyaml.RawTrigger{
					{
						Type: "Shell",
						Name: "Shell",
						Actions: []recurfileyaml.RawAction{
							{Type: "Shell", Name: "Shell"},
						},
					},
				},
			},
		},
	}
	reg.registerRecurfile(f, nil, nil, nil)

	// "Shell" matches both a trigger and an action by name
	refs := reg.resolveEntity("Shell")
	if len(refs) < 2 {
		t.Fatalf("expected >= 2 refs for ambiguous name 'Shell', got %d", len(refs))
	}

	// Filter to just triggers
	refs = reg.resolveEntity("Shell", "trigger")
	if len(refs) != 1 {
		t.Fatalf("expected 1 trigger ref, got %d", len(refs))
	}
	if refs[0].EntityType != "trigger" {
		t.Errorf("type = %q, want trigger", refs[0].EntityType)
	}
}

func TestResolveEntity_ByNameWithFilter(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()
	reg.registerRecurfile(f, nil, nil, nil)

	// "Build" is a group name
	refs := reg.resolveEntity("Build", "group")
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].EntityType != "group" {
		t.Errorf("type = %q, want group", refs[0].EntityType)
	}

	// "Build" filtered to trigger type should return nothing
	refs = reg.resolveEntity("Build", "trigger")
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for Build as trigger, got %d", len(refs))
	}
}

// --- Secrets storage on recurfile ---

func TestRegistryStoresSecretDefs(t *testing.T) {
	reg := newRegistry()
	f := testRecurfileWithSecrets()

	result, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("registerRecurfile failed: %v", err)
	}

	defs := reg.secretDefsForRecurfile(result.RecurfileID)
	if len(defs) != 2 {
		t.Fatalf("expected 2 secret defs, got %d", len(defs))
	}
	if defs[0].Name != "db_password" || defs[0].Source != "env" {
		t.Errorf("secret[0] = %+v", defs[0])
	}
	if defs[1].Name != "api_key" || defs[1].Source != "env" {
		t.Errorf("secret[1] = %+v", defs[1])
	}
}

func TestRegistryNoSecretDefs(t *testing.T) {
	reg := newRegistry()
	f := testRecurfile()

	result, err := reg.registerRecurfile(f, nil, nil, nil)
	if err != nil {
		t.Fatalf("registerRecurfile failed: %v", err)
	}

	defs := reg.secretDefsForRecurfile(result.RecurfileID)
	if defs != nil {
		t.Errorf("expected nil secret defs, got %v", defs)
	}
}

func TestRegistrySecretDefsNotFound(t *testing.T) {
	reg := newRegistry()
	defs := reg.secretDefsForRecurfile("nonexistent")
	if defs != nil {
		t.Errorf("expected nil for nonexistent recurfile, got %v", defs)
	}
}

func TestRegistrySecretsPreservedOnReload(t *testing.T) {
	reg := newRegistry()
	f := testRecurfileWithSecrets()

	result1, _ := reg.registerRecurfile(f, nil, nil, nil)

	// Reload with updated secrets
	f2 := testRecurfileWithSecrets()
	f2.Secrets = []recurfileyaml.SecretDef{
		{Name: "new_secret", Source: "file", Ref: "/etc/secret"},
	}
	result2, err := reg.registerRecurfile(f2, nil, nil, nil)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if result1.RecurfileID != result2.RecurfileID {
		t.Error("recurfile ID should be stable on reload")
	}

	defs := reg.secretDefsForRecurfile(result2.RecurfileID)
	if len(defs) != 1 {
		t.Fatalf("expected 1 secret def after reload, got %d", len(defs))
	}
	if defs[0].Name != "new_secret" {
		t.Errorf("expected new_secret, got %q", defs[0].Name)
	}
}
