# Daemon — Lifecycle

## Application Lifetime

**Startup:**
- Manual start via CLI or system service (e.g., systemd user service)
- Auto-start on login via system service for persistent automations
- Single instance enforced via PID file at `~/.config/recur/run/recurd.pid`
- On startup: configure logging level, read config, load state file, load plugin manifests, resume persisted triggers
- The `recurd` binary accepts `--log-level` to override the config file's `log_level` at startup

**Shutdown:**
- Via CLI command, system service stop, or signal (SIGTERM/SIGINT)
- Graceful shutdown: stop accepting new events, wait for in-progress actions up to `ShutdownTimeout`, persist state, exit
- SIGKILL as hard stop fallback (systemd default 90s `TimeoutStopSec`)

**Crash recovery:**
- Handled by system service (e.g., systemd `Restart=on-failure`)
- State file restored on restart (atomic writes prevent corruption)

## Trigger Lifetime

**States:** Registered → Active ⇄ Suspended → Removed

**Registration:** Triggers auto-activate on registration.

**Suspension:**
- Manual via CLI
- Auto-suspend after consecutive error threshold is reached
- On manual resume: auto-resume is enabled, but error threshold is lowered for that plugin (shorter leash)
- On system shutdown/crash + restart: suspended triggers stay suspended, no auto-resume

**Removal:**
- Manual deregistration via CLI
- Auto-deregistration if daemon detects recurfile deletion (daemon watches its own registered recurfiles)
- On removal: immediately stop accepting new trigger events, allow in-progress actions to complete

**Error isolation:** Errors in one plugin/trigger never crash the daemon or affect other plugins/triggers.

**Logging:**

| Event | Level |
|---|---|
| Plugin/trigger error | error |
| Auto-suspend (threshold reached) | warning |
| Manual suspend | info |
| Manual resume | info |
| Auto-deregistration (recurfile deleted) | warning |
| Removal with in-progress actions | info |

## Action Lifetime

**Lifecycle:**
1. Trigger fires → daemon resolves templates, prepares context file if needed
2. Spawn action plugin process with resolved options, config, and env
3. Process executes, daemon monitors
4. Process exits, context file cleaned up, result logged

**Concurrency modes** (configurable per trigger, default `queue`):
- `queue` — queue new invocations, configurable max queue size. New events dropped with warning when full.
- `parallel` — run concurrently, fully isolated per invocation
- `drop` — skip if action already running, log warning
- `abort` — kill current action, start new one. The user is responsible for handling cleanup. The aborted action receives the same signal handling as daemon shutdown (SIGTERM, then SIGKILL after `shutdown_timeout`). TODO: document signal behavior.

**Error tracking:** Trigger errors and action errors tracked independently, each with their own thresholds. Plugin-level suspension only for cases like plugin binary not found after registration.

## Daemon/Recurfile Options

The following options can be configured at multiple scopes following the standard inheritance pattern (more-specific overrides less-specific).

| Option | Scope | Default | Description |
|---|---|---|---|
| `error_threshold` | daemon config (global, per plugin namespace) | 5 | Consecutive errors before auto-suspend |
| `trigger_error_threshold` | daemon config, group, trigger | falls back to `error_threshold` | Override for trigger-specific error threshold |
| `action_error_threshold` | daemon config, group, action | falls back to `error_threshold` | Override for action-specific error threshold |
| `concurrency_mode` | daemon config, group, trigger | `"queue"` | How concurrent trigger events are handled |
| `max_queue_size` | daemon config, group, trigger | 100 | Max queued events before dropping (queue mode only) |
| `allowed_exit_codes` | action only | `[0]` | Exit codes that do not count toward action error threshold |
| `debounce` | daemon config, group, trigger | `"300ms"` | Batch window for grouping related file events |
| `shutdown_timeout` | daemon config | `"30s"` | Max wait for in-progress actions during graceful shutdown |
