package recurfile

import "testing"

func TestResolveFileExpandsTriggerType(t *testing.T) {
	f := &RawFile{
		Path:    "/tmp/test.yaml",
		Aliases: map[string]string{"fs": "com.recur.filesystem"},
		Groups: []RawGroup{
			{
				Name: "Watch",
				Triggers: []RawTrigger{
					{Type: "fs.FileCreated"},
				},
			},
		},
	}

	Resolve(f)

	if f.Groups[0].Triggers[0].Type != "com.recur.filesystem.FileCreated" {
		t.Errorf("trigger type = %q, want %q", f.Groups[0].Triggers[0].Type, "com.recur.filesystem.FileCreated")
	}
}

func TestResolveFileExpandsActionType(t *testing.T) {
	f := &RawFile{
		Path:    "/tmp/test.yaml",
		Aliases: map[string]string{"sh": "com.recur.shell"},
		Groups: []RawGroup{
			{
				Name: "Build",
				Triggers: []RawTrigger{
					{
						Type: "FileModified",
						Actions: []RawAction{
							{Type: "sh.Execute"},
						},
					},
				},
			},
		},
	}

	Resolve(f)

	if f.Groups[0].Triggers[0].Actions[0].Type != "com.recur.shell.Execute" {
		t.Errorf("action type = %q, want %q", f.Groups[0].Triggers[0].Actions[0].Type, "com.recur.shell.Execute")
	}
}

func TestResolveFileExpandsGroupActionTypes(t *testing.T) {
	f := &RawFile{
		Path:    "/tmp/test.yaml",
		Aliases: map[string]string{"sh": "com.recur.shell"},
		Groups: []RawGroup{
			{
				Name: "Build",
				Triggers: []RawTrigger{
					{Type: "FileModified"},
				},
				Actions: []RawAction{
					{Type: "sh.Execute"},
				},
			},
		},
	}

	Resolve(f)

	// Group-level actions should be resolved
	if f.Groups[0].Actions[0].Type != "com.recur.shell.Execute" {
		t.Errorf("group action type = %q, want %q", f.Groups[0].Actions[0].Type, "com.recur.shell.Execute")
	}
	// Trigger should get a copy of the resolved group actions
	if len(f.Groups[0].Triggers[0].Actions) != 1 {
		t.Fatalf("expected 1 trigger action, got %d", len(f.Groups[0].Triggers[0].Actions))
	}
	if f.Groups[0].Triggers[0].Actions[0].Type != "com.recur.shell.Execute" {
		t.Errorf("inherited action type = %q, want %q", f.Groups[0].Triggers[0].Actions[0].Type, "com.recur.shell.Execute")
	}
}

func TestResolveFileMergesGroupOptionsIntoTrigger(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name:    "Build",
				Options: map[string]any{"path": "/src", "recursive": true},
				Triggers: []RawTrigger{
					{Type: "FileModified", Options: map[string]any{"path": "/custom"}},
				},
				Actions: []RawAction{{Type: "Shell"}},
			},
		},
	}

	Resolve(f)

	opts := f.Groups[0].Triggers[0].Options
	if opts["path"] != "/custom" {
		t.Errorf("path = %v, want /custom (trigger overrides group)", opts["path"])
	}
	if opts["recursive"] != true {
		t.Errorf("recursive = %v, want true (inherited from group)", opts["recursive"])
	}
}

func TestResolveFileMergesGroupOptionsIntoInlineAction(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name:    "Bridge",
				Options: map[string]any{"broker": "tcp://group:1883", "qos": "1"},
				Triggers: []RawTrigger{
					{
						Type: "MessageReceived",
						Actions: []RawAction{
							{Type: "Publish", Options: map[string]any{"qos": "2"}},
						},
					},
				},
			},
		},
	}

	Resolve(f)

	opts := f.Groups[0].Triggers[0].Actions[0].Options
	if opts["broker"] != "tcp://group:1883" {
		t.Errorf("broker = %v, want tcp://group:1883 (inherited from group)", opts["broker"])
	}
	if opts["qos"] != "2" {
		t.Errorf("qos = %v, want 2 (action overrides group)", opts["qos"])
	}
}

