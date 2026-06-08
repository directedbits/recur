package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/directedbits/recur/src/domain/action"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	executorsubprocess "github.com/directedbits/recur/src/infra/subprocess/executor"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
)

// ActionExecutor runs an action and returns the result.
type ActionExecutor interface {
	Execute(ctx context.Context, a *action.Action, execCtx *executorsubprocess.Context) (*action.ExecutionResult, []string)
}

// actionDispatcher routes actions to the appropriate executor based on PluginID.
type actionDispatcher struct {
	shell  *shellExecutor
	plugin *pluginExecutor
}

func newActionDispatcher(cfg *configyaml.Config, plugins []*pluginfs.InstalledPlugin) *actionDispatcher {
	return &actionDispatcher{
		shell:  &shellExecutor{config: cfg},
		plugin: &pluginExecutor{config: cfg, plugins: plugins},
	}
}

// Execute dispatches to the shell or plugin executor based on the action's PluginID.
func (d *actionDispatcher) Execute(ctx context.Context, a *action.Action, execCtx *executorsubprocess.Context) (*action.ExecutionResult, []string) {
	if a.PluginID != "" {
		return d.plugin.Execute(ctx, a, execCtx)
	}
	return d.shell.Execute(ctx, a, execCtx)
}

// shellExecutor runs actions via the built-in shell.
type shellExecutor struct {
	config *configyaml.Config
}

func (e *shellExecutor) Execute(ctx context.Context, a *action.Action, execCtx *executorsubprocess.Context) (*action.ExecutionResult, []string) {
	var warnings []string

	result, err := executorsubprocess.ResolveOptions(a.Options, execCtx)
	if err != nil {
		return failResult(a, friendlyTemplateError(err)), warnings
	}
	resolved := result.Options

	// Get the command string — check for shorthand (_shorthand key) or command option
	command := ""
	if sh, ok := resolved["_shorthand"]; ok {
		command, _ = sh.(string)
	}
	if command == "" {
		if cmd, ok := resolved["command"]; ok {
			command, _ = cmd.(string)
		}
	}
	if command == "" {
		return failResult(a, "no command specified in action options"), warnings
	}

	req := executorsubprocess.ShellRequest(*e.config.DefaultShell, command)

	if a.RecurfilePath != "" {
		req.WorkingDir = filepath.Dir(a.RecurfilePath)
	}

	if envMap, ok := resolved["env"]; ok {
		if em, ok := envMap.(map[string]any); ok {
			for k, v := range em {
				req.Env = append(req.Env, fmt.Sprintf("%s=%v", k, v))
			}
		}
	}

	if timeoutStr, ok := resolved["timeout"]; ok {
		if ts, ok := timeoutStr.(string); ok && ts != "" && ts != "0" {
			if d, err := time.ParseDuration(ts); err == nil {
				req.Timeout = d
			} else {
				warnings = append(warnings, fmt.Sprintf("invalid timeout %q: %v", ts, err))
			}
		}
	}

	execResult, err := executorsubprocess.Execute(ctx, req)
	if err != nil {
		return failResult(a, err.Error()), warnings
	}

	return &action.ExecutionResult{
		ActionID:   a.ID,
		ActionType: a.Type,
		Success:    execResult.ExitCode == 0,
		ExitCode:   execResult.ExitCode,
		Output:     execResult.Stdout,
		Error:      execResult.Stderr,
		Duration:   execResult.Duration.String(),
	}, warnings
}

// pluginExecutor runs actions by spawning a plugin binary.
type pluginExecutor struct {
	config  *configyaml.Config
	plugins []*pluginfs.InstalledPlugin
}

