---
title: "Configuration"
weight: 3
description: "Complete reference for daemon configuration"
---

# Daemon Configuration Reference

Daemon configuration lives at `~/.config/recur/config.yaml`. The file is created automatically with default values when first needed. All keys use `snake_case`.

Configuration can be viewed and modified via the [`recur config`](cli.md#recur-config-get-key) CLI commands. Changes take effect immediately in the running daemon without a restart. Only non-default values are persisted to keep the file clean.

## Example

```yaml
default_shell: "bash -c"
error_threshold: 10
trigger_error_threshold: 3
concurrency_mode: parallel
debounce: 500ms
shutdown_timeout: 60s

plugins:
  core.fileevents:
    poll_interval: 5
    follow_symlinks: true
  core.notifications:
    smtp_host: "localhost"
```

---

## Keys

### `default_shell`

Type: string. Default: `sh -c` (Linux/macOS), `cmd /c` (Windows).

The default shell used to launch commands when using the shell action's short form or when not otherwise specified.

```yaml
default_shell: "bash -c"
```

### `error_threshold`

Type: integer. Default: `5`.

The number of consecutive errors before a trigger or action is auto-suspended. Serves as the fallback for [`trigger_error_threshold`](#trigger_error_threshold) and [`action_error_threshold`](#action_error_threshold) when those are not explicitly set.

```yaml
error_threshold: 10
```

### `trigger_error_threshold`

Type: integer. Default: inherits from [`error_threshold`](#error_threshold).

Override for the error threshold applied specifically to triggers. When not set (or deleted via `recur config delete`), the effective value equals `error_threshold`.

```yaml
trigger_error_threshold: 3
```

### `action_error_threshold`

Type: integer. Default: inherits from [`error_threshold`](#error_threshold).

Override for the error threshold applied specifically to actions. When not set (or deleted via `recur config delete`), the effective value equals `error_threshold`.

```yaml
action_error_threshold: 8
```

### `concurrency_mode`

Type: string. Default: `queue`.

Controls how concurrent trigger events are handled when an action is already running. Valid values:

| Value | Behavior |
|---|---|
| `queue` | Queue new invocations up to [`max_queue_size`](#max_queue_size). Events beyond the limit are dropped with a warning. |
| `parallel` | Run concurrently, fully isolated per invocation. |
| `drop` | Skip the event if an action is already running. A warning is logged. |
| `abort` | Kill the currently running action and start the new one. The aborted action receives SIGTERM, then SIGKILL after [`shutdown_timeout`](#shutdown_timeout). |

```yaml
concurrency_mode: parallel
```

This value can also be overridden at the group or trigger level in [recurfiles](recurfile.md#options).

### `max_queue_size`

Type: integer. Default: `100`.

Maximum number of queued trigger events when [`concurrency_mode`](#concurrency_mode) is `queue`. Events beyond this limit are dropped with a warning.

```yaml
max_queue_size: 50
```

### `debounce`

Type: string (duration). Default: `300ms`.

Batch window for grouping related events. Events arriving within this window are coalesced. Accepts Go duration strings (e.g., `100ms`, `1s`, `1m30s`).

```yaml
debounce: 500ms
```

This value can also be overridden at the group or trigger level in [recurfiles](recurfile.md#options).

### `shutdown_timeout`

Type: string (duration). Default: `30s`.

Maximum time to wait for in-progress actions to complete during graceful daemon shutdown. After this timeout, remaining processes receive SIGKILL.

```yaml
shutdown_timeout: 60s
```

### `socket_address`

Type: string. Default: `~/.config/recur/run/recurd.sock`.

Path to the Unix socket used for CLI-to-daemon communication. Override with this key, the `--socket` CLI flag, or the `RECUR_SOCKET` environment variable (CLI flag takes precedence).

```yaml
socket_address: /tmp/recur.sock
```

### `allowed_hosts`

Type: string (comma-separated). Default: empty (no hosts allowed).

Comma-separated list of hostnames permitted for network-related operations (e.g., webhook triggers). Matching is case-insensitive.

```yaml
allowed_hosts: "localhost,myhost.local"
```

---

## `plugins`

Type: nested map. Optional.

Plugin-specific configuration, keyed by plugin namespace. Each namespace contains a flat map of key-value pairs corresponding to the plugin's declared [configuration entries](manifest.md#configuration).

```yaml
plugins:
  core.fileevents:
    poll_interval: 5
    follow_symlinks: true
  core.notifications:
    smtp_host: "localhost"
```

Plugins can only access their own namespace's configuration. The daemon validates plugin config values against the manifest's declared types. Undeclared keys (present in config but not in the plugin's manifest) emit a warning.

Plugin configuration is accessed via the CLI using dot-path notation:

```
recur config get plugins.core.fileevents.poll_interval
recur config set plugins.core.fileevents.poll_interval 10
recur config delete plugins.core.fileevents.poll_interval
```

---

## Inheritance rules

Several configuration keys can be overridden at multiple scopes. The resolution order (highest priority first):

| Key | Scopes (highest to lowest) |
|---|---|
| `concurrency_mode` | per-trigger > group > daemon config |
| `max_queue_size` | per-trigger > group > daemon config |
| `debounce` | per-trigger > group > daemon config |
| `trigger_error_threshold` | per-trigger > group > daemon config > `error_threshold` |
| `action_error_threshold` | per-action > group > daemon config > `error_threshold` |

The threshold keys have an additional fallback: when not explicitly set at any scope, they inherit from `error_threshold`.
