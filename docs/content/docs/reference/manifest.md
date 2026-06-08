---
title: "Plugin Manifest"
weight: 2
description: "Complete reference for plugin manifest.yaml files"
---

# Plugin Manifest Reference

Every plugin includes a `manifest.yaml` file bundled alongside the plugin binary in the plugin directory (`~/.config/recur/plugins/<name>/manifest.yaml`). The manifest declares the plugin's identity, dependencies, configuration, triggers, and actions.

A plugin may expose triggers, actions, or both, but must expose at least one.

## Full example

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
  - name: Shell
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

---

## Top-level fields

### `name`

Type: string. Required.

The plugin's display name. The plugin binary in the plugin directory must match this name exactly.

```yaml
name: fileevents
```

### `namespace`

Type: string. Required.

A unique namespace used to qualify trigger names, action names, and configuration keys when conflicts occur. Uses reverse-domain convention.

```yaml
namespace: core.fileevents
```

The namespace is used for:
- Fully qualified trigger/action references in [recurfiles](recurfile.md#type) (e.g., `core.fileevents.FileCreated`)
- Plugin configuration keys in [`~/.config/recur/config.yaml`](config.md#plugins) (e.g., `plugins.core.fileevents.poll_interval`)
- [Alias](recurfile.md#aliases) targets

### `version`

Type: string. Required.

Semantic version string.

```yaml
version: "1.0.0"
```

### `description`

Type: string. Optional.

Human-readable description of the plugin's purpose.

```yaml
description: "File system event triggers"
```

### `dependencies`

Type: list of strings. Optional.

System dependencies or requirements. Informational only -- used for discoverability in registries and search, not enforced by the daemon.

```yaml
dependencies:
  - inotify-tools
  - udisks2
```

---

## `configuration`

Type: list of configuration entries. Optional.

Declares the configuration keys this plugin accepts. Values are set by the user in [`~/.config/recur/config.yaml`](config.md#plugins) under `plugins.<namespace>` and passed to the plugin process on each invocation.

```yaml
configuration:
  - key: poll_interval
    type: number
    default: 5
    description: "Polling interval in seconds for fallback mode"
```

Plugins can only access their own namespace's configuration. Undeclared keys (present in config but not in the manifest) emit a warning. Values are validated against the manifest's declared types.

#### `key`

Type: string. Required.

The configuration key name. Referenced in `config.yaml` as `plugins.<namespace>.<key>`.

#### `type`

Type: string. Required.

The value type. One of: `string`, `bool`, `number`, `list`, `map`.

#### `default`

Type: varies (must match `type`). Optional.

Default value used when the key is not set in `config.yaml`.

#### `description`

Type: string. Optional.

Human-readable description of this configuration key.

---

## `triggers`

Type: list of trigger definitions. Optional (but at least one of `triggers` or `actions` is required).

Declares the trigger types this plugin exposes. Each trigger type can be referenced in [recurfile `on` lists](recurfile.md#type).

```yaml
triggers:
  - name: FileCreated
    description: "Fires when a file is created"
    options:
      - name: recursive
        type: bool
        default: false
        description: "Watch subdirectories"
    context:
      - name: FilePath
        type: string
        description: "Path of the created file"
```

### Trigger fields

#### `name`

Type: string. Required.

The trigger type name. Referenced in recurfiles via the [`type` key](recurfile.md#type). Must be unique within the plugin; across plugins, conflicts require namespace qualification.

#### `description`

Type: string. Optional.

Human-readable description of when this trigger fires.

#### Trigger `options`

Type: list of [option entries](#option-entry). Optional.

The options this trigger accepts, set by users in recurfile [`options`](recurfile.md#options) maps.

#### `context`

Type: list of context entries. Optional.

Template variables this trigger provides when it fires. Used for registration-time [template validation](recurfile.md#template-variables) and documented for recurfile authors.

Each context entry has:

##### `name`

Type: string. Required.

The variable name. Referenced in templates as `{{.Name}}`.

##### `type`

Type: string. Required.

The value type. One of: `string`, `bool`, `number`, `list`.

##### `description`

Type: string. Optional.

Human-readable description of this context variable.

---

## `actions`

Type: list of action definitions. Optional (but at least one of `triggers` or `actions` is required).

Declares the actions this plugin exposes. Each action can be referenced in [recurfile `do` lists](recurfile.md#action-definition).

```yaml
actions:
  - name: Shell
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
```

### Action fields

#### `name`

Type: string. Required.

The action name. Referenced in recurfiles via the [`name` key](recurfile.md#name) (detailed form) or as the map key in [shorthand form](recurfile.md#shorthand-form).

#### `description`

Type: string. Optional.

Human-readable description of what this action does.

#### Action `options`

Type: list of [option entries](#option-entry). Optional.

The options this action accepts. Exactly one option may be marked `shorthand: true` (see below).

---

## Option entry

Shared schema for both trigger options and action options.

### `name`

Type: string. Required.

The option name. Used as the key in recurfile `options` maps.

### `type`

Type: string. Required.

The value type. One of: `string`, `bool`, `number`, `list`, `map`.

### `default`

Type: varies (must match `type`). Optional.

Default value used when the option is not set in the recurfile. Serves as the lowest-priority value in the [option inheritance chain](recurfile.md#inheritance-and-resolution).

### `description`

Type: string. Optional.

Human-readable description of this option.

### `shorthand`

Type: bool. Optional. **Action options only.**

Marks this option as the target for the [recurfile shorthand form](recurfile.md#shorthand-form). When a user writes `shell: "echo hello"`, the string value is assigned to the option marked `shorthand: true`.

At most one option per action may set this to `true`. If no option is marked, the first option in the list is assumed and a warning is logged. Setting `shorthand: true` on a trigger option is a validation error.
