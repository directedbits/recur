package executorsubprocess

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestExecuteSimple(t *testing.T) {
	result, err := Execute(context.Background(), &Request{
		Command: "echo",
		Args:    []string{"hello world"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "hello world" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "hello world\n")
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestExecuteShellRequest(t *testing.T) {
	req := ShellRequest("sh -c", "echo test output")
	result, err := Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "test output" {
		t.Errorf("stdout = %q", result.Stdout)
	}
}

func TestExecuteShellRequestDefaultShell(t *testing.T) {
	req := ShellRequest("", "echo fallback")
	result, err := Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "fallback" {
		t.Errorf("stdout = %q", result.Stdout)
	}
}

func TestExecuteExitCode(t *testing.T) {
	req := ShellRequest("sh -c", "exit 42")
	result, err := Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestExecuteStderr(t *testing.T) {
	req := ShellRequest("sh -c", "echo error >&2")
	result, err := Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if strings.TrimSpace(result.Stderr) != "error" {
		t.Errorf("stderr = %q, want %q", result.Stderr, "error")
	}
	if result.Stdout != "" {
		t.Errorf("stdout should be empty, got %q", result.Stdout)
	}
}

func TestExecuteStdoutAndStderrSeparate(t *testing.T) {
	req := ShellRequest("sh -c", "echo out; echo err >&2")
	result, err := Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "out" {
		t.Errorf("stdout = %q", result.Stdout)
	}
	if strings.TrimSpace(result.Stderr) != "err" {
		t.Errorf("stderr = %q", result.Stderr)
	}
}

func TestExecuteTimeout(t *testing.T) {
	req := ShellRequest("sh -c", "sleep 10")
	req.Timeout = 100 * time.Millisecond

	result, err := Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code for timed out process")
	}
	if result.Duration > 2*time.Second {
		t.Errorf("duration = %v, should be near timeout", result.Duration)
	}
}

func TestExecuteStdin(t *testing.T) {
	result, err := Execute(context.Background(), &Request{
		Command: "cat",
		Stdin:   "piped input",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Stdout != "piped input" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "piped input")
	}
}

func TestExecuteEnv(t *testing.T) {
	req := ShellRequest("sh -c", "echo $MY_VAR")
	req.Env = []string{"MY_VAR=hello_from_env"}

	result, err := Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "hello_from_env" {
		t.Errorf("stdout = %q", result.Stdout)
	}
}

func TestExecuteWorkingDir(t *testing.T) {
	req := ShellRequest("sh -c", "pwd")
	req.WorkingDir = "/tmp"

	result, err := Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// On some systems /tmp may be a symlink
	if !strings.Contains(result.Stdout, "tmp") {
		t.Errorf("stdout = %q, want to contain 'tmp'", result.Stdout)
	}
}

func TestExecuteEmptyCommand(t *testing.T) {
	_, err := Execute(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestActionPluginRequest(t *testing.T) {
	input := &ActionPluginInput{
		ActionType: "Publish",
		Options:    map[string]any{"topic": "test/topic", "payload": "hello"},
		Config:     map[string]any{"broker": "localhost:1883"},
		Test:       false,
	}

	req, err := ActionPluginRequest("/usr/bin/mqtt", input, []string{"RECUR_SOCKET=/tmp/test.sock"}, "/tmp", 30*time.Second)
	if err != nil {
		t.Fatalf("ActionPluginRequest error: %v", err)
	}
	if req.Command != "/usr/bin/mqtt" {
		t.Errorf("Command = %q", req.Command)
	}
	if req.WorkingDir != "/tmp" {
		t.Errorf("WorkingDir = %q", req.WorkingDir)
	}
	if req.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", req.Timeout)
	}
	if len(req.Env) != 1 || req.Env[0] != "RECUR_SOCKET=/tmp/test.sock" {
		t.Errorf("Env = %v", req.Env)
	}
	// Stdin should contain valid JSON
	var parsed ActionPluginInput
	if err := json.Unmarshal([]byte(req.Stdin), &parsed); err != nil {
		t.Fatalf("Stdin is not valid JSON: %v", err)
	}
	if parsed.ActionType != "Publish" {
		t.Errorf("ActionType = %q", parsed.ActionType)
	}
}

func TestParseActionPluginOutput_Success(t *testing.T) {
	stdout := `{"success":true,"output":"published to test/topic","error":""}`
	out, err := ParseActionPluginOutput(stdout)
	if err != nil {
		t.Fatalf("ParseActionPluginOutput error: %v", err)
	}
	if !out.Success {
		t.Error("expected Success = true")
	}
	if out.Output != "published to test/topic" {
		t.Errorf("Output = %q", out.Output)
	}
}

func TestParseActionPluginOutput_Error(t *testing.T) {
	stdout := `{"success":false,"output":"","error":"connection refused"}`
	out, err := ParseActionPluginOutput(stdout)
	if err != nil {
		t.Fatalf("ParseActionPluginOutput error: %v", err)
	}
	if out.Success {
		t.Error("expected Success = false")
	}
	if out.Error != "connection refused" {
		t.Errorf("Error = %q", out.Error)
	}
}

func TestParseActionPluginOutput_InvalidJSON(t *testing.T) {
	_, err := ParseActionPluginOutput("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestShellRequestMultipleParts(t *testing.T) {
	req := ShellRequest("bash --norc -c", "echo test")
	if req.Command != "bash" {
		t.Errorf("Command = %q, want %q", req.Command, "bash")
	}
	if len(req.Args) != 3 || req.Args[0] != "--norc" || req.Args[1] != "-c" || req.Args[2] != "echo test" {
		t.Errorf("Args = %v", req.Args)
	}
}

func TestShellRequestDefaultUsesPlatformShell(t *testing.T) {
	req := ShellRequest("", "echo hello")
	// Should use the platform default shell command, not a hardcoded value
	wantParts := strings.Fields(defaultShellCommand)
	if req.Command != wantParts[0] {
		t.Errorf("Command = %q, want %q (from platform default %q)", req.Command, wantParts[0], defaultShellCommand)
	}
	if len(req.Args) < 2 {
		t.Fatalf("Args = %v, want at least 2 elements", req.Args)
	}
	if req.Args[0] != wantParts[1] {
		t.Errorf("Args[0] = %q, want %q (from platform default %q)", req.Args[0], wantParts[1], defaultShellCommand)
	}
	if req.Args[len(req.Args)-1] != "echo hello" {
		t.Errorf("last arg = %q, want %q", req.Args[len(req.Args)-1], "echo hello")
	}
}
