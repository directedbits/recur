---
title: "Installation"
weight: 1
description: "Install recur from binaries, Go, or source"
---

# Installation

## Pre-built binaries

Download the latest release for your platform from the [Releases](https://github.com/directedbits/recur/releases) page.

Available platforms: Linux (amd64, arm64, armv7, i386), macOS (amd64, arm64), Windows (amd64, arm64).

Each release archive contains `recur` (CLI), `recurd` (daemon), and a `plugins/` directory with bundled plugins.

## Using Go

With Go 1.25+:

```sh
go install github.com/directedbits/recur/src/app/recur@latest   # CLI (recur)
go install github.com/directedbits/recur/src/app/recurd@latest  # Daemon (recurd)
```

## Building from source

Requires Go 1.25+ and [Task](https://taskfile.dev/).

```sh
git clone https://github.com/directedbits/recur.git
cd recur
task build          # outputs bin/recur and bin/recurd
```

### Building plugins

Bundled plugins build individually:

```sh
task build:timer
task build:webhook
task build:mqtt
task build:calendar
task build:devicemonitor
task build:docker
```

Install a plugin into the daemon's plugin directory:

```sh
recur install ./bin/timer/
```

### Protobuf generation

Requires [buf](https://buf.build/) v1.66.0+:

```sh
task generate
```

Generated `.pb.go` files are gitignored. Run `task generate` after cloning or changing `.proto` files.
