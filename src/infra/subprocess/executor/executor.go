// Package executor handles subprocess execution for plugin actions.
//
// The execution model:
//  1. Daemon resolves templates in action options using trigger context
//  2. Daemon spawns the plugin process (for shell: the configured shell)
//  3. Stdin receives resolved template content (for shell: not used, command is via -c arg)
//  4. Stdout and stderr are captured separately
//  5. Exit code and wall-clock duration are recorded
package executorsubprocess

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Request describes a subprocess to execute.
type Request struct {
	// Command is the executable to run (e.g., "sh", "/usr/bin/python3").
	Command string

	// Args are the command arguments (e.g., ["-c", "echo hello"]).
	Args []string

	// Stdin is optional content piped to the process's stdin.
	Stdin string

	// Env is additional environment variables (key=value format).
	Env []string

	// WorkingDir is the working directory. Empty means inherit.
	WorkingDir string

	// Timeout is the maximum execution time. Zero means no timeout.
	Timeout time.Duration
}

// Result captures the outcome of a subprocess execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Execute runs a subprocess and captures its output.
func Execute(ctx context.Context, req *Request) (*Result, error) {
	if req.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	// Use a process group so timeout kills the entire process tree.
	setProcessGroup(cmd)
	cmd.Cancel = cancelFunc(cmd)
	cmd.WaitDelay = 3 * time.Second // allow graceful shutdown before SIGKILL

	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	if len(req.Env) > 0 {
		cmd.Env = append(cmd.Environ(), req.Env...)
	}

	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = -1
		} else {
			return nil, fmt.Errorf("executing %s: %w", req.Command, err)
		}
	}

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// ShellRequest builds a Request for executing a shell command.
// shellCmd is the shell invocation (e.g., "sh -c" or "bash -c").
// command is the resolved command string to execute.
func ShellRequest(shellCmd string, command string) *Request {
	parts := strings.Fields(shellCmd)
	if len(parts) == 0 {
		parts = strings.Fields(defaultShellCommand)
	}

	return &Request{
		Command: parts[0],
		Args:    append(parts[1:], command),
	}
}

// Kill terminates the process group, giving the process a chance to clean up.
func Kill(cmd *exec.Cmd) {
	killProcessGroup(cmd)
}
