package triggerengine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
)

// DefaultShutdownTimeout is the default time to wait between SIGTERM and SIGKILL
// when stopping external plugin processes.
const DefaultShutdownTimeout = 3 * time.Second

// externalDriver implements Driver for external plugin binaries.
// One process per active trigger.
type externalDriver struct {
	cmd         *exec.Cmd
	binaryPath  string
	socketPath  string
	triggerID   string
	triggerType string
	options     map[string]any
	config      map[string]any
	workDir     string
	pluginName  string
	logLevel    string
	shutdownTimeout time.Duration
	events      chan TriggerEvent
	router      *PluginEventRouter
	done        chan struct{} // closed on intentional stop or unexpected exit
	exited      chan struct{} // closed when the process has fully exited
}

// stdinPayload is the JSON object written to the plugin's stdin.
type stdinPayload struct {
	TriggerType string         `json:"trigger_type"`
	Options     map[string]any `json:"options"`
	Config      map[string]any `json:"config"`
}

// ConfigLookup returns plugin-specific config for the given namespace.
// It is called each time a driver is created so the config snapshot is fresh.
type ConfigLookup func(namespace string) map[string]any

// ExternalPluginFactory returns a DriverFactory that creates externalDriver
// instances for trigger types handled by the given pluginfs.
// shutdownTimeout is the time to allow for graceful shutdown before SIGKILL.
// If zero, DefaultShutdownTimeout is used.
// configLookup may be nil, in which case drivers receive an empty config.
func ExternalPluginFactory(plugin *pluginfs.InstalledPlugin, socketPath string, router *PluginEventRouter, shutdownTimeout time.Duration, configLookup ConfigLookup, logLevel ...string) DriverFactory {
	// Build a set of trigger type names this plugin handles
	triggerTypes := make(map[string]bool, len(plugin.Manifest.Triggers))
	for _, t := range plugin.Manifest.Triggers {
		triggerTypes[strings.ToLower(t.Name)] = true
	}

	binaryPath := plugin.BinaryPath()

	if shutdownTimeout <= 0 {
		shutdownTimeout = DefaultShutdownTimeout
	}

	lvl := ""
	if len(logLevel) > 0 {
		lvl = logLevel[0]
	}

	return func(triggerID, triggerType string, options map[string]any, recurfilePath string) (Driver, error) {
		if !triggerTypes[strings.ToLower(triggerType)] {
			return nil, nil // not handled by this plugin
		}

		workDir := ""
		if recurfilePath != "" {
			workDir = filepath.Dir(recurfilePath)
		}

		cfg := map[string]any{}
		if configLookup != nil {
			if c := configLookup(plugin.Manifest.Namespace); c != nil {
				cfg = c
			}
		}

		return &externalDriver{
			binaryPath:      binaryPath,
			socketPath:      socketPath,
			triggerID:       triggerID,
			triggerType:     triggerType,
			options:         options,
			config:          cfg,
			workDir:         workDir,
			pluginName:      plugin.Manifest.Name,
			logLevel:        lvl,
			shutdownTimeout: shutdownTimeout,
			events:          make(chan TriggerEvent, 16),
			router:          router,
			done:            make(chan struct{}),
			exited:          make(chan struct{}),
		}, nil
	}
}

// Start spawns the plugin binary, writes JSON to stdin, registers with the
// router, and starts the stderr forwarder and process monitor.
func (d *externalDriver) Start() (<-chan TriggerEvent, error) {
	d.cmd = exec.Command(d.binaryPath)

	// Build environment
	d.cmd.Env = append(os.Environ(), d.buildEnvVars()...)

	if d.workDir != "" {
		d.cmd.Dir = d.workDir
	}

	// Capture stderr for log forwarding
	stderr, err := d.cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Set up stdin pipe for JSON payload
	stdin, err := d.cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	// Use process group so we can signal the whole group on stop
	setPluginProcessGroup(d.cmd)

	if err := d.cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting plugin %s: %w", d.pluginName, err)
	}

	// Write JSON payload to stdin, then close
	payload := stdinPayload{
		TriggerType: d.triggerType,
		Options:     d.options,
		Config:      d.config,
	}
	data, _ := json.Marshal(payload)
	stdin.Write(data)
	stdin.Close()

	// Register with router so gRPC callbacks reach our events channel
	d.router.Register(d.triggerID, d.events)

	// Forward stderr lines to daemon log
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			msg := fmt.Sprintf("[%s] %s", d.pluginName, line)
			lower := strings.ToLower(line)
			if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") {
				slog.Error(msg)
			} else {
				slog.Info(msg)
			}
		}
	}()

	// Monitor process exit
	go d.monitorProcess()

	return d.events, nil
}

