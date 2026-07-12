---
title: "Plugins"
weight: 4
description: "Plugin architecture, first-party plugins, and installation"
---

Recur uses an exec-based plugin architecture. Each plugin is a standalone binary with a `manifest.yaml` that declares its triggers, trigger context variables, actions, and options. The manifest is used to generate stubbed configurations, validate configuration files, and populate context variables when triggers fire.

The first-party plugins listed below are maintained in their own repositories under the [directedbits](https://github.com/directedbits) org — they are no longer bundled inside the core recur repository. Each repo publishes release archives you can install directly.

## First-party Plugins

| Plugin | Triggers | Actions | Description | Repository |
|--------|----------|---------|-------------|------------|
| **fileevents** | `FileCreated`, `FileModified`, `FileDeleted`, `FileMoved`, `FileAttributeChanged` | -- | File system event monitoring | [recur-fileevents](https://github.com/directedbits/recur-fileevents) |
| **timer** | `cron`, `interval` | -- | Cron schedule and interval triggers | [recur-timer](https://github.com/directedbits/recur-timer) |
| **webhook** | `WebhookReceived` | -- | HTTP/HTTPS webhook receiver with HMAC verification | [recur-webhook](https://github.com/directedbits/recur-webhook) |
| **mqtt** | `MessageReceived` | `publish` | MQTT subscribe and publish | [recur-mqtt](https://github.com/directedbits/recur-mqtt) |
| **calendar** | `EventUpcoming`, `EventStarted`, `EventEnded` | -- | iCal/ICS calendar polling | [recur-calendar](https://github.com/directedbits/recur-calendar) |
| **devicemonitor** | `DeviceConnected`, `DeviceDisconnected` | -- | USB/device hotplug (Linux + Windows only) | [recur-devicemonitor](https://github.com/directedbits/recur-devicemonitor) |
| **docker** | `ContainerStarted`, `ContainerStopped`, `HealthChanged` | `ContainerStart`, `ContainerStop`, `ContainerRestart` | Docker container lifecycle | [recur-docker](https://github.com/directedbits/recur-docker) |

## Installing a plugin

Each plugin repo publishes release assets named `<plugin>-<tag>-<os>-<arch>.tar.gz`. Install one directly from its release URL:

```sh
recur plugin install https://github.com/directedbits/recur-timer/releases/download/v1.0.0/timer-v1.0.0-linux-amd64.tar.gz
```

Remote installs require the host to be in the `allowed_hosts` config list first:

```sh
recur config set allowed_hosts github.com
```

Plugins are copied or linked to `~/.config/recur/plugins/`. Any plugins found in that folder on daemon start are loaded and available to registered config files.
