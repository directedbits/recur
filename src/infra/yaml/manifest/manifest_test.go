package manifestyaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifest = `
name: filesystem
namespace: com.example.filesystem
version: "1.0.0"
description: "File system event triggers"
dependencies:
  - inotify-tools

configuration:
  - key: poll_interval
    type: number
    default: 5
  - key: follow_symlinks
    type: bool
    default: false

triggers:
  - name: FileCreated
    description: "Fires when a file is created"
    options:
      - name: recursive
        type: bool
        default: false
      - name: filter
        type: list
        default: []
    context:
      - name: FilePath
        type: string
      - name: TriggeredOn
        type: string

actions:
  - name: Shell
    description: "Execute a shell command"
    options:
      - name: command
        type: string
        shorthand: true
      - name: shell
        type: string
        default: "sh"
`

func TestParseValidManifest(t *testing.T) {
	m, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if m.Name != "filesystem" {
		t.Errorf("Name = %q, want %q", m.Name, "filesystem")
	}
	if m.Namespace != "com.example.filesystem" {
		t.Errorf("Namespace = %q, want %q", m.Namespace, "com.example.filesystem")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if m.Description != "File system event triggers" {
		t.Errorf("Description = %q", m.Description)
	}
	if len(m.Dependencies) != 1 || m.Dependencies[0] != "inotify-tools" {
		t.Errorf("Dependencies = %v", m.Dependencies)
	}
	if len(m.Configuration) != 2 {
		t.Errorf("Configuration count = %d, want 2", len(m.Configuration))
	}
	if len(m.Triggers) != 1 {
		t.Fatalf("Triggers count = %d, want 1", len(m.Triggers))
	}
	if m.Triggers[0].Name != "FileCreated" {
		t.Errorf("Trigger name = %q", m.Triggers[0].Name)
	}
	if len(m.Triggers[0].Options) != 2 {
		t.Errorf("Trigger options count = %d, want 2", len(m.Triggers[0].Options))
	}
	if len(m.Triggers[0].Context) != 2 {
		t.Errorf("Trigger context count = %d, want 2", len(m.Triggers[0].Context))
	}
	if len(m.Actions) != 1 {
		t.Fatalf("Actions count = %d, want 1", len(m.Actions))
	}
	if m.Actions[0].Name != "Shell" {
		t.Errorf("Action name = %q", m.Actions[0].Name)
	}
	if !m.Actions[0].Options[0].Shorthand {
		t.Error("command option should have shorthand=true")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(path, []byte(validManifest), 0644)

	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if m.Name != "filesystem" {
		t.Errorf("Name = %q, want %q", m.Name, "filesystem")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/manifest.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMissingName(t *testing.T) {
	yaml := `
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options: []
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q, want mention of 'name is required'", err)
	}
}

func TestMissingNamespace(t *testing.T) {
	yaml := `
name: test
version: "1.0.0"
triggers:
  - name: Foo
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing namespace")
	}
	if !strings.Contains(err.Error(), "namespace is required") {
		t.Errorf("error = %q", err)
	}
}

func TestMissingVersion(t *testing.T) {
	yaml := `
name: test
namespace: com.example
triggers:
  - name: Foo
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing version")
	}
	if !strings.Contains(err.Error(), "version is required") {
		t.Errorf("error = %q", err)
	}
}

func TestNoTriggersOrActions(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for no triggers or actions")
	}
	if !strings.Contains(err.Error(), "at least one trigger or action") {
		t.Errorf("error = %q", err)
	}
}

func TestActionsOnlyIsValid(t *testing.T) {
	yaml := `
name: shell
namespace: com.example.shell
version: "1.0.0"
actions:
  - name: Shell
    options:
      - name: command
        type: string
        shorthand: true
`
	m, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("expected actions-only manifest to be valid: %v", err)
	}
	if len(m.Actions) != 1 {
		t.Errorf("Actions count = %d, want 1", len(m.Actions))
	}
}

func TestTriggersOnlyIsValid(t *testing.T) {
	yaml := `
name: filesystem
namespace: com.example.filesystem
version: "1.0.0"
triggers:
  - name: FileCreated
    options:
      - name: recursive
        type: bool
`
	_, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("expected triggers-only manifest to be valid: %v", err)
	}
}

func TestInvalidOptionType(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - name: bar
        type: integer
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid option type")
	}
	if !strings.Contains(err.Error(), `invalid type "integer"`) {
		t.Errorf("error = %q", err)
	}
}

func TestInvalidConfigType(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
configuration:
  - key: foo
    type: float
triggers:
  - name: Foo
    options:
      - name: x
        type: string
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid config type")
	}
	if !strings.Contains(err.Error(), `invalid type "float"`) {
		t.Errorf("error = %q", err)
	}
}

func TestInvalidContextType(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    context:
      - name: Bar
        type: object
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid context type")
	}
	if !strings.Contains(err.Error(), `invalid type "object"`) {
		t.Errorf("error = %q", err)
	}
}

func TestMissingOptionName(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - type: string
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing option name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err)
	}
}

func TestMissingTriggerName(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - options:
      - name: x
        type: string
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing trigger name")
	}
	if !strings.Contains(err.Error(), "triggers[0]: name is required") {
		t.Errorf("error = %q", err)
	}
}

func TestTriggerDefaults_AcceptedAndPreserved(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    defaults:
      debounce: "0"
      max_queue_size: 10
`
	m, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := m.Triggers[0].Defaults
	if got["debounce"] != "0" {
		t.Errorf("debounce default = %v, want \"0\"", got["debounce"])
	}
	if n, _ := got["max_queue_size"].(int); n != 10 {
		t.Errorf("max_queue_size default = %v, want 10", got["max_queue_size"])
	}
}

func TestTriggerDefaults_RejectsUnknownKey(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    defaults:
      nonsense: "x"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for unknown defaults key")
	}
	if !strings.Contains(err.Error(), "unknown key \"nonsense\"") {
		t.Errorf("error = %q", err)
	}
}

func TestTriggerDefaults_RejectsWrongType(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    defaults:
      debounce: 300
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for non-string debounce default")
	}
	if !strings.Contains(err.Error(), "defaults.debounce") {
		t.Errorf("error = %q", err)
	}
}

func TestMissingActionName(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
actions:
  - options:
      - name: x
        type: string
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing action name")
	}
	if !strings.Contains(err.Error(), "actions[0]: name is required") {
		t.Errorf("error = %q", err)
	}
}