func TestResolveFileMergesGroupOptionsIntoInheritedAction(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name:    "Bridge",
				Options: map[string]any{"broker": "tcp://group:1883"},
				Triggers: []RawTrigger{
					{Type: "MessageReceived"},
				},
				Actions: []RawAction{
					{Type: "Publish", Options: map[string]any{"_shorthand": "status/pong"}},
				},
			},
		},
	}

	Resolve(f)

	if len(f.Groups[0].Triggers[0].Actions) != 1 {
		t.Fatalf("expected 1 inherited action, got %d", len(f.Groups[0].Triggers[0].Actions))
	}
	opts := f.Groups[0].Triggers[0].Actions[0].Options
	if opts["broker"] != "tcp://group:1883" {
		t.Errorf("broker = %v, want tcp://group:1883 (inherited from group)", opts["broker"])
	}
	if opts["_shorthand"] != "status/pong" {
		t.Errorf("_shorthand = %v, want status/pong (preserved through merge)", opts["_shorthand"])
	}
}

func TestResolveFileMergeAcrossTriggersDoesNotAlias(t *testing.T) {
	// Two triggers in one group both inherit the same group-level action.
	// After resolveFile, mutating one trigger's action options must not
	// leak into the other trigger's action options or the group's source
	// action.
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name:    "Bridge",
				Options: map[string]any{"broker": "tcp://group:1883"},
				Triggers: []RawTrigger{
					{Type: "MessageReceived"},
					{Type: "MessageReceived"},
				},
				Actions: []RawAction{
					{Type: "Publish", Options: map[string]any{"topic": "out"}},
				},
			},
		},
	}

	Resolve(f)

	a0 := f.Groups[0].Triggers[0].Actions[0].Options
	a1 := f.Groups[0].Triggers[1].Actions[0].Options
	src := f.Groups[0].Actions[0].Options

	a0["topic"] = "mutated"

	if a1["topic"] != "out" {
		t.Errorf("trigger[1] action topic = %v, want out (got aliased to trigger[0])", a1["topic"])
	}
	if src["topic"] != "out" {
		t.Errorf("group source action topic = %v, want out (got aliased to trigger[0])", src["topic"])
	}
}

func TestResolveFileMergeKeepsShorthand(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name:    "Bridge",
				Options: map[string]any{"broker": "tcp://group:1883"},
				Triggers: []RawTrigger{
					{
						Type: "MessageReceived",
						Actions: []RawAction{
							{Type: "publish", Options: map[string]any{"_shorthand": "status/pong"}},
						},
					},
				},
			},
		},
	}

	Resolve(f)

	opts := f.Groups[0].Triggers[0].Actions[0].Options
	if opts["_shorthand"] != "status/pong" {
		t.Errorf("_shorthand = %v, want status/pong", opts["_shorthand"])
	}
	if opts["broker"] != "tcp://group:1883" {
		t.Errorf("broker = %v, want tcp://group:1883 (inherited from group)", opts["broker"])
	}
}

func TestResolveFileExpandsOptionKeys(t *testing.T) {
	f := &RawFile{
		Path:    "/tmp/test.yaml",
		Aliases: map[string]string{"fs": "com.recur.filesystem"},
		Groups: []RawGroup{
			{
				Name:    "Watch",
				Options: map[string]any{"fs.path": "/src"},
				Triggers: []RawTrigger{
					{Type: "fs.FileCreated", Options: map[string]any{"fs.recursive": true}},
				},
				Actions: []RawAction{{Type: "Shell"}},
			},
		},
	}

	Resolve(f)

	opts := f.Groups[0].Triggers[0].Options
	if _, ok := opts["com.recur.filesystem.path"]; !ok {
		t.Error("expected expanded group option key com.recur.filesystem.path")
	}
	if _, ok := opts["com.recur.filesystem.recursive"]; !ok {
		t.Error("expected expanded trigger option key com.recur.filesystem.recursive")
	}
}

