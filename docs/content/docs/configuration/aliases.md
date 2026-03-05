---
title: "Aliases"
weight: 3
description: "Plugin namespace aliases for recurfiles"
---

Aliases map short names to plugin namespaces, reducing repetition in recurfiles and handling conflicts. They can be defined at the file level (shared across all groups) or at the group level, with group-level aliases taking precedence.

## File-Level Aliases

File-level aliases are available to all groups in the recurfile:

```yaml
aliases:
  fs: com.example.filesystem
  sh: com.example.shell

Group1:
  on:
    - type: fs.FileCreated       # resolves via "fs" alias
      options:
        path: "./src"
  do:
    - shell: "make build"        # resolves via "sh" alias
```

## Group-Level Aliases

Groups can define their own aliases that override file-level aliases within that group:

```yaml
aliases:
  fs: com.example.filesystem

Group1:
  on:
    - type: fs.FileCreated       # resolves to com.example.filesystem
      options:
        path: "./src"
  do:
    - shell: "make build"

Group2:
  aliases:
    fs: com.another.filesystem   # overrides the file-level fs alias within this group
  on:
    - type: cron
      options:
        expression: "0 2 * * *"
  do:
    - publish: "deploy/status"
```

Group-level aliases override file-level aliases when both define the same key.
