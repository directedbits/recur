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
