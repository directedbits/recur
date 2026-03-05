# Recur

[![CI](../../actions/workflows/ci.yml/badge.svg)](../../actions/workflows/ci.yml)
[![Lint](../../actions/workflows/lint.yml/badge.svg)](../../actions/workflows/lint.yml)
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL_2.0-brightgreen.svg)](LICENSE)

Recur is a cross-platform configuration-driven automation toolset consisting of a daemon and CLI supported by a flexible plugin system. It is written in Go and runs locally on the system.

Simply define what events to wait for — file changes, cron schedules, webhooks, MQTT messages, calendar events, USB devices, or Docker containers — and how to respond.

## Install

Pre-built binaries for Linux (amd64, arm64, armv7, i386) and macOS (amd64, arm64) are available on the [Releases](../../releases) page. Each release ships a sha256 checksum file and a Sigstore cosign signature — see [VERIFYING.md](VERIFYING.md) for the verify commands.

Using Go 1.25+:

```sh
go install github.com/directedbits/recur/src/app/recur@latest   # CLI
go install github.com/directedbits/recur/src/app/recurd@latest  # Daemon
```

Building from source:

```sh
git clone https://github.com/directedbits/recur.git
cd recur
task build    # outputs bin/recur and bin/recurd
```

## Quick Start

```sh
# Start the daemon
recur start

# Create a config file with stubbed values for a FileCreated trigger and shell execution action
recur add MyProject FileCreated --actions=shell --stub --edit

# Check status
recur status
```

The `add` command writes the recurfile and auto-registers it with the daemon. Use `recur register` to register an existing recurfile manually.

## Example Configuration

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
```

See the [recurfile format documentation](docs/content/docs/configuration/recurfile-format.md) for the full syntax including template variables, aliases, group options, and scopes.

## Plugins

| Plugin | Triggers | Actions | Description |
|--------|----------|---------|-------------|
| [**fileevents**](plugins/fileevents/) | `FileCreated`, `FileModified`, `FileDeleted`, `FileMoved`, `FileAttributeChanged` | — | File system event monitoring |
| [**timer**](plugins/timer/) | `cron`, `interval` | — | Cron schedule and interval triggers |
| [**webhook**](plugins/webhook/) | `WebhookReceived` | — | HTTP/HTTPS webhook receiver with HMAC verification |
| [**mqtt**](plugins/mqtt/) | `MessageReceived` | `publish` | MQTT subscribe and publish |
| [**calendar**](plugins/calendar/) | `EventUpcoming`, `EventStarted`, `EventEnded` | — | iCal/ICS calendar polling |
| [**devicemonitor**](plugins/devicemonitor/) | `DeviceConnected`, `DeviceDisconnected` | — | USB/device hotplug (Linux + Windows) |
| [**docker**](plugins/docker/) | `ContainerStarted`, `ContainerStopped`, `HealthChanged` | `ContainerStart`, `ContainerStop`, `ContainerRestart` | Docker container lifecycle |

## How It Works

1. The **daemon** (`recurd`) runs in the background, managing the plugin lifecycle and trigger/action registry
2. **Config files** declare what triggers to listen for and what actions to take
3. **Plugins** are standalone binaries that implement triggers and actions via an exec-based contract (stdin JSON for config, gRPC callback for events)
4. The **CLI** (`recur`) communicates with the daemon via a Unix socket, with graceful fallback to file reads when the daemon isn't running

State is persisted to `~/.config/recur/state/state.json` so triggers and their error counts survive restarts.

## Documentation

- [Getting Started](docs/content/docs/getting-started/)
- [Recurfile Format](docs/content/docs/configuration/recurfile-format.md)
- [CLI Reference](docs/content/docs/cli/reference.md)
- [Settings](docs/content/docs/configuration/settings.md)
- [Plugins](docs/content/docs/plugins/)
- [Architecture](docs/content/docs/architecture/)
- [Developer Guide](docs/content/docs/developers/)

## Contributing

See [CONTRIBUTING.md](.github/CONTRIBUTING.md) for development setup, coding standards, and how to submit changes.

## Security

See [SECURITY.md](.github/SECURITY.md) for the security policy and how to report vulnerabilities.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](.github/CODE_OF_CONDUCT.md).

## License

This project is licensed under the [Mozilla Public License 2.0](LICENSE).
