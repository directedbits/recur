# Daemon — Internals

## Trigger Loading

**Plugin discovery:** On startup, the daemon scans `~/.config/recur/plugins/` for subdirectories containing `manifest.yaml`. Each manifest is parsed and validated. Missing or invalid plugin binaries are logged as errors and skipped.

**Loading order:**
1. Load config
2. Load plugins/manifests
3. Generate recurfile JSON schema (from plugin manifests)
4. Load state (registered recurfile paths/UUIDs, trigger/action states)
5. Validate state (check entries against loaded plugins — flag missing plugins, invalid references, corrupted data. Suspend with reason as needed.)
6. Load registered recurfiles (validate against schema)
7. Activate triggers (respecting persisted states — suspended stays suspended)

**State tracking:**
- Registered recurfiles tracked in state file by UUID (random, assigned at first registration)
- Each trigger/action in state is associated with its source recurfile UUID
- Missing plugin on startup: log error, suspend associated triggers/actions with reason attached to suspension for CLI visibility

**Recurfile change detection:**
- Daemon watches its own registered recurfiles for changes
- On file change: look up associated triggers/actions by recurfile UUID, deregister, re-register from updated file
- On file deletion: deregister all associated triggers/actions
- Implementation detail (deregister/re-register vs diff) deferred to development

## Trigger Management

Trigger management is covered by the combination of trigger lifetime, trigger loading, cross-file merging (recurfile spec), and the API operations. CLI commands for trigger management are defined in the CLI design phase.

## Action Loading and Management

Action loading mirrors trigger loading — actions are defined in the same plugin manifests and loaded during the same startup sequence. Key differences from triggers:

- Actions are passive — no "active/listening" state. They execute on trigger fire.
- Actions are exec-based per invocation — no long-running process to manage.
- Action errors tracked independently with `action_error_threshold`.
- Actions validated at registration (plugin exists, options valid, template variables valid).

**Note:** A CLI `test` command to manually fire a trigger or execute an action would be useful for debugging. Deferred to CLI design.

## API

The CLI-to-daemon API uses **gRPC** with versioned service definitions.

**Core operations:**

| Operation | Description | Accepts / Filters |
|---|---|---|
| `RegisterRecurfile` | Parse, validate, and register a recurfile with the daemon | file path |
| `VerifyRecurfile` | Dry-run validation — report conflicts, errors, and merge results without committing changes | file path |
| `DeregisterRecurfile` | Remove a recurfile and all its associated triggers/actions | by file path or UUID |
| `ListPlugins` | List loaded plugins and their status | by namespace, type (trigger/action/both), status, recurfile (path or UUID) |
| `ListTriggers` | List registered triggers and their current state | by group, plugin, status, recurfile (path or UUID) |
| `ListActions` | List registered actions and their current state | by group, plugin, status, recurfile (path or UUID) |
| `ListGroups` | List registered groups | by recurfile (path or UUID) |
| `InspectGroup` | Show full merged configuration and state for a group | by name or UUID |
| `InspectTrigger` | Show resolved trigger details including inherited options and current state | by name or UUID |
| `InspectAction` | Show resolved action details including options and current state | by name or UUID |
| `GetStatus` | Report daemon health, active trigger/action counts, and recent errors | — |
| `SuspendTrigger` | Manually pause a trigger from firing | by name or UUID |
| `ResumeTrigger` | Resume a suspended trigger | by name or UUID |
| `SuspendAction` | Manually pause an action from executing | by name or UUID |
| `ResumeAction` | Resume a suspended action | by name or UUID |
| `UpdateConfig` | Set a daemon configuration value (e.g., `shutdown_timeout`, `error_threshold`). The change is persisted to `config.yaml` and takes effect immediately. | key + value |
| `TestTrigger` | Manually fire a trigger for debugging/validation | by name or UUID |
| `TestAction` | Manually execute an action for debugging/validation | by name or UUID |
| `InstallPlugin` | Install a plugin from a directory path or archive (zip/tar.gz). Validates manifest and binary, then copies to `~/.config/recur/plugins/<name>/`. Errors if namespace conflicts with existing plugin (must uninstall first, or use `--force`). | file/directory path |
| `UninstallPlugin` | Remove a plugin from the plugin directory. Errors if any triggers or actions from the plugin are currently registered or running — user must deregister them first. Future: `--force` flag to auto-deregister associated triggers/actions. | by namespace or UUID |

All entities (recurfiles, triggers, actions, groups) are assigned UUIDs for unambiguous referencing. Operations accept UUIDs or human-readable identifiers (names, paths) where applicable.

**Future operations:**
- `Resolve` — generic identifier disambiguation across entity types (user-friendly shorthand)
- `StreamLogs` — tail daemon logs
- `StreamEvents` — watch trigger events in real-time

## Connection Types

**Default:** Unix socket at `~/.config/recur/run/recurd.sock`. Standard for local user-level daemons on Linux.

**Future:** TCP as a configurable option for remote management. The transport layer should be abstracted enough during implementation to add TCP without major refactoring.

## Configuration Management

Configuration changes made via CLI are persisted immediately to `~/.config/recur/config.yaml`. All configuration changes take effect immediately — no daemon restart required. If a future config option requires a restart, it will be documented as such.

Plugin configuration is passed to plugins per invocation on each trigger event, so plugins naturally receive the latest config without special hot-reload handling.

## Persistence

Daemon state is persisted as a JSON flat file at `~/.config/recur/state/state.json`. This includes runtime metadata that is not part of the recurfile configuration, such as:

- Plugin error status and suspension state
- Last-fired timestamps per trigger
- Other operational metrics (TBD)

Writes use atomic file operations (write to temp file, then rename) to prevent corruption on crash. The state file is read on daemon startup to restore previous session state.

The recurfiles remain the source of truth for *what* is configured; the state file tracks *how it's running*.

## Configuration

Default configuration is located at: `~/.config/recur/config.yaml`. It is checked on startup and loaded into persistent memory. It can also be modified via CLI.

> **Naming convention:** Configuration keys use snake_case in YAML files (both daemon config and recurfiles). In Go daemon config structs, these will use PascalCase per Go conventions (to be confirmed during implementation).

| Key | Description | Type | Default |
| :-- | :-- | :-- | :-- |
| `default_shell` | Specifies the default shell to launch when using the short form of the shell action or when otherwise not specified. | string | `"sh -c"` (Linux/macOS), `"cmd /c"` (Windows) |
| `log_level` | Controls the daemon's log verbosity. Propagated to plugins via `RECUR_LOG_LEVEL` env var. | string | `""` (defaults to `info`) |
| `socket_address` | Override the daemon socket path or TCP address. | string | `""` (defaults to `~/.config/recur/run/recurd.sock`) |
| `allowed_hosts` | Comma-separated list of allowed hostnames for host-based access control. | string | `""` |

Plugin-specific configuration lives under the `plugins` key, namespaced by plugin namespace:

```yaml
default_shell: "sh -c"

plugins:
  core.fileevents:
    poll_interval: 5
    follow_symlinks: true
  core.notifications:
    smtp_host: "localhost"
```

Each plugin can only access its own namespace's configuration. Keys are validated against the plugin's manifest `configuration` entries.
