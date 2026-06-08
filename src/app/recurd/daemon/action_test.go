package daemon

import (
	"context"
	"testing"

	"github.com/directedbits/recur/src/domain/action"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	executorsubprocess "github.com/directedbits/recur/src/infra/subprocess/executor"
)

func TestFailResult(t *testing.T) {
	a := &action.Action{
		ID:   "act1",
		Type: "Shell",
	}
	r := failResult(a, "something broke")
	if r.ActionID != "act1" {
		t.Errorf("ActionID = %q", r.ActionID)
	}
	if r.ActionType != "Shell" {
		t.Errorf("ActionName = %q", r.ActionType)
	}
	if r.Success {
		t.Error("expected Success = false")
	}
	if r.Error != "something broke" {
		t.Errorf("Error = %q", r.Error)
	}
}

func TestActionDispatcherRoutes(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	d := newActionDispatcher(cfg, nil)

	// Shell action (no PluginID) should use shell executor
	a := &action.Action{
		ID:      "act1",
		Type:    "Shell",
		Options: map[string]any{"command": "echo hello"},
	}
	result, _ := d.Execute(context.Background(), a, &executorsubprocess.Context{Test: true})
	if result.ActionID != "act1" {
		t.Errorf("ActionID = %q", result.ActionID)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

func TestActionDispatcherPluginNotFound(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	d := newActionDispatcher(cfg, nil)

	// Plugin action with no plugins registered
	a := &action.Action{
		ID:       "act2",
		Type:     "Notify",
		PluginID: "com.test.notify",
		Options:  map[string]any{"message": "hello"},
	}
	result, _ := d.Execute(context.Background(), a, &executorsubprocess.Context{Test: true})
	if result.Success {
		t.Error("expected failure for missing plugin")
	}
	if result.Error == "" {
		t.Error("expected non-empty error")
	}
}

func TestShellExecutorNoCommand(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	e := &shellExecutor{config: cfg}

	a := &action.Action{
		ID:      "act1",
		Type:    "Shell",
		Options: map[string]any{},
	}
	result, _ := e.Execute(context.Background(), a, &executorsubprocess.Context{})
	if result.Success {
		t.Error("expected failure for missing command")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestShellExecutorShorthand(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	e := &shellExecutor{config: cfg}

	a := &action.Action{
		ID:      "act1",
		Type:    "Shell",
		Options: map[string]any{"_shorthand": "echo shorthand-works"},
	}
	result, _ := e.Execute(context.Background(), a, &executorsubprocess.Context{Test: true})
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestShellExecutorWithEnv(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	e := &shellExecutor{config: cfg}

	a := &action.Action{
		ID:   "act1",
		Type: "Shell",
		Options: map[string]any{
			"command": "echo $MY_VAR",
			"env":     map[string]any{"MY_VAR": "testvalue"},
		},
	}
	result, _ := e.Execute(context.Background(), a, &executorsubprocess.Context{})
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

func TestShellExecutorWithTimeout(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	e := &shellExecutor{config: cfg}

	a := &action.Action{
		ID:   "act1",
		Type: "Shell",
		Options: map[string]any{
			"command": "echo fast",
			"timeout": "5s",
		},
	}
	result, warnings := e.Execute(context.Background(), a, &executorsubprocess.Context{})
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestShellExecutorInvalidTimeout(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	e := &shellExecutor{config: cfg}

	a := &action.Action{
		ID:   "act1",
		Type: "Shell",
		Options: map[string]any{
			"command": "echo fast",
			"timeout": "not-a-duration",
		},
	}
	_, warnings := e.Execute(context.Background(), a, &executorsubprocess.Context{})
	if len(warnings) == 0 {
		t.Error("expected warning for invalid timeout")
	}
}

func TestShellExecutorSecretInCommand(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	e := &shellExecutor{config: cfg}

	a := &action.Action{
		ID:   "act1",
		Type: "Shell",
		Options: map[string]any{
			"command": `echo {{secret "api_key"}}`,
		},
	}
	execCtx := &executorsubprocess.Context{
		Secrets: map[string]string{"api_key": "sk-test-123"},
	}
	result, _ := e.Execute(context.Background(), a, execCtx)
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestBuildActionEnv_OmitsSensitiveOptions(t *testing.T) {
	options := map[string]any{
		"topic":    "alerts/critical",
		"password": "hunter2",          // manifest-declared sensitive
		"api_key":  "sk-from-template", // template-tracked sensitive
		"qos":      1,
	}
	manifestSensitive := map[string]bool{"password": true}
	templateSensitive := map[string]bool{"api_key": true}

	env := buildActionEnv("Publish", "info", false, options, templateSensitive, manifestSensitive)

	lookup := map[string]string{}
	for _, kv := range env {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				lookup[kv[:i]] = kv[i+1:]
				break
			}
		}
	}

	if lookup["RECUR_ACTION_TYPE"] != "Publish" {
		t.Errorf("RECUR_ACTION_TYPE = %q", lookup["RECUR_ACTION_TYPE"])
	}
	if lookup["RECUR_LOG_LEVEL"] != "info" {
		t.Errorf("RECUR_LOG_LEVEL = %q", lookup["RECUR_LOG_LEVEL"])
	}
	if _, present := lookup["RECUR_TEST"]; present {
		t.Errorf("RECUR_TEST should not be set when test=false")
	}
	if lookup["RECUR_OPT_TOPIC"] != "alerts/critical" {
		t.Errorf("non-sensitive option dropped: RECUR_OPT_TOPIC = %q", lookup["RECUR_OPT_TOPIC"])
	}
	if lookup["RECUR_OPT_QOS"] != "1" {
		t.Errorf("non-sensitive option dropped: RECUR_OPT_QOS = %q", lookup["RECUR_OPT_QOS"])
	}
	if _, present := lookup["RECUR_OPT_PASSWORD"]; present {
		t.Errorf("manifest-sensitive option leaked into env: RECUR_OPT_PASSWORD = %q", lookup["RECUR_OPT_PASSWORD"])
	}
	if _, present := lookup["RECUR_OPT_API_KEY"]; present {
		t.Errorf("template-sensitive option leaked into env: RECUR_OPT_API_KEY = %q", lookup["RECUR_OPT_API_KEY"])
	}
}

func TestBuildActionEnv_TestFlagAndEmptyOptions(t *testing.T) {
	env := buildActionEnv("Publish", "debug", true, nil, nil, nil)

	got := map[string]bool{}
	for _, kv := range env {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				got[kv[:i]] = true
				break
			}
		}
	}
	if !got["RECUR_TEST"] {
		t.Error("expected RECUR_TEST set when test=true")
	}
	if !got["RECUR_ACTION_TYPE"] || !got["RECUR_LOG_LEVEL"] {
		t.Error("expected RECUR_ACTION_TYPE and RECUR_LOG_LEVEL always set")
	}
}
