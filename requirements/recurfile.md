# Recurfile

## Overview

The recurfile is a YAML configuration file that declaratively defines triggers and their associated actions. It follows a map-based structure where each key is a named group of triggers. Multiple recurfiles can be registered with the daemon and their configurations are merged.

## Top-Level Structure

The recurfile has two top-level constructs:

```yaml
aliases:                          # optional, top-level plugin namespace aliases
  fs: core.fileevents

My Triggers:                      # group name (map key)
  ...
```

| Key | Type | Required | Description |
|---|---|---|---|
| `aliases` | map\<string, string\> | no | Plugin namespace aliases available to all groups in this file |
| *(group name)* | map | yes (at least one) | A named group of triggers and actions |

**Reserved top-level keys:** `aliases`

## Group Structure

```yaml
My Triggers:
  aliases:
    fs: org.other.filesystem
  options:
    recursive: true
    filter:
      - "*.md"
  on:
    - type: FileCreated
    - type: FileModified
  do:
    - shell: "cat {{.FilePath}}"
```

| Key | Type | Required | Description |
|---|---|---|---|
| `aliases` | map\<string, string\> | no | Plugin namespace aliases scoped to this group. Overrides top-level aliases. |
| `options` | map | no | Default trigger options inherited by all triggers in this group. Shape defined by trigger plugin manifests. |
| `on` | list | yes | List of trigger definitions. Must not be empty. |
| `do` | list | no | Default action(s) applied to triggers that don't define their own `do`. |

**Reserved group-level keys:** `aliases`, `options`, `on`, `do`

## Trigger Entry

```yaml
on:
  - type: FileCreated
    name: "Source watcher"
    options:
      recursive: false
    do:
      - shell: "echo {{.FilePath}}"
```

| Key | Type | Required | Description |
|---|---|---|---|
| `type` | string | yes | Trigger type key from a loaded plugin. May be unqualified or namespace-qualified (including aliases). |
| `name` | string | no | Optional user-defined label for identification and organization. Resolvable by `inspect` and other commands. |
| `options` | map | no | Trigger-specific options. Overrides group-level `options`. Shape defined by the trigger's plugin manifest. |
| `do` | list | no | Action(s) for this trigger. Overrides group-level `do`. |

## Action Entry

Actions support two forms: detailed and shorthand.

**Detailed form:**
```yaml
do:
  - type: shell
    name: "Build step"
    options:
      shell: sh
      command: "cat {{.FilePath}}"
      params:
        - "--key=val"
        - "--flag"
```

**Shorthand form:**
```yaml
do:
  - shell: "cat {{.FilePath}} --key=val"
  - notify: "Build complete"
  - com.example.slack: "#general Build done"
```

In the shorthand form, the key is the action plugin type (or alias/qualified name) and the value is mapped to the plugin's **shorthand option**. Any option in the plugin manifest can be marked as `shorthand: true`. If no option is explicitly marked, the first option defined in the manifest is assumed and a warning is logged.

| Key | Type | Required | Description |
|---|---|---|---|
| `type` | string | yes* | Action plugin type. Required for detailed form. |
| *\<plugin name\>* | string | yes* | Shorthand key using the plugin name, alias, or fully qualified name. Value is mapped to the plugin's default option. Required for short form. |
| `name` | string | no | Optional user-defined label for identification and organization. Resolvable by `inspect` and other commands. Only valid with detailed form. |
| `options` | map | no | Action-specific options. Shape defined by the action plugin manifest. Only valid with detailed form. |

\* Exactly one of `type` or a plugin shorthand key is required. Specifying both is a validation error.

Plugin name matching is **case-insensitive**, though following the plugin's declared casing is recommended. Reserved keys that cannot be used as plugin names: `type`, `name`, `options`.

## Inheritance & Resolution

All inheritance follows the same pattern: **more-specific overrides less-specific**.

**Trigger options:** per-trigger `options` → group-level `options` → plugin manifest defaults

**Actions (`do`):** per-trigger `do` → group-level `do`

**Aliases:** group-level `aliases` → top-level `aliases`

**Rules:**
- Setting an option value to `null` is a validation error. To exclude an inherited option, move the trigger to a separate group.
- A trigger with no `do` at either level is allowed. Warnings are emitted at daemon startup, CLI registration, and in daemon debug logs when the trigger fires.

## Cross-File Merging

When multiple recurfiles define the same group name:

- **Triggers:** appended as separate registrations. Warned at registration.
- **Group-level `do`:** merged (combined into one action list). Warned at registration.
- **Group-level `options`:** conflicting values are a **validation error**. The user must resolve by moving options to per-trigger level.
- **Aliases:** follow the same conflict rule as options — conflicts are a validation error.

The fully merged configuration is queryable via CLI for debugging.

## Namespace Qualification

When trigger option names or trigger types conflict across plugins, they must be fully qualified with the plugin namespace:

```yaml
My Triggers:
  aliases:
    fs: core.fileevents
  options:
    fs.recursive: true
  on:
    - type: fs.FileCreated
```

Aliases can be used in trigger types, option keys, and template variables.

Options that share a name across plugins but have different value types are a validation error unless namespace-qualified.

Options not recognized by any trigger in the group emit a debug-level warning.

## Template Substitution

Action commands use Go-style template syntax to reference trigger context values:

```yaml
shell: "cat {{.FilePath}}"
```

Available template variables are defined by each trigger plugin's manifest. Aliases may be used within templates to qualify variables.

**Validation:** Templates are validated at registration time against the trigger's declared variables. References to undeclared variables are a registration error.

**Runtime:** If a plugin fails to provide a declared variable at runtime, the action errors with a log message.

## Fallback: Context File

For cases where templates are insufficient, the daemon writes trigger context as key-value pairs to a temporary file unique to each trigger event. These are loaded as environment variables scoped to the action's process only. The context file is cleaned up after the action completes.

## Plugin Activation

If a trigger type references a plugin that is installed but not active, the CLI may prompt the user to enable it during registration. This behavior is controlled by a daemon configuration option (default: `false`).

If the plugin is not installed, registration fails with an error.

## CLI Registration

The `register` command accepts a `--verify` (dry-run) flag that validates the recurfile and reports:
- Merge conflicts with already-registered groups
- Option conflicts
- Unknown trigger types
- Template validation errors
- Triggers with no associated action

Without committing any changes.