func TestResolveFileGroupAliasOverridesFileAlias(t *testing.T) {
	f := &RawFile{
		Path:    "/tmp/test.yaml",
		Aliases: map[string]string{"fs": "com.recur.filesystem"},
		Groups: []RawGroup{
			{
				Name:    "Watch",
				Aliases: map[string]string{"fs": "com.other.fs"},
				Triggers: []RawTrigger{
					{Type: "fs.FileCreated"},
				},
				Actions: []RawAction{{Type: "Shell"}},
			},
		},
	}

	Resolve(f)

	if f.Groups[0].Triggers[0].Type != "com.other.fs.FileCreated" {
		t.Errorf("type = %q, want %q", f.Groups[0].Triggers[0].Type, "com.other.fs.FileCreated")
	}
}

func TestResolveFileTriggerActionsOverrideGroupActions(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name: "Build",
				Triggers: []RawTrigger{
					{
						Type: "FileModified",
						Actions: []RawAction{
							{Type: "TriggerAction"},
						},
					},
				},
				Actions: []RawAction{
					{Type: "GroupAction"},
				},
			},
		},
	}

	Resolve(f)

	actions := f.Groups[0].Triggers[0].Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "TriggerAction" {
		t.Errorf("action = %q, want %q", actions[0].Type, "TriggerAction")
	}
}

func TestResolveFileNoAliasesNoOp(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name: "Build",
				Triggers: []RawTrigger{
					{Type: "FileModified", Options: map[string]any{"path": "/src"}},
				},
				Actions: []RawAction{{Type: "Shell"}},
			},
		},
	}

	Resolve(f)

	if f.Groups[0].Triggers[0].Type != "FileModified" {
		t.Errorf("type changed unexpectedly: %q", f.Groups[0].Triggers[0].Type)
	}
}

func TestResolveFileIsIdempotent(t *testing.T) {
	f := &RawFile{
		Path:    "/tmp/test.yaml",
		Aliases: map[string]string{"fs": "com.recur.filesystem"},
		Groups: []RawGroup{
			{
				Name:    "Watch",
				Options: map[string]any{"path": "/src"},
				Triggers: []RawTrigger{
					{Type: "fs.FileCreated", Options: map[string]any{"recursive": true}},
				},
				Actions: []RawAction{{Type: "Shell"}},
			},
		},
	}

	Resolve(f)
	firstType := f.Groups[0].Triggers[0].Type
	firstOpts := len(f.Groups[0].Triggers[0].Options)

	Resolve(f)
	secondType := f.Groups[0].Triggers[0].Type
	secondOpts := len(f.Groups[0].Triggers[0].Options)

	if firstType != secondType {
		t.Errorf("type changed on second resolve: %q → %q", firstType, secondType)
	}
	if firstOpts != secondOpts {
		t.Errorf("option count changed on second resolve: %d → %d", firstOpts, secondOpts)
	}
}

// --- validateFile tests ---

func TestValidateFileNoWarningsWhenActionsPresent(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name: "Build",
				Triggers: []RawTrigger{
					{
						Type:    "FileModified",
						Actions: []RawAction{{Type: "Shell"}},
					},
				},
			},
		},
	}

	warnings := Validate(f)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestValidateFileWarnsOnNoActions(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name: "Empty",
				Triggers: []RawTrigger{
					{Type: "FileCreated"},
				},
			},
		},
	}

	warnings := Validate(f)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0] != `group "Empty", trigger "FileCreated": no actions defined` {
		t.Errorf("unexpected warning: %q", warnings[0])
	}
}

func TestValidateFileMultipleWarnings(t *testing.T) {
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name: "G1",
				Triggers: []RawTrigger{
					{Type: "T1"},
					{Type: "T2"},
				},
			},
			{
				Name: "G2",
				Triggers: []RawTrigger{
					{Type: "T3", Actions: []RawAction{{Type: "Shell"}}},
					{Type: "T4"},
				},
			},
		},
	}

	warnings := Validate(f)
	if len(warnings) != 3 {
		t.Errorf("expected 3 warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateFileAfterResolveWithGroupActions(t *testing.T) {
	// After resolveFile, triggers that inherited group actions should have no warnings
	f := &RawFile{
		Path: "/tmp/test.yaml",
		Groups: []RawGroup{
			{
				Name: "Build",
				Triggers: []RawTrigger{
					{Type: "FileModified"},
				},
				Actions: []RawAction{{Type: "Shell"}},
			},
		},
	}

	Resolve(f)
	warnings := Validate(f)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings after resolve (group actions inherited), got %v", warnings)
	}
}
