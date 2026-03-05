# CLI — Commands

## Command Hierarchy

The full verb-noun command tree, derived from the daemon API:

**Recurfile commands:**
- `recur register <file>` — register a recurfile (with `--verify` dry-run flag)
- `recur deregister <id>` — deregister a recurfile and all its triggers/actions
- `recur verify <file>` — alias for `recur register --verify <file>`

**List/inspect commands:**
- `recur list triggers` — list registered triggers
- `recur list actions` — list registered actions
- `recur list groups` — list registered groups
- `recur list plugins` — list loaded plugins
- `recur list recurfiles` — list registered recurfiles
- `recur inspect trigger <id>` — show full trigger details
- `recur inspect action <id>` — show full action details
- `recur inspect group <id>` — show full merged group config
- `recur inspect plugin <id>` — show plugin manifest and status
- `recur inspect recurfile <id>` — show recurfile details and associated entities

**Control commands:**
- `recur suspend trigger <id>` — pause a trigger
- `recur resume trigger <id>` — resume a suspended trigger
- `recur suspend action <id>` — pause an action
- `recur resume action <id>` — resume a suspended action

**Test commands:**
- `recur test trigger <id>` — manually fire a trigger
- `recur test action <id>` — manually execute an action

**Plugin commands:**
- `recur install <path>` — install a plugin from directory or archive
- `recur uninstall <id>` — remove a plugin (errors if triggers/actions registered)

**Config commands:**
- `recur config get [key]` — show config value(s). No key = show all.
- `recur config set <key> <value>` — set a config value (persisted immediately)
- `recur config delete <key>` — remove a config key, reverting to its default

**Daemon commands:**
- `recur start` — start the daemon as a background process
- `recur stop` — graceful daemon shutdown
- `recur status` — daemon health, active counts, recent errors
- `recur version` — print version information

**Shell completions:**
- `recur completion <shell>` — generate shell completion script (bash, zsh, fish)

## Per-Command Details

### `recur start`

Start the daemon as a background process (detached Go subprocess). Writes PID to `~/.config/recur/run/recurd.pid`.

| Flag | Description |
|---|---|
| `--foreground` | Run in the foreground with log output to stdout. Useful for debugging. Ctrl+C triggers graceful shutdown. |
| `--file` | Path to config file (default: `~/.config/recur/config.yaml`). |
| `--socket` | Daemon address: Unix socket path or TCP host:port. |
| `--log-level` | Log level: debug, info, warn, error (overrides config). |

**Behavior:**
- Errors if daemon is already running (detected via PID file)
- On success: prints PID and exits
- Launch args are persisted to the state file for use by `recur restart` and `recur status`
- Future: a `recur service install` command could generate systemd/launchd unit files wrapping `recur start --foreground`

### `recur stop`

Send a graceful shutdown to the running daemon via the API. Falls back to SIGTERM via PID file if the API is unresponsive.

**Behavior:**
- Errors if no daemon is running
- Waits for confirmation that shutdown completed (up to `shutdown_timeout`), then exits

### `recur restart`

Stop the running daemon and start a new one with the same launch arguments.

**Behavior:**
- Reads persisted launch args from the state file
- Errors if no previous launch args found (use `recur start` instead)
- Errors if the previous daemon was started in foreground mode (use `recur start --foreground` instead)
- Stops the daemon via SIGTERM, waits for shutdown, then starts a new background process with the same config path, socket, and log level

### `recur status`

Show daemon health, active trigger/action counts, launch args, and recent errors.

**Behavior:**
- If daemon is not running: prints status and exits with code 1
- If daemon is running: prints summary including uptime, version, launch args (config path, socket, log level), entity counts, and exits with code 0
- `--verbose` adds filesystem paths (config, state, socket, PID, plugins directory)

### `recur register [file|id]`

Register a recurfile with the daemon, or reload it if already registered.

| Flag | Description |
|---|---|
| `--verify` | Dry-run mode. Validate and report issues without committing changes. |

**Argument:** Optional file path or recurfile ID. If omitted, searches CWD for default recurfile names in this order: `recur.yaml`, `recur.yml`, `.recur.yaml`, `.recur.yml`.

**Identifier resolution:**
- If argument is a file that exists on disk: resolve to absolute path
- If argument matches a registered recurfile ID: resolve to that recurfile's path for reload
- If no argument: auto-detect in CWD (same as above)

**Reload behavior:** If the resolved path matches an already-registered recurfile, the daemon atomically deregisters the old entities and re-registers from the updated file. Output shows "Reloaded" instead of "Registered".

