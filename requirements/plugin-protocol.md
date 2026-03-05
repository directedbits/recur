# Trigger Plugin Protocol

External trigger plugins are standalone binaries that detect events and report them back
to the daemon via gRPC. This document specifies the communication protocol, lifecycle,
and conventions that all external trigger plugins must follow.

## Binary Location

The plugin binary lives inside the plugin's directory:

```
~/.config/recur/plugins/<plugin-dir>/<manifest.name>
```

The binary name **must** match the `name` field in the plugin's `manifest.yaml`.

## Input (daemon → plugin)

When the daemon spawns a plugin process it provides configuration through two equivalent
channels. A plugin may read either or both — they carry the same data.

### Stdin (JSON)

A single JSON object written to stdin, followed by EOF:

```json
{
  "trigger_type": "DeviceConnected",
  "options": { "device_type": "usb", "path": "/dev" },
  "config": { "poll_interval": 5 }
}
```

| Field          | Type              | Description                                    |
|----------------|-------------------|------------------------------------------------|
| `trigger_type` | string            | The trigger type name from the recurfile        |
| `options`      | map\<string, any\> | Trigger options from the recurfile              |
| `config`       | map\<string, any\> | Plugin configuration from `~/.config/recur/config.yaml`|

### Environment Variables

The same data is flattened under a `RECUR_` prefix. There is no `OPT`/`CFG` distinction
between options and config — all keys are merged into a single flat namespace.

**Flattening rules:**

| Source                          | Environment Variable              |
|---------------------------------|-----------------------------------|
| `options.path`                  | `RECUR_PATH`                       |
| `options.ignore_hidden`         | `RECUR_IGNORE_HIDDEN`              |
| `config.poll_interval`          | `RECUR_POLL_INTERVAL`              |
| Lists                           | Comma-separated: `RECUR_FILTER=*.go,*.md` |
| Nested maps: `options.env.HOME` | `RECUR_ENV_HOME`                   |
| Deep nesting: `a.b.c`          | `RECUR_B_C` (prefix stripped)      |

**Reserved variables** (always set):

| Variable             | Description                                      |
|----------------------|--------------------------------------------------|
| `RECUR_SOCKET`        | Daemon Unix socket path for gRPC callback         |
| `RECUR_TRIGGER_ID`    | Trigger ID this process reports events for         |
| `RECUR_TRIGGER_TYPE`  | Trigger type name (e.g., `DeviceConnected`)       |
| `RECUR_LOG_LEVEL`     | Daemon log level (`debug`, `info`, `warn`, `error`). Plugins may use this to match daemon verbosity. |

### Working Directory

The plugin process's working directory is set to the recurfile's parent directory.

### Stderr

Plugin stderr is forwarded to the daemon log, prefixed with the plugin name.
Plugins should use stderr for diagnostic/debug output.

## Output (plugin → daemon)

Plugins report events by connecting to the daemon's Unix socket (provided via
`RECUR_SOCKET`) and calling the `ReportTriggerEvent` gRPC method.

### Proto Definition

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

### Context Variables

The `context` map contains key-value pairs matching the trigger's declared context
variables from the plugin manifest. For example, a D-Bus plugin might report:

```json
{
  "trigger_id": "a1b2c3d4e5f6",
  "context": {
    "SignalName": "DeviceAdded",
    "DevicePath": "/org/freedesktop/UDisks2/drives/sdb"
  }
}
```

The daemon validates context keys against the manifest's declared context variables.
Unknown keys are rejected with an error response (`accepted: false`).

## Lifecycle

1. **Spawn:** Daemon starts the plugin binary with environment variables set and
   writes the JSON payload to stdin, then closes stdin.
2. **Initialize:** Plugin reads stdin and/or environment variables, performs setup,
   and connects to the gRPC socket.
3. **Report:** Plugin calls `ReportTriggerEvent` each time a matching event occurs.
   Multiple events can be reported over the lifetime of the process.
4. **Stop (graceful):** Daemon sends `SIGTERM`. Plugin should clean up and exit.
   After `shutdown_timeout` (default `30s`, configurable in daemon config), daemon
   sends `SIGKILL`.
5. **Crash handling:** If the plugin process exits unexpectedly, the daemon marks
   the trigger as errored.

### Notes

- There is no explicit readiness signal. The daemon relies on process health monitoring.
- One plugin process corresponds to one active trigger instance.
- The plugin binary is long-lived — it runs for as long as the trigger is active.

## Event Routing (internal)

Inside the daemon, a `PluginEventRouter` bridges gRPC callbacks into the trigger
engine's `Driver` interface:

```
Plugin binary → gRPC ReportTriggerEvent → PluginEventRouter → Driver.events channel → Engine.dispatchLoop
```

This allows the engine to treat external plugin triggers identically to in-process
drivers (e.g., file events).
