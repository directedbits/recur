---
title: "Recurfile Format"
weight: 1
description: "Complete reference for the recurfile YAML format"
---

# Recurfile Format Reference

A recurfile is a YAML configuration file that declaratively defines triggers and their associated actions. Recurfiles are registered with the daemon via `recur register` and live anywhere on the filesystem. Multiple recurfiles can be registered simultaneously; their configurations are merged by the daemon.

Default recurfile names, searched in order when no path is given to `recur register`: `recur.yaml`, `recur.yml`, `.recur.yaml`, `.recur.yml`.

## Top-level structure

A recurfile is a YAML map. Every key is either the reserved key `aliases` or a group name.

```yaml
aliases:
  fs: core.fileevents

My Triggers:
  on:
    - type: FileCreated
  do:
    - shell: "echo {{.FilePath}}"
```

At least one group must be defined.

### `aliases`

Type: map of string to string. Optional.

Defines shorthand names for plugin namespaces. Aliases declared at the file level are available to all groups in the file. Group-level `aliases` override file-level aliases of the same name.

```yaml
aliases:
  fs: core.fileevents
  notify: com.example.notifications
```

Aliases can be used anywhere a plugin namespace is accepted: trigger `type` values, namespace-qualified option keys, and template variables.

---

## Group definition

Each non-reserved top-level key defines a group. The key is the group name (a free-form string).

```yaml
My Triggers:
  aliases:
    fs: org.other.filesystem
  options:
    recursive: true
  on:
    - type: FileCreated
  do:
    - shell: "echo {{.FilePath}}"
```

### `aliases`

Type: map of string to string. Optional.