**Default file resolution:**
- If no file found: error with message listing expected filenames
- If multiple found: error listing the found files, user must specify explicitly
- If one found: use it

**Output on success:** Registered/Reloaded file path, count of triggers and actions, and any warnings (e.g., triggers with no actions, cross-file merge warnings).

`recur verify [file]` is an alias for `recur register --verify [file]`.

### `recur deregister <id>`

Deregister a recurfile and all its associated triggers and actions.

**Argument:** Recurfile identifier (UUID, UUID prefix, name, or file path).

**Behavior:**
- Immediately stops associated triggers
- In-progress actions are allowed to complete (consistent with trigger removal behavior)
- Prints summary of what was deregistered

### `recur list <entity>`

List registered entities. Supported entities: `triggers`, `actions`, `groups`, `plugins`, `recurfiles`.

`recur list` with no entity shows help listing the available entity types.

See [CLI Structure — List Command Flags](cli-structure.md#list-command-flags) for `--all`, `--filter`, and `--format` details.

### `recur inspect <entity> <id>`

Show full details for an entity. Supported entities: `trigger`, `action`, `group`, `plugin`, `recurfile`.

**Argument:** Entity identifier (UUID, UUID prefix, or name).

**Output varies by entity:**
- **trigger:** resolved options (with inheritance chain), status, error count, last fired, source recurfile, associated actions
- **action:** resolved options, status, error count, last executed, source recurfile, associated trigger
- **group:** full merged configuration from all contributing recurfiles, all triggers and actions
- **plugin:** manifest contents, configuration values, status, associated triggers/actions
- **recurfile:** file path, all groups/triggers/actions it contributes

### `recur suspend <entity> <id>`

Manually suspend a trigger or action. Supported entities: `trigger`, `action`.

**Behavior:**
- Already-suspended entities: no-op with informational message
- In-progress actions for a suspended trigger are allowed to complete

### `recur resume <entity> <id>`

Resume a suspended trigger or action. Supported entities: `trigger`, `action`.

**Behavior:**
- Already-active entities: no-op with informational message
- Auto-resume is enabled on manual resume, but with a lowered error threshold (per daemon design)

### `recur test trigger <id>`

Manually fire a trigger for debugging/validation.

| Flag | Description |
|---|---|
| `--set` | Set a context variable value (repeatable). Format: `key=value`. |

```
recur test trigger a1b --set FilePath=/tmp/test.txt --set IsDirectory=false
```

**Behavior:**
- Fires the trigger's associated actions with the provided context
- Missing context variables (not provided via `--set`) use empty string with a warning
- Output shows which actions executed and their results

### `recur test action <id>`

Manually execute an action for debugging/validation.

| Flag | Description |
|---|---|
| `--set` | Set a context variable value (repeatable). Format: `key=value`. |

**Behavior:** Same as `recur test trigger` but executes a single action directly.

### `recur install <path>`

Install a plugin from a directory or archive (zip/tar.gz).

**Argument:** Path to plugin directory or archive file.

**Behavior:**
- Validates manifest and binary
- Copies to `~/.config/recur/plugins/<name>/`
- Errors if a plugin with the same namespace already exists — user must `recur uninstall` first

### `recur uninstall <id>`

Remove a plugin from the plugin directory.

**Argument:** Plugin identifier (UUID, UUID prefix, namespace, or name).

**Behavior:**
- Errors if any triggers or actions from the plugin are currently registered — user must deregister them first
- On success: removes the plugin directory

### `recur config get [key]`

Show configuration values.

**Argument:** Optional config key. If omitted, shows all configuration.

**Behavior:** Reads from `~/.config/recur/config.yaml`. Shows effective values (including defaults for unset keys).

### `recur config set <key> <value>`

Set a configuration value.

**Arguments:** Config key and new value.

**Behavior:** Persisted immediately to `~/.config/recur/config.yaml` and takes effect immediately in the running daemon. Validates key and value type against known config schema.

### `recur config delete <key>`

Remove a configuration key, reverting it to its default value.

**Argument:** Config key to delete.

**Behavior:** Removes the key from `~/.config/recur/config.yaml`. The daemon falls back to the default value. If the key is already absent (at default), the command succeeds as a no-op — the end result is the same.

### `recur version`

Print version information. Also available as `recur --version`.

**Behavior:** Prints version string and exits. Works without the daemon running.

### `recur completion <shell>`

Generate shell completion script.

**Argument:** Shell name — `bash`, `zsh`, or `fish`.

**Output:** Completion script to stdout. User pipes to appropriate location (e.g., `recur completion bash > ~/.bash_completion.d/recur`).
