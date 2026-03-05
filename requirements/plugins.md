# Plugins

## Overview

There are two parts to a plugin: the manifest and the package itself. Each plugin includes a YAML manifest (`manifest.yaml`) bundled alongside the plugin binary in the plugin directory (e.g., `~/.config/recur/plugins/<name>/manifest.yaml`). A plugin may expose triggers, actions, or both.

The plugin itself is a headless application that conforms to the plugin contract (exec-based, process-isolated via gRPC).

While most triggers and actions can simply be referred to by name, in some cases names may conflict with those in other plugins. In those cases warnings will be logged and neither will be registered. To avoid this, plugins must specify a namespace that, when combined with the trigger or action name, will uniquely identify them. In rare cases where there is still a conflict, the user may create an override configuration in the plugin directory to map any exposed properties such as names and namespace.

## Manifest Shape

```yaml
name: fileevents
namespace: core.fileevents
version: "1.0.0"
description: "File system event triggers"
dependencies:
  - inotify-tools

configuration:
  - key: poll_interval
    type: number
    default: 5
    description: "Polling interval in seconds for fallback mode"
  - key: follow_symlinks
    type: bool
    default: false
    description: "Whether to follow symbolic links"

triggers:
  - name: FileCreated
    description: "Fires when a file is created"
    options:
      - name: recursive
        type: bool
        default: false
        description: "Watch subdirectories"
      - name: filter
        type: list
        default: []
        description: "Glob patterns to match"
    context:
      - name: FilePath
        type: string
        description: "Path of the created file"
      - name: TriggeredOn
        type: string
        description: "ISO 8601 timestamp of the event"

actions:
  - name: shell
    description: "Execute a shell command"
    options:
      - name: command
        type: string
        shorthand: true
        description: "Command to execute"
      - name: shell
        type: string
        default: "sh"
        description: "Shell to use"
      - name: params
        type: list
        default: []
        description: "Additional parameters"
```

## Common Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Plugin display name |
| `namespace` | string | yes | Unique namespace for qualification |
| `version` | string | yes | Semver version |
| `description` | string | no | Human-readable description |
| `dependencies` | list\<string\> | no | System dependencies. Informational only — used for discoverability in registries/search, not enforced by the daemon. |
| `configuration` | list | no | Configuration keys this plugin accepts. Validated against `~/.config/recur/config.yaml` entries under `plugins.<namespace>`. |
| `triggers` | list | no* | Trigger definitions exposed by this plugin |
| `actions` | list | no* | Action definitions exposed by this plugin |

\* At least one of `triggers` or `actions` is required.

## Configuration Entry

| Field | Type | Required | Description |
|---|---|---|---|
| `key` | string | yes | Configuration key name |
| `type` | string | yes | Value type: `string`, `bool`, `number`, `list` |
| `default` | varies | no | Default value. Type must match `type` field. |
| `description` | string | no | Human-readable description |

**Rules:**
- Plugin configuration lives in `~/.config/recur/config.yaml` under `plugins.<namespace>`.
- Plugins can only access their own namespace's configuration, enforced by the daemon when passing config to the plugin process.
- Undeclared configuration keys (not in the manifest) emit a warning.
- Configuration values are validated against the manifest's declared types.

## Trigger Definition

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Trigger type name, referenced in recurfiles via `type` |
| `description` | string | no | Human-readable description |
| `options` | list | no | Options this trigger accepts |
| `context` | list | no | Template variables this trigger provides when fired. Used for registration-time template validation. |

## Action Definition

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Action name, referenced in recurfiles via `type` or shorthand key |
| `description` | string | no | Human-readable description |
| `options` | list | no | Options this action accepts. Exactly one may be marked `shorthand: true`. |

## Option Entry (shared by triggers and actions)

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Option name |
| `type` | string | yes | Value type: `string`, `bool`, `number`, `list`, `map` |
| `default` | varies | no | Default value. Type must match `type` field. |
| `description` | string | no | Human-readable description |
| `shorthand` | bool | no | Action options only. Marks this option as the target for the action shorthand form. Exactly one option per action may set this to `true`. If no option is marked, the first option is assumed and a warning is logged. |

## Context Entry (triggers only)

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Variable name, referenced in templates as `{{.Name}}` |
| `type` | string | yes | Value type: `string`, `bool`, `number`, `list` |
| `description` | string | no | Human-readable description |
