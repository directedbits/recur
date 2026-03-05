---
title: "Quick Start"
weight: 2
description: "Your first recur automation in under a minute"
---

# Quick Start

## 1. Start the daemon

```sh
recur start
```

The daemon runs in the background and manages triggers and actions.

## 2. Create a recurfile

```sh
recur add MyProject cron --actions=shell --stub --edit
```

This creates a `recur.yaml` in the current directory with a cron trigger and shell action, pre-populated with options from the plugin manifests. Your editor opens for customization.

## 3. Register it

```sh
recur register recur.yaml
```

The daemon loads the recurfile and activates all triggers.

## 4. Check status

```sh
recur status
```

Shows daemon health, active trigger/action counts, and registered recurfiles.

## Next steps

- [Recurfile format](../configuration/recurfile-format/) — learn all the configuration options
- [Plugins](../plugins/) — see what triggers and actions are available
- [CLI reference](../cli/reference/) — full command documentation
