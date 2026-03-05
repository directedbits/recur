File Events Trigger Plugin Requirements
-----------------------------------------

# Overview

The initial trigger plugin provides file system event monitoring powered by [fsnotify](https://github.com/fsnotify/fsnotify) and [fsbroker](https://github.com/helshabini/fsbroker). FSBroker normalizes raw platform-specific events into reliable high-level actions, handling cross-platform inconsistencies in event sequences.

Both libraries are compiled into the plugin binary — no runtime system dependencies.

# Manifest

```yaml
name: fileevents
namespace: core.fileevents
version: "1.0.0"
description: "File system event triggers powered by fsnotify/fsbroker"

triggers:
  - name: FileCreated
    description: "Fires when a file or directory is created"
    options:
      - name: path
        type: string
        default: ""
        description: "Directory to watch. Defaults to the recurfile's directory."
      - name: recursive
        type: bool
        default: false
        description: "Watch subdirectories"
      - name: filter
        type: list
        default: []
        description: "Glob patterns to match"
      - name: ignore_hidden
        type: bool
        default: true
        description: "Ignore hidden files (dotfiles)"
      - name: ignore_system
        type: bool
        default: true
        description: "Ignore system/OS-generated files"
      - name: entity_type
        type: string
        default: "file"
        description: "Entity type to watch: file, directory, or all"
    context:
      - name: FilePath
        type: string
        description: "Path of the created entity"
      - name: IsDirectory
        type: bool
        description: "Whether the entity is a directory"
      - name: TriggeredOn
        type: string
        description: "ISO 8601 timestamp of the event"

  - name: FileModified
    description: "Fires when a file's content is modified"
    options:
      - name: path
        type: string
        default: ""
        description: "Directory to watch. Defaults to the recurfile's directory."
      - name: recursive
        type: bool
        default: false
        description: "Watch subdirectories"
      - name: filter
        type: list
        default: []
        description: "Glob patterns to match"
      - name: ignore_hidden
        type: bool
        default: true
        description: "Ignore hidden files (dotfiles)"
      - name: ignore_system
        type: bool
        default: true
        description: "Ignore system/OS-generated files"
      - name: entity_type
        type: string
        default: "file"
        description: "Entity type to watch: file, directory, or all"
    context:
      - name: FilePath
        type: string
        description: "Path of the modified entity"
      - name: IsDirectory
        type: bool
        description: "Whether the entity is a directory"
      - name: TriggeredOn
        type: string
        description: "ISO 8601 timestamp of the event"

  - name: FileDeleted
    description: "Fires when a file or directory is deleted"
    options:
      - name: path
        type: string
        default: ""
        description: "Directory to watch. Defaults to the recurfile's directory."
      - name: recursive
        type: bool
        default: false
        description: "Watch subdirectories"
      - name: filter
        type: list
        default: []
        description: "Glob patterns to match"
      - name: ignore_hidden
        type: bool
        default: true
        description: "Ignore hidden files (dotfiles)"
      - name: ignore_system
        type: bool
        default: true
        description: "Ignore system/OS-generated files"
      - name: entity_type
        type: string
        default: "file"
        description: "Entity type to watch: file, directory, or all"
    context:
      - name: FilePath
        type: string
        description: "Path of the deleted entity"
      - name: IsDirectory
        type: bool
        description: "Whether the entity is a directory"
      - name: PermanentlyDeleted
        type: bool
        description: "True if permanently deleted, false if moved to trash/recycle bin. Accuracy is platform-dependent."
      - name: TriggeredOn
        type: string
        description: "ISO 8601 timestamp of the event"

  - name: FileMoved
    description: "Fires when a file or directory is renamed or moved"
    options:
      - name: path
        type: string
        default: ""
        description: "Directory to watch. Defaults to the recurfile's directory."
      - name: recursive
        type: bool
        default: false
        description: "Watch subdirectories"
      - name: filter
        type: list
        default: []
        description: "Glob patterns to match"
      - name: ignore_hidden
        type: bool
        default: true
        description: "Ignore hidden files (dotfiles)"
      - name: ignore_system
        type: bool
        default: true
        description: "Ignore system/OS-generated files"
      - name: entity_type
        type: string
        default: "file"
        description: "Entity type to watch: file, directory, or all"
    context:
      - name: From
        type: string
        description: "Original path"
      - name: To
        type: string
        description: "New path"
      - name: IsDirectory
        type: bool
        description: "Whether the entity is a directory"
      - name: TriggeredOn
        type: string
        description: "ISO 8601 timestamp of the event"

  - name: FileAttributeChanged
    description: "Fires when a file or directory's metadata/attributes change"
    options:
      - name: path
        type: string
        default: ""
        description: "Directory to watch. Defaults to the recurfile's directory."
      - name: recursive
        type: bool
        default: false
        description: "Watch subdirectories"
      - name: filter
        type: list
        default: []
        description: "Glob patterns to match"
      - name: ignore_hidden
        type: bool
        default: true
        description: "Ignore hidden files (dotfiles)"
      - name: ignore_system
        type: bool
        default: true
        description: "Ignore system/OS-generated files"
      - name: entity_type
        type: string
        default: "file"
        description: "Entity type to watch: file, directory, or all"
    context:
      - name: FilePath
        type: string
        description: "Path of the affected entity"
      - name: IsDirectory
        type: bool
        description: "Whether the entity is a directory"
      - name: TriggeredOn
        type: string
        description: "ISO 8601 timestamp of the event"
```

# Notes

- All five triggers share the same options schema, making them ideal for group-level option hoisting in recurfiles.
- `path` specifies the directory to watch. When empty (the default), it falls back to the recurfile's parent directory. This allows a recurfile placed in a project root to watch that project without explicit path configuration.
- `entity_type` accepts `"file"` (default), `"directory"`, or `"all"`.
- `"File"` in trigger names refers to filesystem entities broadly, including directories when `entity_type` is set accordingly.
- `ignore_hidden` and `ignore_system` default to `true` to reduce noise from OS and application-generated file activity.
- `recursive` defaults to `false` — watches only the target directory unless explicitly enabled.
- `PermanentlyDeleted` on `FileDeleted` distinguishes true deletions from trash/recycle bin moves. Accuracy is platform-dependent.
- `FileMoved` provides both `From` and `To` context variables for the original and new paths.
