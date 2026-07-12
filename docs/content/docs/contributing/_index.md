---
title: "Contributing"
weight: 8
description: "Build, test, and contribute to recur"
---

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
task test:e2e       # End-to-end tests
task test:all       # All of the above
```

The first-party plugins are no longer part of this repository — they are
built, tested, and released from their own repositories under the
[directedbits](https://github.com/directedbits) org.

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

Two distinct doc trees serve different audiences.

### `requirements/` — specs for implementers

Design specifications for the *people (or agents) building the
feature*. Each file pins down behavior in enough detail that a
contributor reading only the spec could reasonably (re)produce a
working implementation: exact identifiers, contracts, invariants,
edge cases, command shapes, lifecycle states, file paths, exit
codes. Examples in this repo: `daemon-lifecycle.md`, `plugin.calendar.md`,
`plugin-protocol.md`, `cli-commands.md`.

Write or update the spec **before or alongside** the code, in the
same PR. This serves two purposes:

- **Agent-implementable.** Specs are the input format for AI-assisted
  feature work. A spec that's tight enough to ship is also tight
  enough for an agent to draft from. If you can't write the spec,
  the feature isn't ready to build.
- **Easier review.** Reviewers verify "the code matches the spec"
  rather than re-deriving intent from the implementation. Disagreement
  about scope surfaces on the spec, not in line-by-line PR comments
  on code that already exists.

### `docs/content/docs/` — user-facing documentation

The reference site at https://directedbits.github.io/recur/.
Audience: operators and end-users who want to install, configure,
and use recur. Tone is task-oriented, not exhaustive — leave the
internal contract details to `requirements/`.

When in doubt: if a future contributor or agent would need it to
**build** the feature, it's `requirements/`; if a user would read
it to **use** the feature, it's `docs/`. Most non-trivial features
need both.

Documentation changes (specs and user docs) go in the same commit
as the code they describe.

Update the docs site whenever a change is **user-observable** or
affects how someone integrates with recur. Update triggers:

- **Contract changes** — gRPC RPCs, recurfile YAML schema, manifest
  format, env vars passed to plugins, exit-code semantics, CLI flag
  additions/removals/renames.
- **Architecture or invariants** — package layer rules, request flow,
  state persistence guarantees, atomic-write ordering.
- **New features** — a new plugin, trigger type, action type, or
  built-in command.
- **Behavior changes** — anything a user might notice and want to
  read about: defaults, error messages, log format, where files live.
- **Configuration surface** — new daemon config keys, plugin options,
  recurfile fields.

Skip the docs for internal refactors, dependency bumps, test-only
changes, and bug fixes that restore documented behavior.

### Where to edit

Some pages are **synced** from a source-of-truth README that lives
next to the code; editing the rendered copy under `docs/content/docs/`
directly will be clobbered by `task docs:sync` on the next build.

**Edit the source, then run `task docs:sync`:**

| Page | Source |
|------|--------|
| `docs/content/docs/contributing/_index.md` | `.github/CONTRIBUTING.md` |
| `docs/content/docs/deployment/docker.md`   | `examples/docker/README.md` |

**Edit `docs/content/docs/` directly** for anything else — the
homepage, getting started, configuration, CLI, reference, deployment
(other than `docker.md`), plugins section index, developer guide.

The list of synced sources is in `scripts/sync-docs.sh`; add new
entries there if you introduce another README that should mirror
into the site.

## Architecture Notes

- **Config directory:** `~/.config/recur/` (no XDG). Contains `config.yaml`, `state/`, `plugins/`, `run/`.
- **Plugin contract:** exec-based, process-isolated. Daemon launches plugin binary, passes options via stdin JSON, plugin reports events back via gRPC.
- **Persistence:** JSON flat file with atomic writes (timestamped temp file + rename).
- **gRPC:** internal daemon communication only. Plugins call back to the daemon socket. CLI uses gRPC when daemon is running, falls back to file reads when not.
- **Recurfile format:** YAML with ordered group parsing. Actions support detailed and shorthand forms.
