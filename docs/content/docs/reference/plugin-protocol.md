---
title: "Plugin Protocol"
weight: 4
description: "How plugins communicate with the daemon"
---

# Plugin Protocol Reference

Plugins are standalone binaries that communicate with the daemon using a combination of stdin JSON, environment variables, and gRPC. This document specifies the protocol for both trigger plugins (long-running event detectors) and action plugins (short-lived executors).

## Binary location

The plugin binary lives inside the plugin directory and its filename must match the `name` field in the plugin's [`manifest.yaml`](manifest.md#name):

```
~/.config/recur/plugins/<plugin-dir>/<manifest.name>
```

## Process isolation

Each plugin instance runs as a separate process in its own process group. The daemon kills the entire process group on deactivation, preventing orphaned child processes. The working directory is set to the recurfile's parent directory.

Plugin stderr is forwarded to the daemon log, prefixed with the plugin name. Plugins should use stderr for diagnostic and debug output.

---

## Trigger mode

Trigger plugins are long-lived processes that detect events and report them to the daemon via gRPC. One process corresponds to one active trigger instance.

### Input (daemon to plugin)

The daemon provides configuration through two equivalent channels. A plugin may read either or both.

#### Stdin (JSON)

A single JSON object written to stdin, followed by EOF:

```json
{
  "trigger_type": "DeviceConnected",
  "options": { "device_type": "usb", "path": "/dev" },
  "config": { "poll_interval": 5 }
}
```

