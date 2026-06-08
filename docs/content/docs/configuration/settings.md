---
title: "Settings"
weight: 2
description: "Daemon configuration options"
---

Settings live at `~/.config/recur/config.yaml`. All values have sensible defaults.

## Settings Reference

| Key | Default | Description |
|-----|---------|-------------|
| `default_shell` | `sh -c` or `cmd.exe \c` | Shell used to execute commands |
| `error_threshold` | `5` | Consecutive errors before suspending |
| `trigger_error_threshold` | *(inherits from error_threshold)* | Specific override for triggers |
| `action_error_threshold` | *(inherits from error_threshold)* | Specific override for actions |
| `concurrency_mode` | `queue` | How concurrent events are handled |
| `max_queue_size` | `100` | Max queued events before dropping |
| `debounce` | `300ms` | Batch window for rapid events. Duplicate events within this window are treated as a single event. |
| `shutdown_timeout` | `30s` | Time to finish in-progress actions before stopping the daemon. |
| `socket_address` | *(platform default)* | Daemon address: Unix socket path or TCP host:port |
| `allowed_hosts` | *(empty)* | Comma-separated hosts for remote plugin installs |

## Concurrency Modes

The `concurrency_mode` setting controls how the daemon handles events that arrive while an action is already running:

- **`queue`** -- Wait in order (default)
- **`parallel`** -- Run concurrently
- **`drop`** -- Skip if busy
- **`abort`** -- Kill current action, start new one

## Managing Settings

Use the `recur config` commands to view and modify settings:

```sh
recur config get                    # Show all config values
recur config get error_threshold    # Show a specific value
recur config set error_threshold 10
recur config delete error_threshold # Revert to default
```

Values show their source:

```
error_threshold          = 10
concurrency_mode         = queue (inherited from default)
trigger_error_threshold  = 10 (inherited from error_threshold)
```

## Plugin-Specific Configuration

Plugin-specific config uses dot-path keys under a `plugins` section:

```sh
recur config set plugins.core.mqtt.broker "tcp://localhost:1883"
```
