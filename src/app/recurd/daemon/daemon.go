// Package daemon provides the daemon orchestration layer — startup, shutdown,
// and coordination of triggers, actions, and plugins.
package daemon

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pkgconfig "github.com/directedbits/recur/pkg/config"
	triggerengine "github.com/directedbits/recur/src/app/recurd/triggerengine"
	"github.com/directedbits/recur/src/domain/action"
	"github.com/directedbits/recur/src/domain/trigger"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	servergrpc "github.com/directedbits/recur/src/infra/grpc/server"
	statejsonfile "github.com/directedbits/recur/src/infra/jsonfile/state"
	processos "github.com/directedbits/recur/src/infra/os/process"
	secretcomposite "github.com/directedbits/recur/src/infra/secret/composite"
	secretkeyring "github.com/directedbits/recur/src/infra/secret/keyring"
	executorsubprocess "github.com/directedbits/recur/src/infra/subprocess/executor"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
)

// Version is the daemon version. Bump alongside releases.
var Version = "0.1.0-alpha"

// Daemon manages the watch daemon lifecycle.
type Daemon struct {
	config      *configyaml.Config
	configStore *pkgconfig.Store[configyaml.Config]
	configPath  string
	pidPath     string
	socketPath  string
	statePath   string
	startTime   time.Time
	plugins        []*pluginfs.InstalledPlugin
	registry       *registry
	launchArgs     *statejsonfile.LaunchArgs
	actionExecutor ActionExecutor
	triggerEngine  *triggerengine.Engine
	eventRouter    *triggerengine.PluginEventRouter
	logRedactor    *secretcomposite.Redactor
	done           chan struct{}
}

// SetLaunchArgs records the arguments used to start the daemon so they
// can be persisted in the state file and replayed on restart.
func (d *Daemon) SetLaunchArgs(args *statejsonfile.LaunchArgs) {
	d.launchArgs = args
}

// New creates a new Daemon instance from a pre-built config store.
// If socketPath is empty, the default is used.
func New(store *pkgconfig.Store[configyaml.Config], pidPath string, socketPath string) *Daemon {
	effective := store.Get()
	d := &Daemon{
		config:      &effective,
		configStore: store,
		pidPath:     pidPath,
		socketPath:  socketPath,
		registry:    newRegistry(),
		done:        make(chan struct{}),
	}
	return d
}

// ConfigStore returns the underlying config store for use by service RPCs.
func (d *Daemon) ConfigStore() *pkgconfig.Store[configyaml.Config] {
	return d.configStore
}

// SetConfigPath sets the path to the config file for save operations.
func (d *Daemon) SetConfigPath(path string) {
	d.configPath = path
}

// SetStatePath sets the path to the state file for persistence.
func (d *Daemon) SetStatePath(path string) {
	d.statePath = path
}

// SetPlugins overrides the discovered plugins (useful for testing).
func (d *Daemon) SetPlugins(plugins []*pluginfs.InstalledPlugin) {
	d.plugins = plugins
}

// configureLogging sets the default slog logger based on the given level string.
// An empty string or unrecognized value defaults to info.
func (d *Daemon) configureLogging(level string) {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})
	d.logRedactor = secretcomposite.NewRedactor(handler)
	slog.SetDefault(slog.New(d.logRedactor))
}

// resolveSecretsInto resolves secrets for a recurfile and populates execCtx.Secrets.
// Also updates the log redactor with the resolved values.
func (d *Daemon) resolveSecretsInto(execCtx *executorsubprocess.Context, recurfileID string) {
	defs := d.registry.secretDefsForRecurfile(recurfileID)
	if len(defs) == 0 {
		return
	}
	resolver := secretcomposite.New(&secretkeyring.OSKeyring{})
	resolved, err := resolver.ResolveAll(defs)
	if err != nil {
		slog.Error("secret resolution failed", "recurfile", recurfileID[:8], "error", err)
		return
	}
	execCtx.Secrets = resolved
	if d.logRedactor != nil {
		d.logRedactor.UpdateSecrets(resolved)
	}
}

