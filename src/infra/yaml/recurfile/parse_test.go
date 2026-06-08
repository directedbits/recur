package recurfileyaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMinimalValid(t *testing.T) {
	yaml := `
My Group:
  on:
    - type: FileCreated
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(f.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(f.Groups))
	}
	if f.Groups[0].Name != "My Group" {
		t.Errorf("group name = %q, want %q", f.Groups[0].Name, "My Group")
	}
	if len(f.Groups[0].Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(f.Groups[0].Triggers))
	}
	if f.Groups[0].Triggers[0].Type != "FileCreated" {
		t.Errorf("trigger type = %q, want %q", f.Groups[0].Triggers[0].Type, "FileCreated")
	}
}

func TestParseFullRecurfile(t *testing.T) {
	yaml := `
aliases:
  fs: com.example.filesystem

My Triggers:
  aliases:
    sh: com.example.shell
  options:
    recursive: true
    filter:
      - "*.md"
  on:
    - type: FileCreated
    - type: FileModified
      options:
        recursive: false
      do:
        - Shell: "echo modified"
  do:
    - type: Shell
      options:
        command: "cat {{.FilePath}}"
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Top-level aliases
	if f.Aliases["fs"] != "com.example.filesystem" {
		t.Errorf("alias fs = %q, want %q", f.Aliases["fs"], "com.example.filesystem")
	}

	g := f.Groups[0]

	// Group aliases
	if g.Aliases["sh"] != "com.example.shell" {
		t.Errorf("group alias sh = %q, want %q", g.Aliases["sh"], "com.example.shell")
	}

	// Group options
	if g.Options["recursive"] != true {
		t.Errorf("group option recursive = %v, want true", g.Options["recursive"])
	}

	// Triggers
	if len(g.Triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(g.Triggers))
	}
	if g.Triggers[0].Type != "FileCreated" {
		t.Errorf("trigger[0] type = %q, want %q", g.Triggers[0].Type, "FileCreated")
	}
	if g.Triggers[1].Type != "FileModified" {
		t.Errorf("trigger[1] type = %q, want %q", g.Triggers[1].Type, "FileModified")
	}

	// Trigger-level options override
	if g.Triggers[1].Options["recursive"] != false {
		t.Errorf("trigger[1] recursive = %v, want false", g.Triggers[1].Options["recursive"])
	}

	// Trigger-level actions
	if len(g.Triggers[1].Actions) != 1 {
		t.Fatalf("trigger[1] expected 1 action, got %d", len(g.Triggers[1].Actions))
	}
	if g.Triggers[1].Actions[0].Type != "Shell" {
		t.Errorf("trigger[1] action type = %q, want %q", g.Triggers[1].Actions[0].Type, "Shell")
	}

	// Group-level default actions (detailed form)
	if len(g.Actions) != 1 {
		t.Fatalf("expected 1 group action, got %d", len(g.Actions))
	}
	if g.Actions[0].Type != "Shell" {
		t.Errorf("group action type = %q, want %q", g.Actions[0].Type, "Shell")
	}
	if g.Actions[0].Options["command"] != "cat {{.FilePath}}" {
		t.Errorf("group action command = %v, want %q", g.Actions[0].Options["command"], "cat {{.FilePath}}")
	}
}

