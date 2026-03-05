---
title: "Architecture"
weight: 7
description: "How recur works internally"
---

## Overview

Recur is composed of four main components that work together:

1. **Daemon** (`recurd`) -- Runs in the background, managing the plugin lifecycle and trigger/action registry.
2. **Configuration files** -- YAML recurfiles that declare what triggers to listen for and what actions to take.
3. **Plugins** -- Standalone binaries that implement triggers and actions via an exec-based contract (stdin JSON for config, gRPC callback for events).
4. **CLI** (`recur`) -- Communicates with the daemon via a Unix socket, with graceful fallback to file reads when the daemon is not running.

## Data Flow

```
  recurfile.yaml
       |
       v
  recur register ──> recurd (daemon)
                      |
                      ├── loads plugin binaries
                      ├── sends config via stdin JSON
                      ├── plugins call back via gRPC when triggers fire
                      └── daemon executes matching actions
```

## State Persistence

State is persisted to `~/.config/recur/state/state.json` using atomic writes, so triggers and their error counts survive daemon restarts.

## Plugin Lifecycle

1. On startup, the daemon scans `~/.config/recur/plugins/` for plugin binaries.
2. Each plugin's `manifest.yaml` is read to discover available triggers and actions.
3. When a recurfile is registered, the daemon starts the relevant plugin processes.
4. Plugins receive their configuration via stdin as JSON.
5. When a trigger fires, the plugin notifies the daemon via a gRPC callback.
6. The daemon executes the associated actions, expanding template variables from the trigger's context.

See the [Plugin Protocol](../reference/plugin-protocol/) reference for the full communication specification.
