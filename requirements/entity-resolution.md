# Entity Resolution

## Overview

All RPCs that accept an entity identifier follow a consistent resolution pattern. The daemon's entity index resolves identifiers to entity references, optionally filtered by type.

## Resolution Order

Given an identifier string, the index searches in this order:

1. **Exact ID match** — 12-character hex entity ID
2. **Name match** — case-insensitive match against entity names (user-defined `name` field, or the type/path as fallback)
3. **ID prefix match** — shortest unique prefix (minimum 3 characters)

## Type Filtering

Each RPC provides a type hint:

- **Typed commands** (e.g., `inspect trigger <id>`, `suspend action <id>`) pass the entity type. The index only returns matches of that type. If the identifier resolves to a different type, the RPC returns "not found".
- **Untyped commands** (e.g., `inspect <id>`, `suspend <id>`) pass no type filter. The index returns all matches. If exactly one match is found, it is used. If multiple matches are found, the RPC returns an error listing the candidates.

## API Contract

Every RPC that accepts an `identifier` field follows this contract:

1. Resolve the identifier via the entity index
2. If type-filtered: only consider entities of the requested type
3. If zero matches: return `NotFound` with the identifier
4. If one match: proceed with the resolved entity
5. If multiple matches: return `InvalidArgument` with a structured `AmbiguousEntity` error detail containing the candidates (type, ID, name, group, recurfile). The CLI extracts the detail and displays the options to the user.

## CLI Behavior

The CLI maps user commands to RPCs:

| Command | Type filter | Behavior |
|---------|------------|----------|
| `recur inspect trigger <id>` | `"trigger"` | Only matches triggers |
| `recur inspect <id>` | none | Matches any entity type |
| `recur suspend action <id>` | `"action"` | Only matches actions |
| `recur suspend <id>` | none | Matches triggers and actions |
| `recur test <id>` | none | Matches triggers and actions |

When a subcommand specifies the type, the CLI passes it as a hint. When no subcommand is given, the CLI passes no type and the backend resolves.

## Implementation

The daemon service layer uses a single helper:

```
resolveEntity(identifier string, allowedTypes ...string) (*EntityRef, error)
```

- If `allowedTypes` is empty: search all types
- If `allowedTypes` is set: filter results to those types only
- Returns the single match, or an error (not found / ambiguous)

Each RPC calls this helper with the appropriate type filter. The CLI never needs to make multiple RPCs to guess the type.

## Ambiguous Identifier Display

When an ambiguous identifier is returned, the CLI displays candidates with full IDs (not truncated) so the user can copy-paste one to disambiguate.

**Compact output** (default — candidates have unique type+name combinations):
```
Ambiguous identifier "abc" matches 2 entities:
  trigger    abc12345deadbeef  Cron
  action     abc67890beefcafe  Shell
Use the full ID from the list above to disambiguate.
```

**Wide output** (when candidates share type and name, e.g., two triggers both named "Shell"):
```
Ambiguous identifier "Shell" matches 2 entities:
  trigger    aaa11111  Shell            group=Build   recurfile=/path/a.yaml
  trigger    bbb22222  Shell            group=Deploy  recurfile=/path/b.yaml
Use the full ID from the list above to disambiguate.
```

### JSON output (`--json`)

When `--json` is set, structured output is emitted to stdout with exit code 2:
```json
{
  "error": "ambiguous_identifier",
  "identifier": "Shell",
  "candidates": [
    {"entity_type": "trigger", "id": "aaa11111", "name": "Shell", "group": "Build", "recurfile": "/path/a.yaml"},
    {"entity_type": "trigger", "id": "bbb22222", "name": "Shell", "group": "Deploy", "recurfile": "/path/b.yaml"}
  ]
}
```

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (not found, daemon not running, etc.) |
| 2 | Ambiguous identifier — multiple matches |