func TestParseMultipleGroups(t *testing.T) {
	yaml := `
Group A:
  on:
    - type: FileCreated

Group B:
  on:
    - type: FileDeleted
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(f.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(f.Groups))
	}
	if f.Groups[0].Name != "Group A" {
		t.Errorf("group[0] name = %q, want %q", f.Groups[0].Name, "Group A")
	}
	if f.Groups[1].Name != "Group B" {
		t.Errorf("group[1] name = %q, want %q", f.Groups[1].Name, "Group B")
	}
}

func TestParseShorthandAction(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  do:
    - Shell: "echo hello"
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actions := f.Groups[0].Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "Shell" {
		t.Errorf("action type = %q, want %q", actions[0].Type, "Shell")
	}
	if actions[0].Options["_shorthand"] != "echo hello" {
		t.Errorf("shorthand value = %v, want %q", actions[0].Options["_shorthand"], "echo hello")
	}
}

func TestParseShorthandActionMapping(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  do:
    - ContainerStop:
        container: "my-container"
        timeout: 5
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actions := f.Groups[0].Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "ContainerStop" {
		t.Errorf("action type = %q, want %q", actions[0].Type, "ContainerStop")
	}
	if _, hasShorthand := actions[0].Options["_shorthand"]; hasShorthand {
		t.Error("mapping shorthand should not produce _shorthand key")
	}
	if actions[0].Options["container"] != "my-container" {
		t.Errorf("options[container] = %v, want %q", actions[0].Options["container"], "my-container")
	}
	if actions[0].Options["timeout"] != 5 {
		t.Errorf("options[timeout] = %v, want 5", actions[0].Options["timeout"])
	}
}

func TestParseQualifiedShorthandAction(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  do:
    - com.example.slack: "#general Build done"
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actions := f.Groups[0].Actions
	if actions[0].Type != "com.example.slack" {
		t.Errorf("action type = %q, want %q", actions[0].Type, "com.example.slack")
	}
}

func TestParseMixedActionForms(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  do:
    - Shell: "echo hello"
    - type: notify
      options:
        message: "done"
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actions := f.Groups[0].Actions
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].Type != "Shell" {
		t.Errorf("action[0] type = %q, want %q", actions[0].Type, "Shell")
	}
	if actions[1].Type != "notify" {
		t.Errorf("action[1] type = %q, want %q", actions[1].Type, "notify")
	}
}

func TestParseErrorNoGroups(t *testing.T) {
	yaml := `
aliases:
  fs: com.example.filesystem
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for recurfile with no groups")
	}
}

func TestParseErrorMissingTriggers(t *testing.T) {
	yaml := `
Test:
  do:
    - Shell: "echo hello"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for group without triggers")
	}
	if !strings.Contains(err.Error(), "missing required key: on") {
		t.Errorf("error should mention 'on' key, got: %v", err)
	}
}

func TestParseTriggersKeyAsSynonym(t *testing.T) {
	yaml := `
Test:
  triggers:
    - type: DeviceConnected
  do:
    - Shell: "echo hello"
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse should accept 'triggers' as synonym for 'on': %v", err)
	}
	if len(f.Groups[0].Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(f.Groups[0].Triggers))
	}
	if f.Groups[0].Triggers[0].Type != "DeviceConnected" {
		t.Errorf("trigger type = %q, want %q", f.Groups[0].Triggers[0].Type, "DeviceConnected")
	}
}

func TestParseErrorEmptyTriggers(t *testing.T) {
	yaml := `
Test:
  on: []
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty triggers list")
	}
}

func TestParseErrorTriggerMissingType(t *testing.T) {
	yaml := `
Test:
  on:
    - options:
        recursive: true
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for trigger without type")
	}
}

func TestParseErrorUnknownGroupKey(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  unknown_key: value
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for unknown group key")
	}
}

func TestParseErrorUnknownTriggerKey(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
      badkey: value
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for unknown trigger key")
	}
}

func TestParseErrorReservedActionKey(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  do:
    - options: "this is invalid"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for reserved key used as action name")
	}
}

func TestParseErrorShorthandMultipleKeys(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  do:
    - Shell: "echo a"
      notify: "done"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for shorthand action with multiple keys")
	}
}

func TestParseErrorDetailedActionMixedKeys(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  do:
    - type: Shell
      Shell: "echo conflicting"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for detailed action with extra shorthand key")
	}
}