func TestMissingConfigKey(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
configuration:
  - type: string
triggers:
  - name: Foo
    options:
      - name: x
        type: string
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing config key")
	}
	if !strings.Contains(err.Error(), "key is required") {
		t.Errorf("error = %q", err)
	}
}

func TestShorthandOnTriggerOption(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - name: bar
        type: string
        shorthand: true
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for shorthand on trigger option")
	}
	if !strings.Contains(err.Error(), "shorthand is only valid on action options") {
		t.Errorf("error = %q", err)
	}
}

func TestMultipleShorthand(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
actions:
  - name: Foo
    options:
      - name: a
        type: string
        shorthand: true
      - name: b
        type: string
        shorthand: true
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for multiple shorthand options")
	}
	if !strings.Contains(err.Error(), "at most one option may be marked shorthand") {
		t.Errorf("error = %q", err)
	}
}

func TestDefaultTypeMismatchString(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - name: bar
        type: string
        default: 123
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for string default type mismatch")
	}
	if !strings.Contains(err.Error(), "default value must be a string") {
		t.Errorf("error = %q", err)
	}
}

func TestDefaultTypeMismatchBool(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - name: bar
        type: bool
        default: "yes"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for bool default type mismatch")
	}
	if !strings.Contains(err.Error(), "default value must be a bool") {
		t.Errorf("error = %q", err)
	}
}

func TestDefaultTypeMismatchNumber(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - name: bar
        type: number
        default: "five"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for number default type mismatch")
	}
	if !strings.Contains(err.Error(), "default value must be a number") {
		t.Errorf("error = %q", err)
	}
}

func TestDefaultTypeMismatchList(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - name: bar
        type: list
        default: "not a list"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for list default type mismatch")
	}
	if !strings.Contains(err.Error(), "default value must be a list") {
		t.Errorf("error = %q", err)
	}
}

func TestDefaultTypeMismatchMap(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
actions:
  - name: Foo
    options:
      - name: env
        type: map
        default: "not a map"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for map default type mismatch")
	}
	if !strings.Contains(err.Error(), "default value must be a map") {
		t.Errorf("error = %q", err)
	}
}

func TestValidDefaultTypes(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
actions:
  - name: Foo
    options:
      - name: cmd
        type: string
        default: "echo hello"
      - name: verbose
        type: bool
        default: true
      - name: retries
        type: number
        default: 3
      - name: tags
        type: list
        default:
          - a
          - b
      - name: env
        type: map
        default:
          FOO: bar
`
	_, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("expected valid defaults to pass: %v", err)
	}
}

func TestMultipleValidationErrors(t *testing.T) {
	yaml := `
namespace: com.example
version: "1.0.0"
triggers:
  - options:
      - type: integer
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected errors")
	}
	// Should report multiple issues
	errStr := err.Error()
	if !strings.Contains(errStr, "name is required") {
		t.Errorf("missing 'name is required' in error: %s", errStr)
	}
	if !strings.Contains(errStr, "invalid type") {
		t.Errorf("missing 'invalid type' in error: %s", errStr)
	}
}

func TestInvalidYAML(t *testing.T) {
	_, err := Parse([]byte("name:\n  - bad\n  nested: [unclosed"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestConfigDefaultTypeMismatch(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
configuration:
  - key: poll_interval
    type: number
    default: "not a number"
triggers:
  - name: Foo
    options:
      - name: x
        type: string
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for config default type mismatch")
	}
	if !strings.Contains(err.Error(), "default value must be a number") {
		t.Errorf("error = %q", err)
	}
}

func TestMissingContextName(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    context:
      - type: string
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing context name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err)
	}
}

func TestMapTypeOnTriggerOption(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - name: metadata
        type: map
        default:
          key1: value1
`
	_, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("map type should be valid on trigger options: %v", err)
	}
}

func TestNumberDefaultFloat(t *testing.T) {
	yaml := `
name: test
namespace: com.example
version: "1.0.0"
triggers:
  - name: Foo
    options:
      - name: threshold
        type: number
        default: 3.14
`
	_, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("float default for number type should be valid: %v", err)
	}
}