Group-scoped plugin namespace aliases. Overrides [file-level `aliases`](#aliases) when the same short name is used.

```yaml
My Triggers:
  aliases:
    fs: org.other.filesystem
```

### `options`

Type: map. Optional.

Default trigger options inherited by all triggers in this group. The available keys and value types are defined by the trigger plugin's [manifest](manifest.md#trigger-options). Per-trigger `options` override these values.

```yaml
My Triggers:
  options:
    recursive: true
    filter:
      - "*.md"
  on:
    - type: FileCreated
    - type: FileModified
      options:
        recursive: false
```

In this example, both triggers inherit `filter: ["*.md"]`. `FileCreated` inherits `recursive: true`; `FileModified` overrides it to `false`.

When option names conflict across plugins, qualify them with the plugin namespace or alias:

```yaml
options:
  fs.recursive: true
```

Options not recognized by any trigger in the group emit a debug-level warning. Setting an option to `null` is a validation error.

### `on`

Type: list of [trigger definitions](#trigger-definition). Required.

The list of triggers in this group. Must contain at least one entry.

```yaml
on:
  - type: FileCreated
  - type: FileModified
    options:
      recursive: false
    do:
      - shell: "echo modified {{.FilePath}}"
```

### `do`

Type: list of [action definitions](#action-definition). Optional.

Default actions applied to every trigger in this group that does not define its own `do`. A trigger with no `do` at either level is allowed but produces warnings at registration and when the trigger fires.

```yaml
do:
  - shell: "echo {{.FilePath}}"
```

---

## Trigger definition

Each item in an [`on`](#on) list defines a single trigger.

```yaml
on:
  - type: FileCreated
    name: "Source watcher"
    options:
      recursive: false
    do:
      - shell: "echo {{.FilePath}}"
```

### `type`

Type: string. Required.

The trigger type name from an installed plugin. May be unqualified (`FileCreated`), namespace-qualified (`core.fileevents.FileCreated`), or alias-qualified (`fs.FileCreated`). Matching is case-insensitive, though following the plugin's declared casing is recommended.

If the referenced plugin is installed but not active, the CLI may prompt to enable it during registration. If the plugin is not installed, registration fails.

### `name`

Type: string. Optional.

A user-defined label for identification and organization. When set, the name is used as the display name in CLI output and is searchable by `recur inspect` and other commands.

### `options`

Type: map. Optional.

Trigger-specific options that override [group-level `options`](#options). The available keys and value types are defined by the trigger plugin's [manifest](manifest.md#trigger-options).

```yaml
on:
  - type: FileCreated
    options:
      recursive: true
      filter:
        - "*.go"
```

### `do`

Type: list of [action definitions](#action-definition). Optional.

Actions for this specific trigger. When present, overrides the [group-level `do`](#do) entirely (no merging).

```yaml
on:
  - type: FileCreated
    do:
      - shell: "go build ./..."
      - notify: "Build complete"
```

---

## Action definition

Actions appear in [`do`](#do) lists at either the group level or the trigger level. Two forms are supported: shorthand and detailed.

### Shorthand form

A single-key map where the key is the action plugin name (or alias or fully qualified namespace) and the value is mapped to the plugin's [shorthand option](manifest.md#shorthand).

```yaml
do:
  - shell: "echo {{.FilePath}}"
  - notify: "Build complete"
  - com.example.slack: "#general Build done"
```

Plugin name matching is case-insensitive. The reserved keys `type` and `options` cannot be used as plugin names in shorthand form.

### Detailed form

#### `type`

Type: string. Required (in detailed form).

The action plugin type, alias, or fully qualified namespace.

#### `name`

Type: string. Optional.

A user-defined label for identification and organization. When set, the name is used as the display name in CLI output and is searchable by `recur inspect` and other commands. Only available in the detailed form.

#### `options`

Type: map. Optional.

Action-specific options. The available keys and value types are defined by the action plugin's [manifest](manifest.md#action-options).

```yaml
do:
  - type: shell
    name: "Build step"
    options:
      shell: bash
      command: "cat {{.FilePath}}"
      params:
        - "--key=val"
        - "--flag"
```

Specifying both `type` and a shorthand plugin key in the same action entry is a validation error.

---

## Template variables

Action option values support Go template syntax to reference trigger context values. Available variables are declared in the trigger plugin's [manifest `context` list](manifest.md#context).

```yaml
do:
  - shell: "echo {{.FilePath}} was created at {{.TriggeredOn}}"
```

Templates are validated at registration time against the trigger's declared context variables. References to undeclared variables are a registration error. If a plugin fails to provide a declared variable at runtime, the action errors with a log message.

When an alias qualifies a trigger, the alias can also qualify template variables.

### `{{ .Test }}`

A special boolean variable, always available regardless of the trigger plugin. Set to `true` when the trigger is fired via `recur test`. Useful for conditional behavior in actions.

---

## Inheritance and resolution

All inheritance follows the pattern: more-specific overrides less-specific.

| What | Resolution order (highest priority first) |
|---|---|
| Trigger options | Per-trigger `options` > group `options` > plugin manifest defaults |
| Actions (`do`) | Per-trigger `do` > group `do` |
| Aliases | Group `aliases` > file `aliases` |

Setting an option value to `null` is a validation error. To exclude an inherited option, move the trigger to a separate group.

---

## Cross-file merging

When multiple registered recurfiles define the same group name:

| Element | Behavior |
|---|---|
| Triggers (`on`) | Appended as separate registrations. A warning is emitted at registration. |
| Group-level actions (`do`) | Merged into a combined action list. A warning is emitted. |
| Group-level `options` | Conflicting values are a **validation error**. Resolve by moving options to per-trigger level. |
| `aliases` | Conflicts (same short name, different namespace) are a **validation error**. |

The fully merged configuration is queryable via [`recur inspect group`](cli.md#recur-inspect-entity-id).

---

## Context file fallback

For cases where templates are insufficient, the daemon writes trigger context as key-value pairs to a temporary file unique to each trigger event. These are loaded as environment variables scoped to the action's process. The context file is cleaned up after the action completes.