func TestParseErrorInvalidYAML(t *testing.T) {
	_, err := Parse([]byte(":::bad yaml\n  [unclosed"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseErrorTopLevelScalar(t *testing.T) {
	_, err := Parse([]byte("just a string"))
	if err == nil {
		t.Fatal("expected error for scalar top-level")
	}
}

func TestParseTriggerNoActions(t *testing.T) {
	// Trigger with no do at either level is allowed (warning at runtime)
	yaml := `
Test:
  on:
    - type: FileCreated
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(f.Groups[0].Actions) != 0 {
		t.Errorf("expected no group actions")
	}
	if len(f.Groups[0].Triggers[0].Actions) != 0 {
		t.Errorf("expected no trigger actions")
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recurfile.yaml")
	content := `
Build:
  on:
    - type: FileModified
  do:
    - Shell: "make build"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if f.Path != path {
		t.Errorf("path = %q, want %q", f.Path, path)
	}
	if len(f.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(f.Groups))
	}
}

func TestLoadResolvesRelativeFileSecretPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recurfile.yaml")
	content := `
secrets:
  rel_token: !file ./token.secret
  abs_token: !file /etc/absolute/token

Build:
  on:
    - type: FileModified
  do:
    - Shell: "true"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	wantRel := filepath.Join(dir, "token.secret")
	var gotRel, gotAbs string
	for _, s := range f.Secrets {
		switch s.Name {
		case "rel_token":
			gotRel = s.Ref
		case "abs_token":
			gotAbs = s.Ref
		}
	}
	if gotRel != wantRel {
		t.Errorf("rel_token Ref = %q, want %q", gotRel, wantRel)
	}
	if gotAbs != "/etc/absolute/token" {
		t.Errorf("abs_token Ref = %q, want absolute path unchanged", gotAbs)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/recurfile.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(":::bad"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
}

func TestParseNamespacedTriggerType(t *testing.T) {
	yaml := `
Test:
  on:
    - type: fs.FileCreated
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if f.Groups[0].Triggers[0].Type != "fs.FileCreated" {
		t.Errorf("trigger type = %q, want %q", f.Groups[0].Triggers[0].Type, "fs.FileCreated")
	}
}

func TestParseGroupOptionsWithList(t *testing.T) {
	yaml := `
Test:
  options:
    filter:
      - "*.go"
      - "*.md"
  on:
    - type: FileCreated
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	filter, ok := f.Groups[0].Options["filter"].([]any)
	if !ok {
		t.Fatalf("filter is not a list: %T", f.Groups[0].Options["filter"])
	}
	if len(filter) != 2 {
		t.Errorf("filter has %d items, want 2", len(filter))
	}
}

func TestParseYAMLAnchor_Action(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
  do:
    - &notify
      type: Shell
      options:
        command: "echo notification"
    - *notify
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	actions := f.Groups[0].Actions
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].Type != "Shell" {
		t.Errorf("action[0] type = %q, want %q", actions[0].Type, "Shell")
	}
	if actions[1].Type != "Shell" {
		t.Errorf("action[1] type = %q, want %q", actions[1].Type, "Shell")
	}
	if actions[1].Options["command"] != "echo notification" {
		t.Errorf("action[1] command = %v, want %q", actions[1].Options["command"], "echo notification")
	}
}

func TestParseYAMLAnchor_Trigger(t *testing.T) {
	yaml := `
Test:
  on:
    - &watcher
      type: FileCreated
      options:
        path: /data
    - *watcher
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	triggers := f.Groups[0].Triggers
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0].Type != "FileCreated" {
		t.Errorf("trigger[0] type = %q, want %q", triggers[0].Type, "FileCreated")
	}
	if triggers[1].Type != "FileCreated" {
		t.Errorf("trigger[1] type = %q, want %q", triggers[1].Type, "FileCreated")
	}
	if triggers[1].Options["path"] != "/data" {
		t.Errorf("trigger[1] path = %v, want %q", triggers[1].Options["path"], "/data")
	}
}

func TestParseYAMLAnchor_CrossGroup(t *testing.T) {
	yaml := `
Group A:
  on:
    - type: FileCreated
  do:
    - &shared_action
      type: Shell
      options:
        command: "make build"

Group B:
  on:
    - type: FileModified
  do:
    - *shared_action
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(f.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(f.Groups))
	}

	actionsA := f.Groups[0].Actions
	actionsB := f.Groups[1].Actions
	if len(actionsA) != 1 || len(actionsB) != 1 {
		t.Fatalf("expected 1 action per group, got %d and %d", len(actionsA), len(actionsB))
	}
	if actionsA[0].Type != "Shell" {
		t.Errorf("group A action type = %q", actionsA[0].Type)
	}
	if actionsB[0].Type != "Shell" {
		t.Errorf("group B action type = %q", actionsB[0].Type)
	}
	if actionsB[0].Options["command"] != "make build" {
		t.Errorf("group B action command = %v", actionsB[0].Options["command"])
	}
}

func TestParseYAMLAnchor_Options(t *testing.T) {
	yaml := `
Test:
  on:
    - type: FileCreated
      options: &watch_opts
        path: /data
        recursive: true
    - type: FileModified
      options: *watch_opts
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	triggers := f.Groups[0].Triggers
	if triggers[1].Options["path"] != "/data" {
		t.Errorf("trigger[1] path = %v", triggers[1].Options["path"])
	}
	if triggers[1].Options["recursive"] != true {
		t.Errorf("trigger[1] recursive = %v", triggers[1].Options["recursive"])
	}
}

