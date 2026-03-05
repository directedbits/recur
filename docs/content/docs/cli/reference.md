---
title: "Command Reference"
weight: 1
description: "Complete CLI command reference"
---

## Daemon

```sh
recur start                  # Start daemon in background
recur start --foreground     # Start in foreground (logs to stdout)
recur stop                   # Graceful shutdown
recur restart                # Stop + start
recur status                 # Health and active counts
recur status --verbose       # Include file paths (config, socket, etc.)
```

## Configuration Files

```sh
recur add [group] [trigger]  # Create/append to a config file
recur add --stub --edit      # Pre-populate options from manifests, open editor
recur register [file]        # Register with daemon
recur verify [file]          # Validate without registering
recur deregister <id>        # Remove from daemon
```

## Inspection

```sh
recur list triggers          # List active triggers
recur list actions           # List active actions
recur list triggers --suspended  # Show only suspended
recur list plugins           # Show installed plugins
recur list recurfiles        # Show registered config files
recur inspect trigger <id>   # Full trigger details
recur inspect plugin <id>    # Plugin manifest and status
```

## Testing

```sh
recur test action <id>       # Execute an action manually
```

## Suspend / Resume

```sh
recur suspend trigger <id>   # Pause a trigger
recur resume trigger <id>    # Resume a paused trigger
```

## Plugins

```sh
recur install ./path/to/plugin      # Copy plugin to ~/.config/recur/plugins/
recur install ./plugin --link       # Symlink instead of copy
recur install https://example.com/plugin.tar.gz  # Download and install
recur uninstall <id>                # Remove plugin
```

## Settings

```sh
recur config get                    # Show all config values
recur config get error_threshold    # Show a specific value
recur config set error_threshold 10
recur config delete error_threshold # Revert to default
```

Values show their source:

```
error_threshold          = 10
concurrency_mode         = queue (inherited from default)
trigger_error_threshold  = 10 (inherited from error_threshold)
```

## Shell Completions

Cobra generates completions for bash, zsh, fish, and PowerShell:

```sh
# Bash -- add to ~/.bashrc
eval "$(recur completion bash)"

# Zsh -- add to ~/.zshrc
eval "$(recur completion zsh)"

# Fish
recur completion fish | source

# Or write to a file for faster shell startup
recur completion bash > ~/.local/share/bash-completion/completions/recur
```
