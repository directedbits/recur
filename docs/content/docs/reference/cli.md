---
title: "CLI Commands"
weight: 5
description: "Complete CLI command reference"
---

# CLI Command Reference

The `recur` CLI communicates with the daemon over a Unix socket (default `~/.config/recur/run/recurd.sock`). Override the socket path with the `--socket` flag or the `RECUR_SOCKET` environment variable.

## Global flags

| Flag | Short | Description |
|---|---|---|
| `--json` | `-j` | Output in JSON format instead of human-friendly tables |
| `--quiet` | `-q` | Suppress non-essential output (warnings, info messages). Errors still go to stderr. |
| `--verbose` | `-v` | Show debug-level detail (expanded UUIDs, timing, internal state) |
| `--socket` | `-s` | Override the daemon socket path |
| `--yes` | `-y` | Skip confirmation prompts (for scripting) |
| `--version` | | Print version and exit (equivalent to `recur version`) |

Flags can appear before or after the subcommand.

## Output conventions

- **Tables** are the default for list commands. Columns auto-fit terminal width.
- **JSON** (`--json` or `--format json`) outputs full structured data with no truncation.
- **Exit codes:** `0` success, `1` error, `2` validation error (e.g., `--verify` failures).
- Errors and warnings go to stderr; data goes to stdout.
- UUID display: list commands show 6-character prefixes (extended if needed for uniqueness); inspect commands show full UUIDs; `--verbose` shows full UUIDs everywhere.

## Identifier resolution

All commands that accept an entity reference support universal identifier resolution:

1. **UUID prefix** -- any unique prefix of the full UUID.
2. **Name** -- trigger type, action name, group name, or plugin name (case-insensitive).
3. **File path** -- for recurfiles, absolute or relative path.

If the identifier matches multiple entities, the command errors with a list of candidates. UUID prefix matching takes priority over name matching.

## Commands without a running daemon

These commands work without the daemon: `recur start`, `recur completion`, `recur version`, `recur config get`.

All other commands require a running daemon and exit with code 1 if it is not available:

```
Error: daemon is not running. Start it with: recur start
```

---

## Daemon management

### `recur start`

Start the daemon as a background process. Writes PID to `~/.config/recur/run/recurd.pid`.

```
recur start [--foreground]
```

| Flag | Description |
|---|---|
| `--foreground` | Run in the foreground with log output to stdout. Ctrl+C triggers graceful shutdown. |

Errors if the daemon is already running (detected via PID file). On success, prints the PID and exits.

### `recur stop`

Graceful daemon shutdown via the API. Falls back to SIGTERM via PID file if the API is unresponsive.

```
recur stop
```