// Run starts the daemon. It writes the PID file, starts the gRPC server,
// sets up signal handling, and blocks until Shutdown() is called or a
// termination signal is received.
func (d *Daemon) Run() error {
	d.configureLogging(*d.config.LogLevel)

	// Resolve socket path
	sockPath := d.socketPath
	if sockPath == "" {
		p, err := processos.DefaultSocketPath()
		if err != nil {
			return err
		}
		sockPath = p
	}

	d.startTime = time.Now()

	// Discover installed plugins (skip if already set, e.g. by tests)
	if d.plugins == nil {
		plugins, pluginErrs := pluginfs.Discover()
		d.plugins = plugins
		for _, e := range pluginErrs {
			slog.Warn("plugin discovery issue", "error", e)
		}
		if len(plugins) > 0 {
			slog.Info("discovered plugins", "count", len(plugins))
			for _, w := range pluginfs.CheckConflicts(plugins) {
				slog.Warn("plugin conflict", "detail", w)
			}
		}
	}

	// Initialize the action executor now that plugins are known
	d.actionExecutor = newActionDispatcher(d.config, d.plugins)

	// Config recovery is handled by InitStore during config loading.
	if d.statePath != "" {
		if err := statejsonfile.Recover(d.statePath); err != nil {
			slog.Warn("state recovery failed", "error", err)
		}
	}

	// Load persisted state and restore registry
	if d.statePath != "" {
		if err := d.loadState(); err != nil {
			slog.Warn("could not load state", "error", err)
		}
	}

	if err := processos.WritePID(d.pidPath, os.Getpid()); err != nil {
		return err
	}

	// Persist launch args immediately so restart can read them even if
	// the daemon crashes before a normal shutdown save.
	if d.statePath != "" && d.launchArgs != nil {
		if err := d.saveState(); err != nil {
			slog.Warn("could not persist launch args", "error", err)
		}
	}

	defer func() {
		if d.statePath != "" {
			if err := d.saveState(); err != nil {
				slog.Warn("could not save state on shutdown", "error", err)
			}
		}
		_ = processos.RemovePID(d.pidPath)
		slog.Info("daemon stopped")
	}()

	// Start gRPC server
	svc := &service{daemon: d}
	srv, err := servergrpc.NewServer(sockPath, svc)
	if err != nil {
		_ = processos.RemovePID(d.pidPath)
		return err
	}

	// Initialize trigger engine with driver factories
	d.eventRouter = triggerengine.NewPluginEventRouter()
	shutdownTimeout := triggerengine.DefaultShutdownTimeout
	if *d.config.ShutdownTimeout != "" {
		if parsed, err := time.ParseDuration(*d.config.ShutdownTimeout); err == nil {
			shutdownTimeout = parsed
		} else {
			slog.Warn("invalid shutdown_timeout, using default", "value", *d.config.ShutdownTimeout)
		}
	}
	configLookup := func(namespace string) map[string]any {
		if d.config.Plugins == nil {
			return nil
		}
		entries := d.config.Plugins[namespace]
		if entries == nil {
			return nil
		}
		// `trigger_defaults` is a daemon-side concern (engine debounce,
		// concurrency, etc.) — strip it before passing the rest to the
		// plugin processos. The plugin only sees its own config keys.
		if _, hasOverrides := entries[pluginTriggerOverridesKey]; !hasOverrides {
			return entries
		}
		filtered := make(map[string]any, len(entries)-1)
		for k, v := range entries {
			if k == pluginTriggerOverridesKey {
				continue
			}
			filtered[k] = v
		}
		return filtered
	}
	var factories []triggerengine.DriverFactory
	for _, p := range d.plugins {
		if len(p.Manifest.Triggers) > 0 {
			factories = append(factories, triggerengine.ExternalPluginFactory(p, sockPath, d.eventRouter, shutdownTimeout, configLookup, *d.config.LogLevel))
		}
	}
	d.triggerEngine = triggerengine.NewEngine(
		d.registry,
		func(ctx context.Context, a *action.Action, execCtx *executorsubprocess.Context) {
			d.resolveSecretsInto(execCtx, a.RecurfileID)
			a.LastExecuted = time.Now().UTC()
			result, warnings := d.actionExecutor.Execute(ctx, a, execCtx)
			for _, w := range warnings {
				slog.Warn("action warning", "action", a.ID[:8], "detail", w)
			}
			if ctx.Err() == context.Canceled {
				slog.Info("action aborted", "action", a.ID[:8])
				return
			}
			if result.Success {
				a.ErrorCount = 0
				slog.Info("action completed", "action", a.ID[:8], "type", a.Type)
			} else {
				a.ErrorCount++
				slog.Error("action failed", "action", a.ID[:8], "exit_code", result.ExitCode, "error", result.Error, "error_count", a.ErrorCount)
				if a.ErrorThreshold > 0 && a.ErrorCount >= a.ErrorThreshold {
					a.Status = action.StatusError
					slog.Warn("action error threshold reached, disabling action",
						"action", a.ID[:8], "type", a.Type,
						"error_count", a.ErrorCount, "threshold", a.ErrorThreshold)
					if err := d.saveState(); err != nil {
						slog.Warn("could not persist state after action threshold breach", "error", err)
					}
				}
			}
		},
		factories...,
	)

	// Persist state after each trigger fire (saves LastFired, LastExecuted)
	d.triggerEngine.SetOnFired(func() {
		if err := d.saveState(); err != nil {
			slog.Warn("could not persist state after trigger fire", "error", err)
		}
	})

	// Handle trigger driver errors (plugin crash / unexpected exit)
	d.triggerEngine.SetOnTriggerError(func(triggerID string) {
		t := d.registry.GetTrigger(triggerID)
		if t == nil {
			return
		}
		t.ErrorCount++
		slog.Warn("trigger plugin error",
			"trigger", triggerID[:8], "type", t.Type, "error_count", t.ErrorCount)
		if t.ErrorThreshold > 0 && t.ErrorCount >= t.ErrorThreshold {
			t.Status = trigger.StatusError
			d.triggerEngine.Deactivate(triggerID)
			slog.Warn("trigger error threshold reached, deactivating trigger",
				"trigger", triggerID[:8], "type", t.Type,
				"error_count", t.ErrorCount, "threshold", t.ErrorThreshold)
		}
		if err := d.saveState(); err != nil {
			slog.Warn("could not persist state after trigger error", "error", err)
		}
	})

	// Activate triggers that were restored from state
	d.activateRegisteredTriggers()

	// Run server in background goroutine
	srvErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(); err != nil {
			srvErr <- err
		}
	}()
	defer func() {
		if d.triggerEngine != nil {
			d.triggerEngine.StopAll()
		}
		srv.Stop()
	}()

	slog.Info("daemon started", "pid", os.Getpid())

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		slog.Info("received signal, shutting down", "signal", sig)
	case <-d.done:
		slog.Info("shutdown requested")
	case err := <-srvErr:
		slog.Error("gRPC server error", "error", err)
		return err
	}

	signal.Stop(sigCh)
	return nil
}