func (e *pluginExecutor) Execute(ctx context.Context, a *action.Action, execCtx *executorsubprocess.Context) (*action.ExecutionResult, []string) {
	var warnings []string

	p := pluginfs.FindByIdentifier(e.plugins, a.PluginID)
	if p == nil {
		return failResult(a, fmt.Sprintf("plugin %q not found", a.PluginID)), warnings
	}

	result, err := executorsubprocess.ResolveOptions(a.Options, execCtx)
	if err != nil {
		return failResult(a, friendlyTemplateError(err)), warnings
	}
	resolved := result.Options

	// Resolve _shorthand to the manifest's shorthand option name
	options := make(map[string]any, len(resolved))
	for k, v := range resolved {
		if k == "_shorthand" {
			shorthandName, isFallback := p.FindShorthandOption(a.Type)
			if shorthandName != "" {
				options[shorthandName] = v
				if isFallback {
					warnings = append(warnings, fmt.Sprintf("action %q: no shorthand option declared, using first option %q", a.Type, shorthandName))
				}
			} else {
				warnings = append(warnings, "shorthand value present but no shorthand option in manifest")
			}
			continue
		}
		options[k] = v
	}

	// Extract execution metadata before passing options to the plugin
	delete(options, "env")
	delete(options, "command")
	timeout := 30 * time.Second
	if timeoutStr, ok := options["timeout"]; ok {
		if ts, ok := timeoutStr.(string); ok && ts != "" && ts != "0" {
			if d, err := time.ParseDuration(ts); err == nil {
				timeout = d
			} else {
				warnings = append(warnings, fmt.Sprintf("invalid timeout %q: %v", ts, err))
			}
		}
		delete(options, "timeout")
	}

	// Build plugin config from daemon config
	var pluginConfig map[string]any
	if e.config.Plugins != nil {
		pluginConfig = e.config.Plugins[p.Manifest.Namespace]
	}

	input := &executorsubprocess.ActionPluginInput{
		ActionType: a.Type,
		Options:    options,
		Config:     pluginConfig,
		Test:       execCtx != nil && execCtx.Test,
	}

	// Manifest-declared sensitive options for this action type — unioned
	// with the template-tracked result.SensitiveKeys so a literal
	// credential is excluded from env just as a secret-templated one is.
	manifestSensitive := map[string]bool{}
	if def := p.FindActionDefinition(a.Type); def != nil {
		for _, opt := range def.Options {
			if opt.Sensitive {
				manifestSensitive[opt.Name] = true
			}
		}
	}
	env := buildActionEnv(a.Type, *e.config.LogLevel, input.Test, options, result.SensitiveKeys, manifestSensitive)

	workDir := ""
	if a.RecurfilePath != "" {
		workDir = filepath.Dir(a.RecurfilePath)
	}

	req, err := executorsubprocess.ActionPluginRequest(p.BinaryPath(), input, env, workDir, timeout)
	if err != nil {
		return failResult(a, fmt.Sprintf("building plugin request: %v", err)), warnings
	}

	execResult, err := executorsubprocess.Execute(ctx, req)
	if err != nil {
		return failResult(a, err.Error()), warnings
	}

	// Parse JSON output from plugin
	output, err := executorsubprocess.ParseActionPluginOutput(execResult.Stdout)
	if err != nil {
		// If JSON parsing fails, treat as raw output
		return &action.ExecutionResult{
			ActionID:   a.ID,
			ActionType: a.Type,
			Success:    execResult.ExitCode == 0,
			ExitCode:   execResult.ExitCode,
			Output:     execResult.Stdout,
			Error:      execResult.Stderr,
			Duration:   execResult.Duration.String(),
		}, warnings
	}

	return &action.ExecutionResult{
		ActionID:   a.ID,
		ActionType: a.Type,
		Success:    output.Success,
		ExitCode:   execResult.ExitCode,
		Output:     output.Output,
		Error:      output.Error,
		Duration:   execResult.Duration.String(),
	}, warnings
}

// failResult builds a failed ExecutionResult with the given error message.
func failResult(a *action.Action, errMsg string) *action.ExecutionResult {
	return &action.ExecutionResult{
		ActionID:   a.ID,
		ActionType: a.Type,
		Success:    false,
		Error:      errMsg,
	}
}

// friendlyTemplateError rewrites raw Go template errors into user-friendly messages.
// buildActionEnv constructs the RECUR_* environment passed to an action
// plugin process. Options whose names appear in either templateSensitive
// (resolved from a `{{secret …}}` template) or manifestSensitive (declared
// `sensitive: true` in the plugin manifest) are omitted; their values still
// reach the plugin via the stdin JSON payload.
func buildActionEnv(actionType, logLevel string, test bool, options map[string]any, templateSensitive, manifestSensitive map[string]bool) []string {
	env := []string{
		fmt.Sprintf("RECUR_ACTION_TYPE=%s", actionType),
		fmt.Sprintf("RECUR_LOG_LEVEL=%s", logLevel),
	}
	if test {
		env = append(env, "RECUR_TEST=true")
	}
	for k, v := range options {
		if templateSensitive[k] || manifestSensitive[k] {
			continue
		}
		env = append(env, fmt.Sprintf("RECUR_OPT_%s=%v", strings.ToUpper(k), v))
	}
	return env
}

func friendlyTemplateError(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "map has no entry for key") {
		// Extract the key name from: option "command": executing template: ... map has no entry for key "FilePath"
		if idx := strings.Index(msg, "map has no entry for key"); idx >= 0 {
			keyPart := msg[idx+len("map has no entry for key"):]
			keyPart = strings.TrimSpace(keyPart)
			keyPart = strings.Trim(keyPart, "\"")
			return fmt.Sprintf("context variable %q not found — provide it with --set %s=<value> when testing, or ensure the trigger sets it", keyPart, keyPart)
		}
	}
	return fmt.Sprintf("template error: %v", err)
}
