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
| `--socket` | `-s` | Override the daemon socket path (default: `~/.config/recur/run/recurd.sock`; also overridable via `$RECUR_SOCKET`) |

Flags can appear before or after the subcommand (Cobra default behavior).

Version is available as the `recur version` subcommand.

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
- List commands show short UUID prefixes (8 characters by default — matching `displayterminal.SafeID`)
- Inspect commands show the full UUID
- `--verbose` mode shows full UUIDs everywhere

The minimum prefix length for *resolving* an identifier (which can be
shorter than the display width) is defined in
[entity-resolution.md](entity-resolution.md).

## List Command Flags

| Flag | Short | Description |
|---|---|---|
| `--all` | `-a` | Include suspended entities. Default: hide suspended; active and error states are always shown. |

For JSON output use the global `--json` flag (no `--format` flag and
no template support; see `displayterminal.PrintJSON` for the
emitted shape).

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

