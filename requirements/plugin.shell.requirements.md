shell Action Plugin Requirements
-----------------------------------

# Overview

The initial action plugin provides shell command and script execution. It is the default action plugin and supports both inline commands and script files, auto-detecting which is provided.

No runtime system dependencies beyond the configured shell.

# Manifest

```yaml
name: shell
namespace: core.shell
version: "1.0.0"
description: "Execute shell commands and scripts"

actions:
  - name: shell
    description: "Execute a shell command or script"
    options:
      - name: command
        type: string
        shorthand: true
        description: "Inline command or path to a script file. Auto-detected."
      - name: shell
        type: string
        default: ""
        description: "Shell to use. Defaults to the daemon's DefaultShell configuration."
      - name: params
        type: list
        default: []
        description: "Additional parameters passed to the command, applied in order."
      - name: working_dir
        type: string
        default: ""
        description: "Working directory for the command. Defaults to the recurfile's directory."
      - name: env
        type: map
        default: {}
        description: "Additional environment variables as key-value pairs. Merged with context file variables."
      - name: timeout
        type: string
        default: "0"
        description: "Maximum execution time before killing the process. Uses Go duration format (e.g., '30s', '5m', '1h30m'). '0' means no timeout."
```

# Notes

- `command` auto-detects inline commands vs script file paths. If the value is a path to an existing file, it is executed as a script; otherwise it is treated as an inline command.
- `shell` defaults to the daemon's `DefaultShell` configuration value when empty.
- `working_dir` defaults to the directory containing the recurfile when empty.
- `env` variables are merged with context file variables (from the trigger's context file fallback). Explicit `env` values take precedence over context file values on conflict. Conflicts are detected at registration time and a warning is emitted identifying which `env` keys shadow trigger context variables.
- `timeout` uses Go's `time.ParseDuration` format. `"0"` means no timeout.
- Template substitution (e.g., `{{.FilePath}}`) is resolved in `command` and `params` values before execution.

# Future Considerations

- Shell variable expansion support (similar to Docker Compose variable substitution) — deferred.
