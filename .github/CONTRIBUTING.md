---
title: "Contributing"
weight: 8
description: "Build, test, and contribute to recur"
---

# Contributing

Requires Go 1.25+, [Task](https://taskfile.dev/) for the task runner, and [buf](https://buf.build/) v1.66.0+ for protobuf generation.

## Environment Preparation:

Run the platform setup script to install the tools automatically:

```sh
./setup/linux.sh
```

```PowerShell
./setup/windows.ps1
```

## Build & Test

```sh
task build          # Build recur + recurd
task test           # Unit tests
task test:plugins   # Plugin unit tests
task test:e2e       # End-to-end tests
task test:all       # All of the above
```

Plugins build individually:

```sh
task build:timer
task build:webhook
task build:calendar
task build:devicemonitor
task build:mqtt
```

Protobuf generation:

```sh
task generate
```

Generated `.pb.go` files are gitignored. Run `task generate` after cloning or changing `.proto` files.

## Project Layout

```
src/
  app/
    recur/                 The recur CLI binary
      main.go
      cli/                 Cobra commands
      text/                CLI-local Levenshtein helper
    recurd/                The recurd daemon binary
      main.go
      daemon/              Daemon core, gRPC handlers, registry
      triggerengine/       Trigger engine (drivers, router)
  domain/                  Entities, value objects, rules
    action/, config/, group/, plugin/, recurfile/, secret/, trigger/
  infra/                   I/O implementations: <impl>/<subdomain>/
    yaml/{recurfile,manifest,config}/
    jsonfile/state/
    grpc/{server,client,v1}/
    subprocess/executor/
    os/{process,defaults}/
    terminal/display/
    secret/{env,file,keyring,composite}/
    fs/{plugin,atomicfile}/
plugins/                   External trigger/action plugins (each a standalone binary)
requirements/              Design specs
test/
  e2e/                     End-to-end tests
```

See **[src/ARCHITECTURE.md](src/ARCHITECTURE.md)** for the full layering
rules, allowed import directions, the implementation/subdomain naming
rule, and a decision tree for "where does this code go?". The short
version: dependencies flow inward (`app → infra → domain`); `domain`
knows nothing about `infra` or `app`; `infra` knows nothing about `app`;
each binary lives under `src/app/<name>/`.

## Code Style

### Naming

- Spell out names fully. Avoid unnecessary abbreviations (`mutex` not `mu`, `context` not `ctx` where it's a struct field, `config` not `cfg`).
- Go-standard abbreviations in function signatures are fine where idiomatic (e.g., `ctx context.Context` as a parameter).
- `PascalCase` for exported Go types and functions.
- `snake_case` in YAML files (recurfiles, manifests, config).
- Package names match their directory name.

### General

- Standard Go conventions: `gofmt`, `go vet`, idiomatic error handling.
- No unnecessary abstractions — three similar lines is better than a premature helper.
- Only add comments where logic isn't self-evident. Don't add docstrings to code you didn't change.
- Don't add error handling for scenarios that can't happen. Trust internal code and framework guarantees.

## Commits

- Small, buildable commits — each commit should compile and pass tests.
- One logical change per commit.
- Commit message: short summary line explaining *why*, not just *what*.
- Stage files explicitly by name (`git add file1 file2`). Never `git add -A` or `git add .`.

## Testing

- All features need unit tests.
- End-to-end tests go in `test/e2e/`.
- E2e tests use a shared `TestMain` that builds `recur` and `recurd` once for the entire suite.
- Tests should not depend on external services or network access (except e2e daemon tests which use localhost sockets).

## Plugins

- All trigger and action plugins are **external** — standalone binaries using the exec-based plugin contract (stdin/stdout JSON, gRPC callback to daemon).
- No in-process/built-in trigger or action plugins.
- Each plugin directory contains: Go source, `manifest.yaml`, and builds to a single binary.
- Plugin manifest must be included in the build artifacts to be installable by the daemon.
- Plugin manifests declare triggers, actions, options, and context variables.
- Plugins are installed to `~/.config/recur/plugins/` (copy or symlink with `--link`).

## Documentation

- Design specs live in `requirements/` and are committed with the code they describe.
- Documentation changes go in the same commit as the code they document.

## Architecture Notes

- **Config directory:** `~/.config/recur/` (no XDG). Contains `config.yaml`, `state/`, `plugins/`, `run/`.
- **Plugin contract:** exec-based, process-isolated. Daemon launches plugin binary, passes options via stdin JSON, plugin reports events back via gRPC.
- **Persistence:** JSON flat file with atomic writes (timestamped temp file + rename).
- **gRPC:** internal daemon communication only. Plugins call back to the daemon socket. CLI uses gRPC when daemon is running, falls back to file reads when not.
- **Recurfile format:** YAML with ordered group parsing. Actions support detailed and shorthand forms.