// pluginTriggerOverridesKey is the key under `plugins.<namespace>` in daemon
// config that holds engine-level overrides (debounce, concurrency_mode,
// etc.) for that plugin's triggers. The value is a map[string]any. Stripped
// from the config passed to plugin processes — the plugin doesn't need to
// see it.
const pluginTriggerOverridesKey = "trigger_defaults"

// pluginTriggerOverrides extracts the user's per-plugin engine-level
// overrides from daemon configyaml. Returns a map keyed by plugin namespace,
// with the same shape as the registry's "plugin_override" layer expects.
func (d *Daemon) pluginTriggerOverrides() map[string]map[string]any {
	if d.config.Plugins == nil {
		return nil
	}
	out := make(map[string]map[string]any, len(d.config.Plugins))
	for ns, entries := range d.config.Plugins {
		raw, ok := entries[pluginTriggerOverridesKey]
		if !ok {
			continue
		}
		if overrides, ok := raw.(map[string]any); ok && len(overrides) > 0 {
			out[ns] = overrides
		}
	}
	return out
}

// triggerDefaultsMap returns daemon-level default values from the effective
// config as a flat map, suitable for use as the "daemon" layer in a MapStore.
func (d *Daemon) triggerDefaultsMap() map[string]any {
	triggerThreshold := *d.config.ErrorThreshold
	if d.config.TriggerErrorThreshold != nil {
		triggerThreshold = *d.config.TriggerErrorThreshold
	}
	actionThreshold := *d.config.ErrorThreshold
	if d.config.ActionErrorThreshold != nil {
		actionThreshold = *d.config.ActionErrorThreshold
	}

	m := map[string]any{
		"concurrency_mode":     *d.config.ConcurrencyMode,
		"max_queue_size":       *d.config.MaxQueueSize,
		"error_threshold":      triggerThreshold,
		"action_error_threshold": actionThreshold,
	}
	if *d.config.Debounce != "" {
		m["debounce"] = *d.config.Debounce
	}
	return m
}