| Field | Type | Description |
|---|---|---|
| `trigger_type` | string | The trigger type name from the recurfile |
| `options` | map | Trigger options from the recurfile, with [inheritance](recurfile.md#inheritance-and-resolution) already resolved |
| `config` | map | Plugin configuration from [`~/.config/recur/config.yaml`](config.md#plugins), scoped to this plugin's namespace |

#### Environment variables

The same data is flattened under a `RECUR_` prefix. Options and config are merged into a single flat namespace (no `OPT`/`CFG` distinction).

Flattening rules:

| Source | Environment variable |
|---|---|
| `options.path` | `RECUR_PATH` |
| `options.ignore_hidden` | `RECUR_IGNORE_HIDDEN` |
| `config.poll_interval` | `RECUR_POLL_INTERVAL` |
| Lists | Comma-separated: `RECUR_FILTER=*.go,*.md` |
| Nested maps: `options.env.HOME` | `RECUR_ENV_HOME` |
| Deep nesting: `a.b.c` | `RECUR_B_C` (top-level prefix stripped) |

#### Reserved environment variables

These are always set, regardless of plugin options or config:

| Variable | Description |
|---|---|
| `RECUR_SOCKET` | Daemon Unix socket path for gRPC callback |
| `RECUR_TRIGGER_ID` | Trigger ID this process reports events for |
| `RECUR_TRIGGER_TYPE` | Trigger type name (e.g., `DeviceConnected`) |

### Output (plugin to daemon)

Trigger plugins report events by connecting to the daemon's Unix socket (`RECUR_SOCKET`) and calling the `ReportTriggerEvent` gRPC method. Multiple events can be reported over the lifetime of the process.

#### gRPC definition

```proto
rpc ReportTriggerEvent(ReportTriggerEventRequest) returns (ReportTriggerEventResponse);

message ReportTriggerEventRequest {
  string trigger_id = 1;
  map<string, string> context = 2;
}

message ReportTriggerEventResponse {
  bool accepted = 1;
  string error = 2;
}
```

The `context` map contains key-value pairs matching the trigger's declared [context variables](manifest.md#context). The daemon validates context keys against the manifest; unknown keys are rejected with `accepted: false`.

```json
{
  "trigger_id": "a1b2c3d4e5f6",
  "context": {
    "SignalName": "DeviceAdded",
    "DevicePath": "/org/freedesktop/UDisks2/drives/sdb"
  }
}
```

### Lifecycle

1. **Spawn.** Daemon starts the binary with environment variables set and writes the JSON payload to stdin, then closes stdin.
2. **Initialize.** Plugin reads stdin and/or environment variables, performs setup, and connects to the gRPC socket.
3. **Report.** Plugin calls `ReportTriggerEvent` each time a matching event occurs.
4. **Stop (graceful).** Daemon sends `SIGTERM`. Plugin should clean up and exit within [`shutdown_timeout`](config.md#shutdown_timeout) (default 30s). After the timeout, the daemon sends `SIGKILL`.
5. **Crash handling.** If the process exits unexpectedly, the daemon marks the trigger as errored and increments the error counter toward [`trigger_error_threshold`](config.md#trigger_error_threshold).

There is no explicit readiness signal. The daemon relies on process health monitoring.

---

## Action mode

Action plugins are short-lived processes spawned once per trigger event. They execute a task and exit.

### Input (daemon to plugin)

A single JSON object written to stdin, followed by EOF:

```json
{
  "action_type": "Shell",
  "options": {
    "command": "echo hello",
    "shell": "bash"
  },
  "config": { "smtp_host": "localhost" },
  "test": false
}
```

| Field | Type | Description |
|---|---|---|
| `action_type` | string | The action name from the recurfile |
| `options` | map | Action options from the recurfile, with [template variables](recurfile.md#template-variables) already resolved |
| `config` | map | Plugin configuration from [`~/.config/recur/config.yaml`](config.md#plugins), scoped to this plugin's namespace |
| `test` | bool | `true` when the action is being executed via [`recur test`](cli.md#recur-test-trigger-id). Plugins can use this to skip side effects. |

Environment variables follow the same flattening rules as trigger mode, with one exception: **sensitive options are excluded from environment variables** and are only available via stdin JSON.

#### Sensitive options

An option is considered sensitive if either:

- The plugin manifest declares `sensitive: true` on the option definition (e.g., `password`, `secret`).
- The recurfile value was sourced from a `{{secret "name"}}` template function.

Sensitive option values appear in the stdin JSON `options` map exactly as normal options do — the JSON schema is unchanged. However, the corresponding `RECUR_OPT_*` environment variable is **not set**. This prevents secrets from being visible in `/proc/<pid>/environ`.

Plugins that read options exclusively from stdin JSON require no changes. Plugins that read options from environment variables as a convenience shortcut must read sensitive options from stdin JSON instead.

### Output (plugin to daemon)

Action plugins write a single JSON object to stdout before exiting:

```json
{
  "success": true,
  "output": "Command completed successfully",
  "error": ""
}
```

| Field | Type | Description |
|---|---|---|
| `success` | bool | Whether the action completed successfully |
| `output` | string | Human-readable output or result data |
| `error` | string | Error message on failure (empty on success) |

### Lifecycle

1. **Spawn.** Daemon starts the binary with environment variables and writes JSON to stdin.
2. **Execute.** Plugin reads input, performs the action, writes output JSON to stdout.
3. **Exit.** Plugin exits. Exit code `0` with `success: true` indicates success. Non-zero exit codes or `success: false` count toward [`action_error_threshold`](config.md#action_error_threshold).
4. **Cleanup.** Daemon reads stdout, cleans up the context file (if any), and logs the result.

---

## Test mode

When a trigger or action is fired via [`recur test`](cli.md#recur-test-trigger-id), the behavior is identical to normal execution with the following differences:

- The `test` field in the action input JSON is set to `true`.
- The special template variable `{{ .Test }}` is set to `true`.
- Plugins may use these signals to skip destructive side effects.

---

## Signal handling

Plugins must handle the following signals:

| Signal | When sent | Expected behavior |
|---|---|---|
| `SIGTERM` | Graceful shutdown (daemon stop, trigger deactivation) | Clean up resources and exit promptly |
| `SIGKILL` | After `shutdown_timeout` expires | Forced termination (cannot be caught) |

The daemon sends signals to the entire process group, so child processes spawned by the plugin also receive the signal.

---

## Event routing (internal)

Inside the daemon, a `PluginEventRouter` bridges gRPC callbacks into the trigger engine's `Driver` interface:

```
Plugin binary -> gRPC ReportTriggerEvent -> PluginEventRouter -> Driver.events channel -> Engine.dispatchLoop
```

This allows the engine to treat external plugin triggers identically to any other driver implementation.