func TestParseSecretsEnvVar(t *testing.T) {
	yaml := `
secrets:
  db_password: ${DB_PASSWORD}
  optional_key: ${MAYBE_SET:-fallback_value}
  required_key: ${MUST_SET:?API key is required}

Build:
  on:
    - type: Cron
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(f.Secrets) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(f.Secrets))
	}

	// Plain env var
	s := f.Secrets[0]
	if s.Name != "db_password" || s.Source != "env" || s.Ref != "DB_PASSWORD" {
		t.Errorf("secret[0] = %+v", s)
	}
	if s.Default != "" || s.Required {
		t.Errorf("secret[0] should have no default and not be required: %+v", s)
	}

	// Env var with default
	s = f.Secrets[1]
	if s.Name != "optional_key" || s.Source != "env" || s.Ref != "MAYBE_SET" {
		t.Errorf("secret[1] = %+v", s)
	}
	if s.Default != "fallback_value" {
		t.Errorf("secret[1] default = %q, want %q", s.Default, "fallback_value")
	}

	// Env var required
	s = f.Secrets[2]
	if s.Name != "required_key" || s.Source != "env" || s.Ref != "MUST_SET" {
		t.Errorf("secret[2] = %+v", s)
	}
	if !s.Required || s.ErrorMsg != "API key is required" {
		t.Errorf("secret[2] required=%v, errorMsg=%q", s.Required, s.ErrorMsg)
	}
}

func TestParseSecretsFileSource(t *testing.T) {
	yaml := `
secrets:
  api_key: !file /etc/recur/secrets/api-key

Build:
  on:
    - type: Cron
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(f.Secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(f.Secrets))
	}
	s := f.Secrets[0]
	if s.Name != "api_key" || s.Source != "file" || s.Ref != "/etc/recur/secrets/api-key" {
		t.Errorf("secret = %+v", s)
	}
}

func TestParseSecretsKeyringSource(t *testing.T) {
	yaml := `
secrets:
  wifi_pass: !keyring recur/wifi_password

Build:
  on:
    - type: Cron
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(f.Secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(f.Secrets))
	}
	s := f.Secrets[0]
	if s.Name != "wifi_pass" || s.Source != "keyring" || s.Ref != "recur/wifi_password" {
		t.Errorf("secret = %+v", s)
	}
}

func TestParseSecretsErrorInvalidFormat(t *testing.T) {
	yaml := `
secrets:
  bad: plaintext_value

Build:
  on:
    - type: Cron
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid secret format")
	}
	if !strings.Contains(err.Error(), "unrecognized secret format") {
		t.Errorf("error = %q, expected to mention unrecognized format", err.Error())
	}
}

func TestParseSecretsErrorKeyringNoSlash(t *testing.T) {
	yaml := `
secrets:
  bad: !keyring just_a_key

Build:
  on:
    - type: Cron
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for keyring without slash")
	}
	if !strings.Contains(err.Error(), "service/key format") {
		t.Errorf("error = %q, expected service/key format message", err.Error())
	}
}

func TestParseSecretsErrorFileEmpty(t *testing.T) {
	yaml := `
secrets:
  bad: !file ""

Build:
  on:
    - type: Cron
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty file path")
	}
	if !strings.Contains(err.Error(), "requires a file path") {
		t.Errorf("error = %q, expected file path message", err.Error())
	}
}

func TestParseSecretsMixedSources(t *testing.T) {
	yaml := `
secrets:
  db_password: ${DB_PASSWORD}
  api_key: !file /etc/recur/secrets/api-key
  wifi_pass: !keyring recur/wifi_password

Build:
  on:
    - type: Cron
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(f.Secrets) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(f.Secrets))
	}
	if f.Secrets[0].Source != "env" {
		t.Errorf("secret[0] source = %q, want env", f.Secrets[0].Source)
	}
	if f.Secrets[1].Source != "file" {
		t.Errorf("secret[1] source = %q, want file", f.Secrets[1].Source)
	}
	if f.Secrets[2].Source != "keyring" {
		t.Errorf("secret[2] source = %q, want keyring", f.Secrets[2].Source)
	}
}

func TestParseNoSecretsSection(t *testing.T) {
	yaml := `
Build:
  on:
    - type: Cron
`
	f, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if f.Secrets != nil {
		t.Errorf("expected nil secrets, got %v", f.Secrets)
	}
}
