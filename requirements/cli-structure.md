# CLI — Structure & Conventions

The CLI is primarily a wrapper around an API client for the daemon. It communicates with the daemon over a Unix socket (default `~/.config/recur/run/recurd.sock`), overridable via `--socket` flag or the `RECUR_SOCKET` environment variable.

## Structure & Conventions

**Root command:** `recur`

**Command style:** Verb-noun — the verb (action) comes first, the noun (entity) second.

```
recur <verb> <noun> [args] [flags]
```

**Default behavior:** `recur` with no subcommand shows help. Daemon status is available via `recur status`.

**Framework:** Cobra (Go). Provides subcommand structure, flag parsing, help generation, and shell completion out of the box.

## Global Flags

| Flag | Short | Description |
|---|---|---|
| `--json` | `-j` | Output in JSON format instead of human-friendly tables |
| `--quiet` | `-q` | Suppress non-essential output (warnings, info messages). Errors still printed to stderr. |
| `--verbose` | `-v` | Show debug-level detail (expanded UUIDs, timing, internal state) |
| `--socket` | `-s` | Override the daemon socket path (default: `~/.config/recur/run/recurd.sock`) |
| `--yes` | `-y` | Skip confirmation prompts (for scripting) |
| `--version` | | Print version and exit |

Flags can appear before or after the subcommand (Cobra default behavior).

`recur version` is also available as a subcommand (equivalent to `recur --version`).

## Output Format

**Default:** Human-friendly aligned tables for list commands, structured key-value output for inspect commands.

```
$ recur list triggers
ID        GROUP        TYPE           STATUS
a1b2c3    My Files     FileCreated    active
d4e5f6    My Files     FileModified   suspended
```

**JSON mode (`--json`):** Full structured output for scripting and piping.

```
$ recur list triggers --json
[{"id":"a1b2c3d4-e5f6-...","group":"My Files","type":"FileCreated","status":"active"}]
```

**Rules:**
- Table output auto-detects terminal width and truncates columns as needed
- JSON output always includes full values (no truncation)
- Exit codes: 0 for success, 1 for errors, 2 for validation errors (e.g., `--verify` failures)
- Errors and warnings go to stderr; data goes to stdout
- `--quiet` suppresses warnings and info to stderr; data output is unaffected

## Identifier Resolution

All commands that accept an entity reference (trigger, action, group, recurfile, plugin) use **universal identifier resolution**:

1. **UUID prefix** — any unique prefix of the full UUID (minimum length determined by ambiguity)
2. **Name** — human-readable name (trigger type, action name, group name, plugin name)
3. **File path** — for recurfiles, absolute or relative path to the `.yaml` file

**Resolution rules:**
- If the identifier matches exactly one entity, it resolves immediately
- If ambiguous (multiple matches), the command errors with a list of candidates
- UUID prefix takes priority over name matching (a hex string that matches a UUID prefix is treated as UUID first)
- Name matching is case-insensitive (consistent with plugin name matching)

**UUID display:**
- List commands show short UUID prefixes (6 characters by default, extended if needed for uniqueness)
- Inspect commands show the full UUID
- `--verbose` mode shows full UUIDs everywhere

## List Command Flags

All `recur list` commands support the following flags:

| Flag | Short | Description |
|---|---|---|
| `--all` | `-a` | Show all entities including suspended (default: hides suspended) |
| `--filter` | `-f` | Filter output based on conditions (repeatable, see below) |
| `--format` | | Format output using a Go template or `json` (see below) |

**`--all` behavior:** By default, list commands hide suspended entities. `--all` includes them. Active and error states are always shown.

**`--filter` syntax:** `key=value`, repeatable. Multiple filters with the same key use OR logic; different keys use AND logic.

```
# AND: active triggers in "My Files" group
recur list triggers --filter status=active --filter group="My Files"

# OR: active or suspended triggers
recur list triggers --filter status=active --filter status=suspended
```

**Available filter keys:**

| Key | Applies to | Description |
|---|---|---|
| `group` | triggers, actions | Group name |
| `plugin` | triggers, actions, plugins | Plugin namespace or name |
| `status` | triggers, actions | Entity status: `active`, `suspended`, `error` |
| `recurfile` | triggers, actions, groups | Recurfile path or UUID |
| `type` | triggers | Trigger type name |
| `type` | actions | Action type |

Unrecognized filter keys are a validation error.

**`--format` syntax:** Either a Go template string or the literal `json`.

```
# Custom template
recur list triggers --format '{{.ID}}\t{{.Type}}\t{{.Status}}'

# JSON output (equivalent to --json global flag)
recur list triggers --format json
```

`--json` (global flag) is an alias for `--format json`. If both `--json` and `--format` are specified, `--format` takes precedence.

## Daemon Connectivity

Most commands require a running daemon. Commands that work **without** the daemon:
- `recur start` — starts the daemon
- `recur restart` — reads launch args from state file, stops daemon, restarts
- `recur completion <shell>` — generates shell completions
- `recur version` — prints version
- `recur config get` — reads directly from config store (default + file layers)
- `recur config set` / `recur config delete` — writes directly to config file (prefers gRPC when daemon is running)

All other commands communicate with the daemon via the API. If the daemon is not running, they exit with code 1 and the message:

```
Error: daemon is not running. Start it with: recur start
```

## Confirmation Prompts

The following commands prompt for confirmation before executing:
- `recur deregister` — shows count of triggers/actions that will be removed
- `recur uninstall` — shows plugin name and namespace
- `recur stop` — shows count of active triggers/in-progress actions

Confirmation prompt format: `<summary>. Continue? [y/N]`. Default is No.

The global `--yes`/`-y` flag skips all confirmation prompts (for scripting and automation).