// Stop shuts down the plugin process: signal done, deregister from router,
// SIGTERM, wait shutdown timeout, SIGKILL if needed.
func (d *externalDriver) Stop() {
	// Signal intentional stop so monitorProcess doesn't log unexpected exit
	select {
	case <-d.done:
		// Already exited — just deregister and wait for cleanup
		d.router.Deregister(d.triggerID)
		<-d.exited
		return
	default:
		close(d.done)
	}

	d.router.Deregister(d.triggerID)

	if d.cmd == nil || d.cmd.Process == nil {
		return
	}

	// Request graceful shutdown
	killPluginProcess(d.cmd)

	// Wait for graceful shutdown before force kill
	select {
	case <-d.exited:
		return
	case <-time.After(d.shutdownTimeout):
		forceKillPluginProcess(d.cmd)
		<-d.exited
	}
}

// monitorProcess waits for the plugin process to exit and closes the
// events channel so the engine's dispatchLoop terminates.
func (d *externalDriver) monitorProcess() {
	if d.cmd != nil {
		d.cmd.Wait()
	}

	d.router.Deregister(d.triggerID)

	select {
	case <-d.done:
	default:
		close(d.done)
		slog.Warn("plugin process exited unexpectedly", "plugin", d.pluginName, "trigger", d.triggerID)
	}

	close(d.events)
	close(d.exited)
}

// buildEnvVars constructs the RECUR_* environment variables from options,
// config, and reserved variables.
func (d *externalDriver) buildEnvVars() []string {
	vars := []string{
		fmt.Sprintf("RECUR_SOCKET=%s", d.socketPath),
		fmt.Sprintf("RECUR_TRIGGER_ID=%s", d.triggerID),
		fmt.Sprintf("RECUR_TRIGGER_TYPE=%s", d.triggerType),
		fmt.Sprintf("RECUR_LOG_LEVEL=%s", d.logLevel),
	}

	vars = append(vars, flattenToEnvVars("", d.options)...)
	vars = append(vars, flattenToEnvVars("", d.config)...)

	return vars
}

// flattenToEnvVars recursively flattens a map into RECUR_KEY=value environment
// variable strings. The prefix parameter is used for nested maps.
func flattenToEnvVars(prefix string, m map[string]any) []string {
	var result []string

	for key, val := range m {
		envKey := strings.ToUpper(key)
		if prefix != "" {
			envKey = prefix + "_" + envKey
		} else {
			envKey = "RECUR_" + envKey
		}

		switch v := val.(type) {
		case string:
			result = append(result, fmt.Sprintf("%s=%s", envKey, v))
		case bool:
			result = append(result, fmt.Sprintf("%s=%t", envKey, v))
		case int:
			result = append(result, fmt.Sprintf("%s=%d", envKey, v))
		case float64:
			result = append(result, fmt.Sprintf("%s=%g", envKey, v))
		case []any:
			parts := make([]string, len(v))
			for i, item := range v {
				parts[i] = fmt.Sprintf("%v", item)
			}
			result = append(result, fmt.Sprintf("%s=%s", envKey, strings.Join(parts, ",")))
		case map[string]any:
			result = append(result, flattenToEnvVars(envKey, v)...)
		default:
			result = append(result, fmt.Sprintf("%s=%v", envKey, v))
		}
	}

	return result
}

// FindTriggerDef looks up a trigger definition by name in a manifest.
func FindTriggerDef(m *manifestyaml.Manifest, triggerType string) *manifestyaml.TriggerDef {
	for i := range m.Triggers {
		if strings.EqualFold(m.Triggers[i].Name, triggerType) {
			return &m.Triggers[i]
		}
	}
	return nil
}