// Shutdown triggers a graceful daemon shutdown.
func (d *Daemon) Shutdown() {
	select {
	case <-d.done:
		// Already closed
	default:
		close(d.done)
	}
}

// activateRegisteredTriggers starts file watchers for all active triggers.
func (d *Daemon) activateRegisteredTriggers() {
	if d.triggerEngine == nil {
		return
	}
	for _, t := range d.registry.listTriggers() {
		if t.Status != trigger.StatusActive {
			continue
		}
		if err := d.triggerEngine.Activate(t); err != nil {
			slog.Error("trigger activation failed", "trigger", t.ID[:8], "error", err)
		}
	}
}

// saveState persists the current registry state to the state file.
func (d *Daemon) saveState() error {
	if d.statePath == "" {
		return nil
	}
	f := d.buildState()
	return statejsonfile.Save(f, d.statePath)
}

// buildState creates a statejsonfile.File from the current registry contents.
func (d *Daemon) buildState() *statejsonfile.File {
	snapshots := d.registry.snapshot()

	var wfStates []statejsonfile.RecurfileState
	for _, snap := range snapshots {
		ws := statejsonfile.RecurfileState{
			ID:       snap.ID,
			FilePath: snap.FilePath,
		}
		for _, t := range snap.Triggers {
			ws.Triggers = append(ws.Triggers, statejsonfile.EntityState{
				ID:           t.ID,
				Status:       t.Status,
				ErrorCount:   t.ErrorCount,
				LastActivity: t.LastActivity,
			})
		}
		for _, a := range snap.Actions {
			ws.Actions = append(ws.Actions, statejsonfile.EntityState{
				ID:           a.ID,
				Status:       a.Status,
				ErrorCount:   a.ErrorCount,
				LastActivity: a.LastActivity,
			})
		}
		wfStates = append(wfStates, ws)
	}

	return &statejsonfile.File{
		LaunchArgs: d.launchArgs,
		Recurfiles: wfStates,
	}
}

// loadState restores registry state from the state file by re-registering
// recurfiles and applying persisted entity states.
func (d *Daemon) loadState() error {
	f, err := statejsonfile.Load(d.statePath)
	if err != nil {
		return err
	}

	if len(f.Recurfiles) == 0 {
		return nil
	}

	for _, ws := range f.Recurfiles {
		// Re-parse and register the recurfile
		wfFile, err := recurfileyaml.Load(ws.FilePath)
		if err != nil {
			slog.Warn("state restore: skipping recurfile", "path", ws.FilePath, "error", err)
			continue
		}

		result, err := d.registry.registerRecurfile(wfFile, d.plugins, d.triggerDefaultsMap(), d.pluginTriggerOverrides())
		if err != nil {
			slog.Warn("state restore: skipping recurfile", "path", ws.FilePath, "error", err)
			continue
		}

		// Apply persisted entity states
		triggerStates := make(map[string]entitySnapshot, len(ws.Triggers))
		for _, ts := range ws.Triggers {
			triggerStates[ts.ID] = entitySnapshot{ID: ts.ID, Status: ts.Status, ErrorCount: ts.ErrorCount, LastActivity: ts.LastActivity}
		}
		actionStates := make(map[string]entitySnapshot, len(ws.Actions))
		for _, as := range ws.Actions {
			actionStates[as.ID] = entitySnapshot{ID: as.ID, Status: as.Status, ErrorCount: as.ErrorCount, LastActivity: as.LastActivity}
		}

		d.registry.restoreEntityStates(result.RecurfileID, triggerStates, actionStates)

		slog.Info("state restore: loaded recurfile", "path", ws.FilePath, "triggers", result.TriggerCount, "actions", result.ActionCount)
	}

	return nil
}
