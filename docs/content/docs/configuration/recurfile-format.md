---
title: "Recurfile Format"
weight: 1
description: "How to write recur recurfiles"
---

Configuration files are YAML files that declare groups of triggers and actions. Triggers and their associated actions can be organized into groups within the file.

## Basic Structure

A recurfile contains one or more named groups. Each group has an `on:` block listing triggers and a `do:` block listing actions:

```yaml
Build: # Group name
  on:
    - type: cron
      name: "Nightly schedule"
      options:
        expression: "0 2 * * *"
      do:
        - shell: "nightly-build.sh"
```

Triggers and actions support an optional `name` field for labeling. Names are searchable by `recur inspect` and other commands.

## Shorthand vs Detailed Actions

Actions support two forms. Shorthand is a key-value pair consisting of the plugin name (case-insensitive) and the arguments to pass to the plugin's primary option:

```yaml
do:
  - shell: "echo hello"
```

The long form exposes additional options to configure the response. An optional `name` field can be added for identification:

```yaml
do:
  - type: shell
    name: "Build step"
    options:
      command: "echo hello"
      timeout: "30"
```

## Group-Level vs Trigger-Level Actions

Actions can be defined at the group level. Actions in the group's `do:` block run when any trigger in the group fires:

```yaml
# Group-level: both triggers run the same action
Monitoring:
  on:
    - type: cron
      options:
        expression: "*/5 * * * *"
    - type: WebhookReceived
      options:
        port: "9090"
        path: "/check"
  do:
    - shell: "health-check.sh"
```

When a trigger has its own `do:` block, it ignores all group-level actions. While actions can be defined at both levels within the same group, it is recommended to separate them into their own groups for clarity.

## Template Variables

Triggers provide information that is available as variables to actions using Go template syntax:

```yaml
WebhookLog:
  on:
    - type: WebhookReceived
      options:
        port: "8080"
        path: "/events"
  do:
    - shell: "echo '{{ .RequestMethod }} {{ .RequestPath }} from {{ .RemoteAddr }}' >> events.log"
```

Available variables are declared in the trigger's plugin manifest -- see each plugin's repository for its context variables:

- [timer](https://github.com/directedbits/recur-timer)
- [webhook](https://github.com/directedbits/recur-webhook)
- [mqtt](https://github.com/directedbits/recur-mqtt)
- [calendar](https://github.com/directedbits/recur-calendar)
- [devicemonitor](https://github.com/directedbits/recur-devicemonitor)

For convenience, when an action is fired via `recur test`, the special variable `{{ .Test }}` is provided with the boolean value `true`.

## Group Options

Groups can declare shared options that are passed to all triggers. When a shared option does not exist for a trigger, it is ignored and the daemon logs a warning:

```yaml
ProjectWatch:
  options:
    project_root: "/opt/myproject"
  on:
    - type: cron
      options:
        expression: "@hourly"
  do:
    - shell: "cd {{ .project_root }} && make check"
```

It is recommended to only use shared options for triggers from the same plugin.

## The `--stub` Flag

Using `recur add --stub [plugin]` generates a config file with pre-populated options:

```yaml
Group:
  on:
    - type: MessageReceived
      options:
        broker: ""          # Broker URL (e.g. tcp://localhost:1883)
        topic: ""           # Topic filter (supports + and # wildcards)
        # qos: "0"          # QoS level (0, 1, or 2)
        # keepalive: "30"   # Keepalive interval in seconds
  do:
    - publish: ""           # Topic to publish to
```

These options are generated from and validated against the manifest provided by the plugin. Required options appear uncommented with empty values. Optional ones are commented out with their defaults.

## Multiple Groups in One File

Multiple groups can be declared within a single file:

```yaml
Build:
  on:
    - type: cron
      options:
        expression: "0 2 * * *"
  do:
    - shell: "make build"

Deploy:
  on:
    - type: WebhookReceived
      options:
        port: "9090"
        path: "/deploy"
        secret: "my-secret"
  do:
    - shell: "deploy.sh"

Notify:
  on:
    - type: MessageReceived
      options:
        broker: "tcp://localhost:1883"
        topic: "alerts/#"
  do:
    - shell: "notify-send 'Alert' '{{ .Payload }}'"
```

## Scopes

Configuration files can live anywhere. The recognized basenames are
`recurfile`, `recurfile.yaml`, and `recurfile.yml` (case-insensitive on the
entire name; the `.yaml`/`.yml` extension is optional). Two common locations:

- **Local** (`Recurfile.yaml` in any directory) -- directory-specific triggers
- **User** (`~/.config/recur/Recurfile.yaml`) -- personal/global triggers

The `recur add` command targets these with `--local` (default) or `--user`.

All configuration files are combined when loaded into the daemon. Groups sharing a name will be merged. Any conflicts in trigger or action names from different plugins are ignored with a warning logged. Use `recur status` and `recur list` to view any issues.
