---
title: "File Events"
weight: 0
description: "File system event triggers"
---

File system event monitoring powered by [fsnotify](https://github.com/fsnotify/fsnotify) and [fsbroker](https://github.com/helshabini/fsbroker). Both libraries are compiled into the plugin binary with no runtime system dependencies.

## Triggers

All five triggers share the same options schema, making them ideal for group-level option hoisting in recurfiles.

### FileCreated

Fires when a file or directory is created.

### FileModified

Fires when a file's content is modified.

### FileDeleted

Fires when a file or directory is deleted.

### FileMoved

Fires when a file or directory is renamed or moved within the watched directory.

### FileAttributeChanged

Fires when a file or directory's metadata/attributes change (permissions, timestamps, etc.).

## Options

All triggers accept the same options:

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `path` | no | *(recurfile directory)* | Directory to watch. When empty, defaults to the recurfile's parent directory. |
| `recursive` | no | `false` | Watch subdirectories recursively |
| `filter` | no | `[]` | Glob patterns to match (e.g., `["*.go", "*.md"]`). Empty list matches all files. |
| `ignore_hidden` | no | `true` | Ignore hidden files (dotfiles) |
| `ignore_system` | no | `true` | Ignore system/OS-generated files |
| `entity_type` | no | `file` | Entity type to watch: `file`, `directory`, or `all` |
| `exclude_paths` | no | *(see below)* | Glob patterns of paths to exclude (doublestar dialect; `**` supported). |

### `exclude_paths` defaults and resolution

When neither daemon config nor trigger options sets `exclude_paths`, the plugin filters out:

- Anything under `~/.config/recur/**` (so the daemon's own state writes never fire your triggers)
- Any file whose basename is a Recurfile (`recurfile`, `recurfile.yaml`, `recurfile.yml` — case-insensitive)

`exclude_paths` may be set in two places:

- **Daemon config** -- `plugins.core.fileevents.exclude_paths` in `~/.config/recur/config.yaml`. Setting it here **replaces** the defaults entirely for every fileevents trigger.
- **Recurfile trigger options** -- per-trigger. With no daemon-config value, a non-empty list here is **additive** to the defaults. With a daemon-config value, the trigger value replaces it (standard option overlay).

Pass an explicit empty list (`exclude_paths: []`) at either level to drop all defaults without adding anything.

| Daemon config | Trigger options | Effective list | Default Recurfile filter active? |
|---|---|---|---|
| unset | unset | `["~/.config/recur/**"]` | yes |
| unset | non-empty list | defaults ∪ options list | yes |
| unset | `[]` | nothing | no |
| set | unset | config list | no |
| set | any | options list | no |

## Context Variables

### FileCreated, FileModified, FileAttributeChanged

| Variable | Type | Description |
|----------|------|-------------|
| `FilePath` | string | Path of the affected entity |
| `IsDirectory` | bool | Whether the entity is a directory |
| `TriggeredOn` | string | ISO 8601 timestamp of the event |

### FileDeleted

| Variable | Type | Description |
|----------|------|-------------|
| `FilePath` | string | Path of the deleted entity |
| `IsDirectory` | bool | Whether the entity is a directory |
| `PermanentlyDeleted` | bool | True if permanently deleted. Accuracy is platform-dependent. |
| `TriggeredOn` | string | ISO 8601 timestamp of the event |

### FileMoved

| Variable | Type | Description |
|----------|------|-------------|
| `From` | string | Original path. **May not always be available** -- fsnotify only provides the new name on some platforms. |
| `To` | string | New path |
| `IsDirectory` | bool | Whether the entity is a directory |
| `TriggeredOn` | string | ISO 8601 timestamp of the event |

## Examples

### Watch for new Go files

```yaml
GoWatcher:
  on:
    - type: FileCreated
      options:
        filter:
          - "*.go"
  do:
    - shell: "echo 'New file: {{.FilePath}}'"
```

### Build on source changes

```yaml
AutoBuild:
  options:
    recursive: true
    filter:
      - "*.go"
      - "*.mod"
  on:
    - type: FileModified
    - type: FileCreated
  do:
    - shell: "make build"
```

### Recursive watch with entity type filtering

```yaml
NewDirectories:
  on:
    - type: FileCreated
      options:
        path: "/data/incoming"
        recursive: true
        entity_type: directory
  do:
    - shell: "echo 'New directory: {{.FilePath}}' >> /var/log/dirs.log"
```

### Clean up deleted files

```yaml
CleanupTracker:
  on:
    - type: FileDeleted
      options:
        path: "/data/uploads"
        filter:
          - "*.tmp"
  do:
    - shell: "echo '{{.FilePath}} deleted (permanent={{.PermanentlyDeleted}})' >> /var/log/cleanup.log"
```

### Watch everything in current directory

```yaml
AllEvents:
  options:
    entity_type: all
    ignore_hidden: false
  on:
    - type: FileCreated
      do:
        - shell: "echo 'created: {{.FilePath}}'"
    - type: FileModified
      do:
        - shell: "echo 'modified: {{.FilePath}}'"
    - type: FileDeleted
      do:
        - shell: "echo 'deleted: {{.FilePath}}'"
```

## Notes

- `"File"` in trigger names refers to filesystem entities broadly, including directories when `entity_type` is set to `directory` or `all`.
- `ignore_hidden` and `ignore_system` default to `true` to reduce noise from OS and application-generated file activity.
- `recursive` defaults to `false` -- watches only the target directory unless explicitly enabled.
- `PermanentlyDeleted` on `FileDeleted` attempts to distinguish true deletions from trash/recycle bin moves. Accuracy is platform-dependent.
- For `FileMoved`, the `From` field may be empty on platforms where fsnotify does not provide the original path. Templates should handle this gracefully.
