---
title: "Plugins"
weight: 4
description: "Plugin architecture, included plugins, and installation"
---

Recur uses an exec-based plugin architecture. Each plugin is a standalone binary with a `manifest.yaml` that declares its triggers, trigger context variables, actions, and options. The manifest is used to generate stubbed configurations, validate configuration files, and populate context variables when triggers fire.

## Included Plugins

| Plugin | Triggers | Actions | Description |
|--------|----------|---------|-------------|
| [**fileevents**](fileevents/) | `FileCreated`, `FileModified`, `FileDeleted`, `FileMoved`, `FileAttributeChanged` | -- | File system event monitoring |
| [**timer**](timer/) | `cron`, `interval` | -- | Cron schedule and interval triggers |
| [**webhook**](webhook/) | `WebhookReceived` | -- | HTTP/HTTPS webhook receiver with HMAC verification |
| [**mqtt**](mqtt/) | `MessageReceived` | `publish` | MQTT subscribe and publish |
| [**calendar**](calendar/) | `EventUpcoming`, `EventStarted`, `EventEnded` | -- | iCal/ICS calendar polling |
| [**devicemonitor**](devicemonitor/) | `DeviceConnected`, `DeviceDisconnected` | -- | USB/device hotplug (Linux + Windows only) |

## Installing Plugins

Plugins can be installed from a local directory, archive, or URL:

```sh
# From a local directory
recur install ./my-plugin/

# Symlink for development
recur install ./my-plugin/ --link

# From a .tar.gz or .zip archive
recur install ./my-plugin.tar.gz

# From a URL (must match allowed_hosts in config)
recur install https://github.com/user/repo/releases/download/v1.0/plugin.tar.gz
```

Plugins are copied or linked to `~/.config/recur/plugins/`. Any plugins found in that folder on daemon start are loaded and available to registered config files.

Remote installs require the host to be in the `allowed_hosts` config list:

```sh
recur config set allowed_hosts "github.com,gitlab.com"
```
