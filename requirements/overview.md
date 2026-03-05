# Recur (recur) — Overview

## Description

Recur (recur) is a cross-platform application (Linux, macOS, Windows) that allows declarative configuration of triggered actions driven by local YAML files.

## Architecture

Recur is composed of the following parts:
-   Trigger Plugins
-   Action Plugins
-   Recurfiles that specify the triggers and actions to be taken
-   The daemon (`recurd`) that manages the creation and lifetime of triggers in the system.
-   The CLI (`recur`) to send commands to the daemon's API.
-   A Docker image for containerized deployment.

### Trigger Plugins

Triggers are available in packages as self-contained plugins that are loaded by the daemon and referenced by the recurfile configuration. These can be distributed by any means and are simply stored in the backend daemon's plugin directory.

**Current trigger plugins:**
- **fileevents** — file system change detection (create, modify, delete, move, attribute change)
- **devicemonitor** — USB/block device hotplug (Linux via UDisks2/D-Bus, Windows via WMI)
- **timer** — cron schedules and fixed intervals
- **calendar** — iCal/ICS calendar event triggers with filtering
- **webhook** — inbound HTTP/HTTPS webhooks with HMAC verification, TLS, and rate limiting
- **mqtt** — MQTT topic subscription
- **docker** — Docker container lifecycle events (start, stop, health changes)

### Action Plugins

Similar to Trigger Plugins, Action Plugins are self-contained actions that can be run when a trigger is fired. By default, a shell script action (using the default configured shell for the current environment) is included and if not specified will be default choice.

**Current action plugins:**
- **shell** — execute shell commands (built-in default)
- **mqtt** — publish MQTT messages
- **docker** — container management (start, stop, restart)

### Recurfile Configuration

The YAML configuration specifies the components of the triggers and actions and automatically refers to any file within the current directory. There is also an option to apply the configuration recursively to all files under the directory as well.

The simplest structure is a single type of trigger and a single action on any file in the folder. At its most complex, it may recursively define multiple trigger/action pairs each with their own options.

The configuration can also be split amongst multiple files specified via CLI arguments and merged by the daemon when managing the lifetime of the triggers.

### Daemon (`recurd`)

A background process that manages the lifetime of trigger configurations. Interaction takes place via API that receives commands and updates the backend state, including merging configuration files, starting and executing triggers, logging, and cleanup.

Trigger state is persisted across sessions and should resume automatically upon system start or daemon restart.

The daemon runs as a user-level application.

The daemon implementation varies based on the underlying platform capabilities available for configured triggers, for example inotify on Linux, kqueue on BSD, FileSystemWatcher on Windows, etc. Cross-platform libraries such as fsnotify (Go) can be leveraged.

The daemon uses `log/slog` for structured logging, with a configurable log level (`debug`, `info`, `warn`, `error`) that is propagated to plugins via the `RECUR_LOG_LEVEL` environment variable.

### CLI (`recur`)

A shell utility that calls the API to register new configuration and change behaviors of the application. It is the main interface for the application, though interaction is mainly limited to registering new triggers.

Connection to the backend depends on the platform implementation of the daemon.

### Docker Image

A multi-stage Docker image is available for containerized deployment. The build stage compiles the daemon, CLI, and a configurable set of plugins (`RECUR_PLUGINS` build arg, defaults to `timer webhook`). The runtime stage is Alpine-based, runs as a non-root `recur` user, and uses `tini` as the init process. State is persisted via a volume at `/home/recur/.recur/state`. The `RECUR_SOCKET` and `RECUR_CONFIG_PATH` environment variables configure the daemon.

### Platform Support

| Platform | Status |
|---|---|
| Linux (amd64, arm64, arm/v7, 386) | Full support |
| macOS (amd64, arm64) | Supported. POSIX-compatible daemon code uses `_unix.go` build tags. The devicemonitor plugin is excluded on macOS (no DiskArbitration implementation). |
| Windows (amd64, arm64) | Supported. The devicemonitor plugin uses WMI polling instead of D-Bus. |
