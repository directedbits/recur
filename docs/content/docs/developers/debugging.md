---
title: "Debugging"
weight: 4
description: "Practical tips for diagnosing issues"
---

## Checking Daemon Status

```sh
recur status
```

Shows whether the daemon is running, its PID, uptime, and counts of active/suspended triggers and actions.

```sh
recur status --verbose
```

Adds registered recurfile and plugin counts.

## Inspecting Configuration

View all configuration values:

```sh
recur config get
```

View a specific key:

```sh
recur config get log_level
recur config get default_shell
```

## Inspecting Entities

Inspect any registered entity by ID or ID prefix (minimum 3 characters):

```sh
recur inspect trigger abc123
recur inspect action def456
recur inspect group my-group
recur inspect plugin core.timer
recur inspect recurfile /path/to/recur.yaml
```

Inspect output includes status, options, associated entities, error counts, and last activity timestamps.

## Verbose Logging

The daemon logs to stderr. To enable debug-level logging:

**At startup:**

```sh
recurd --log-level debug
```

**At runtime (without restarting):**

```sh
recur config set log_level debug
```

Log levels: `debug`, `info` (default), `warn`, `error`.

## Plugin Debugging

### Stderr forwarding

External plugin processes write to stderr for logging. The daemon captures every line and forwards it to the daemon log with a `[plugin-name]` prefix:

```
[timer] started: cron expression="*/5 * * * *" timezone=UTC fire_on_start=false
[timer] tick #1 reported (uptime: 5m0s)
```

Lines containing the words "error", "fatal", or "panic" (case-insensitive) are logged at **error** level in the daemon. All other lines are logged at **info** level.

### Plugin environment variables

When the daemon spawns a plugin, it sets these environment variables:

| Variable | Description |
|----------|------------|
| `RECUR_SOCKET` | Path to the daemon's Unix socket for gRPC callbacks |
| `RECUR_TRIGGER_ID` | The trigger ID this plugin instance is serving |
| `RECUR_TRIGGER_TYPE` | The trigger type name (e.g., `cron`, `MessageReceived`) |
| `RECUR_LOG_LEVEL` | The daemon's configured log level |
| `RECUR_*` | Flattened trigger options and plugin config |

### Running a plugin manually

You can test a plugin binary outside the daemon by providing the expected stdin JSON and env vars:

```sh
echo '{"trigger_type":"interval","options":{"every":"5s"},"config":{}}' | \
  RECUR_SOCKET=/tmp/recur.sock RECUR_TRIGGER_ID=test123 RECUR_TRIGGER_TYPE=interval \
  ./bin/plugins/timer/timer
```

The plugin will start and attempt to connect to the daemon socket. Without a running daemon, the gRPC dial will fail -- but you can verify that parsing and event source startup work correctly from the stderr output.

## Manual Testing

### Test a trigger

Simulate a trigger firing and execute its associated actions:

```sh
recur test trigger <id-or-prefix>
```

Optionally pass context variables:

```sh
recur test trigger abc123 --context TickCount=42
```

### Test an action

Run a single action in test mode:

```sh
recur test action <id-or-prefix>
```

### Verify a recurfile

Validate a recurfile without registering it:

```sh
recur verify /path/to/recur.yaml
```

Reports parsing errors, warnings (e.g., triggers with no actions), and trigger/action counts.

## Common Issues

### Stale PID file

**Symptom:** `recur start` fails with "daemon already running" but no daemon process exists.

**Fix:** The PID file at `~/.config/recur/recurd.pid` was not cleaned up (e.g., after a crash or `kill -9`). Remove it manually:

```sh
rm ~/.config/recur/recurd.pid
recur start
```

### Socket permission errors

**Symptom:** `recur status` or other commands fail with "permission denied" on the socket.

**Fix:** The socket at `~/.config/recur/recurd.sock` (or `$RECUR_SOCKET`) may have been created by a different user. Remove it and restart:

```sh
rm ~/.config/recur/recurd.sock
recur start
```

### Plugin not discovered

**Symptom:** `recur list plugins` does not show your plugin.

**Checklist:**
1. Plugin directory must be under `~/.config/recur/plugins/<name>/`.
2. Directory must contain both the binary and `manifest.yaml`.
3. Binary must be executable (`chmod +x`).
4. `manifest.yaml` must parse without errors.

Install explicitly:

```sh
recur install ./path/to/plugin-dir
```

### Plugin exits immediately

**Symptom:** Trigger activates but plugin process exits right away. Daemon logs show "plugin process exited unexpectedly".

**Checklist:**
1. Check daemon logs for the `[plugin-name]` stderr output -- the plugin likely logged a fatal error.
2. Common causes: invalid stdin JSON (malformed options), missing required options, failed gRPC connection (bad socket path).
3. Try running the plugin manually (see above) to isolate the issue.

### Trigger not firing (auto-suspended)

**Symptom:** Trigger shows status "suspended" even though you did not suspend it.

**Cause:** Triggers can be persisted as suspended from a previous session. Check with:

```sh
recur inspect trigger <id>
```

Resume it:

```sh
recur resume trigger <id>
```

### State corruption recovery

**Symptom:** Daemon fails to start or loads with missing entities after a crash.

The daemon automatically recovers from interrupted writes on startup using the atomic file recovery mechanism. If the state file is still corrupted:

```sh
# Remove the state file -- the daemon will start fresh
rm ~/.config/recur/state/state.json
recur start

# Re-register your recurfiles
recur register /path/to/recur.yaml
```

Recurfiles on disk remain the source of truth for configuration. The state file only tracks runtime data (status, error counts, timestamps), so removing it just means re-registering.
