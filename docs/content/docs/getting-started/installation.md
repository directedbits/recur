---
title: "Installation"
weight: 1
description: "Install recur from binaries, Go, or source"
---

## Pre-built binaries

Download the latest release for your platform from the [Releases](https://github.com/directedbits/recur/releases) page.

Available platforms: Linux (amd64, arm64, armv7, i386), macOS (amd64, arm64), Windows (amd64, arm64).

Each release archive contains `recur` (CLI) and `recurd` (daemon). Plugins are distributed separately — the first-party plugins live in their own repositories under the [directedbits](https://github.com/directedbits) org and are installed via `recur install` (see [Plugins](../plugins/)).

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

### Installing plugins

The first-party plugins are maintained in their own repositories under the
[directedbits](https://github.com/directedbits) org and published as release
archives. Install one from its release URL:

```sh
recur config set allowed_hosts github.com
recur install https://github.com/directedbits/recur-timer/releases/download/v0.1.0/timer-v0.1.0-linux-amd64.tar.gz
```

See [Plugins](../plugins/) for the full catalog and installation details.

### Protobuf generation

Requires [buf](https://buf.build/) v1.66.0+:

```sh
task generate
```

Generated `.pb.go` files are gitignored. Run `task generate` after cloning or changing `.proto` files.