Prompts for confirmation showing the count of active triggers and in-progress actions. Use `--yes` to skip. Errors if no daemon is running. Waits for confirmation that shutdown completed (up to [`shutdown_timeout`](config.md#shutdown_timeout)).

### `recur status`

Show daemon health, active trigger/action counts, and recent errors.

```
recur status
```

Exit code `0` if the daemon is running, `1` if not.

### `recur version`

Print version information. Also available as `recur --version`. Works without the daemon running.

```
recur version
```

---

## Recurfile management

### `recur register [file]`

Register a recurfile with the daemon.

```
recur register [file] [--verify]
```

| Flag | Description |
|---|---|
| `--verify` | Dry-run mode. Validate and report issues without committing changes. |

**Argument:** Optional file path. If omitted, searches the current directory for default recurfile names in order: `recur.yaml`, `recur.yml`, `.recur.yaml`, `.recur.yml`. If none found, errors with expected filenames. If multiple found, errors listing them.

**Output on success:** Registered file path, count of triggers and actions, and any warnings (e.g., triggers with no actions, cross-file merge warnings).

**Exit code:** `2` when `--verify` finds validation errors.

### `recur verify [file]`

Alias for `recur register --verify [file]`. Validates a recurfile without registering it.

```
recur verify [file]
```

Reports: merge conflicts, option conflicts, unknown trigger types, template validation errors, and triggers with no associated action.

### `recur deregister <id>`

Deregister a recurfile and all its associated triggers and actions.

```
recur deregister <id>
```

**Argument:** Recurfile identifier (UUID, UUID prefix, name, or file path).

Prompts for confirmation showing the count of triggers and actions that will be removed. Use `--yes` to skip. Immediately stops associated triggers; in-progress actions are allowed to complete.

---

## Entity inspection

### `recur list <entity>`

List registered entities.

```
recur list <entity> [--all] [--filter key=value] [--format template|json]
```

Supported entities: `triggers`, `actions`, `groups`, `plugins`, `recurfiles`. Running `recur list` with no entity shows help listing available types.

| Flag | Short | Description |
|---|---|---|
| `--all` | `-a` | Include suspended entities (hidden by default). Active and error states always shown. |
| `--filter` | `-f` | Filter results. Repeatable. Same key = OR logic; different keys = AND logic. |
| `--format` | | Go template string or `json`. Overrides `--json` if both specified. |

**Filter keys:**

| Key | Applies to | Description |
|---|---|---|
| `group` | triggers, actions | Group name |
| `plugin` | triggers, actions, plugins | Plugin namespace or name |
| `status` | triggers, actions | Entity status: `active`, `suspended`, `error` |
| `recurfile` | triggers, actions, groups | Recurfile path or UUID |
| `type` | triggers | Trigger type name |
| `name` | actions | Action name |

Examples:

```
recur list triggers
recur list triggers --all --filter status=active --filter group="My Files"
recur list plugins --format '{{.Namespace}}\t{{.Version}}'
recur list actions --json
```

### `recur inspect <entity> <id>`

Show full details for a single entity.

```
recur inspect <entity> <id>
```

Supported entities: `trigger`, `action`, `group`, `plugin`, `recurfile`.

**Output by entity type:**

| Entity | Details shown |
|---|---|
| `trigger` | Resolved options (with inheritance chain), status, error count, last fired, source recurfile, associated actions |
| `action` | Resolved options, status, error count, last executed, source recurfile, associated trigger |
| `group` | Full merged configuration from all contributing recurfiles, all triggers and actions |
| `plugin` | Manifest contents, configuration values, status, associated triggers/actions |
| `recurfile` | File path, all groups/triggers/actions it contributes |

Examples:

```
recur inspect trigger a1b2c3
recur inspect plugin core.fileevents
recur inspect group "My Files"
```

---

## Entity control

### `recur suspend <entity> <id>`

Suspend a trigger or action.

```
recur suspend trigger <id>
recur suspend action <id>
```

Already-suspended entities produce a no-op with an informational message. In-progress actions for a suspended trigger are allowed to complete.

### `recur resume <entity> <id>`

Resume a suspended trigger or action.

```
recur resume trigger <id>
recur resume action <id>
```

Already-active entities produce a no-op with an informational message. On manual resume, auto-resume is re-enabled with a lowered error threshold.

### `recur test trigger <id>`

Manually fire a trigger for debugging and validation.

```
recur test trigger <id> [--set key=value]
```

| Flag | Description |
|---|---|
| `--set` | Set a context variable value. Repeatable. Format: `key=value`. |

Missing context variables (not provided via `--set`) use an empty string with a warning. Output shows which actions executed and their results. The special variable `{{ .Test }}` is set to `true`.

```
recur test trigger a1b --set FilePath=/tmp/test.txt --set IsDirectory=false
```

### `recur test action <id>`

Manually execute a single action for debugging and validation.

```
recur test action <id> [--set key=value]
```

| Flag | Description |
|---|---|
| `--set` | Set a context variable value. Repeatable. Format: `key=value`. |

Same behavior as `recur test trigger` but executes a single action directly.

---

## Plugin management

### `recur install <path>`

Install a plugin from a directory or archive (zip/tar.gz).

```
recur install <path>
```

**Argument:** Path to a plugin directory or archive file.

Validates the manifest and binary, then copies to `~/.config/recur/plugins/<name>/`. Errors if a plugin with the same namespace already exists; uninstall the existing plugin first.

### `recur uninstall <id>`

Remove an installed plugin.

```
recur uninstall <id>
```

**Argument:** Plugin identifier (UUID, UUID prefix, namespace, or name).

Prompts for confirmation showing the plugin name and namespace. Use `--yes` to skip. Errors if any triggers or actions from the plugin are currently registered; deregister them first.

---

## Configuration

### `recur config get [key]`

Show configuration values. Works without the daemon running (reads directly from `~/.config/recur/config.yaml`).

```
recur config get [key]
```

**Argument:** Optional config key. If omitted, shows all configuration with effective values (including defaults for unset keys).

Plugin config uses dot-path notation: `recur config get plugins.core.fileevents.poll_interval`.

See the [configuration reference](config.md) for all available keys.

### `recur config set <key> <value>`

Set a configuration value.

```
recur config set <key> <value>
```

Persisted immediately to `~/.config/recur/config.yaml` and takes effect immediately in the running daemon. The key and value type are validated against the known config schema.

```
recur config set concurrency_mode parallel
recur config set plugins.core.fileevents.poll_interval 10
```

### `recur config delete <key>`

Remove a configuration key, reverting it to its default value.

```
recur config delete <key>
```

If the key is already absent (at default), the command succeeds as a no-op.

```
recur config delete trigger_error_threshold
```

---

## Shell completions

### `recur completion <shell>`

Generate a shell completion script.

```
recur completion <shell>
```

**Argument:** Shell name: `bash`, `zsh`, or `fish`.

Outputs the completion script to stdout. Pipe to the appropriate location:

```
recur completion bash > ~/.bash_completion.d/recur
recur completion zsh > "${fpath[1]}/_recur"
recur completion fish > ~/.config/fish/completions/recur.fish
```
