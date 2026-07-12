# Source layout and layering

This file is the rulebook for where code goes. If you are about to add a
new package, function, or type and you're not sure where it belongs, this
is the document to consult.

## Top-level layout

```
src/
  app/                            # orchestration: each binary lives here
    recur/                        # the recur CLI binary
      main.go                     # package main
      cli/                        # package cli — Cobra commands
      text/                       # CLI-local helpers (Levenshtein for suggestions)
    recurd/                       # the recurd daemon binary
      main.go                     # package main
      daemon/                     # package daemon — lifecycle, registry, gRPC handlers
      triggerengine/              # package triggerengine — event engine + drivers

  domain/                         # abstractions: entities, value objects, rules
    action/                       # action entity
    config/                       # Config struct + key schema (impl-free)
    group/                        # group entity
    plugin/                       # plugin domain types + lookups
    recurfile/                    # recurfile entity + Raw* parsing shapes
                                  # + resolve/validate/merge/builder
    secret/                       # SecretDef + Resolver interface (impl-free)
    trigger/                      # trigger entity

  infra/                          # implementations: <impl>/<subdomain>
    yaml/
      recurfile/                  # package recurfileyaml — YAML parser
      manifest/                   # package manifestyaml — plugin manifest parser
      config/                     # package configyaml — config file I/O
    jsonfile/
      state/                      # package statejsonfile — state.json persistence
    grpc/
      server/                     # package servergrpc — inbound ACL (recurd)
      client/                     # package clientgrpc — outbound ACL (recur, plugins)
      v1/                         # proto-generated
    subprocess/
      executor/                   # package executorsubprocess — exec.Command + pipes
    os/
      process/                    # package processos — PID file, signals, sockets
      defaults/                   # package defaultsos — platform-specific config defaults
    terminal/
      display/                    # package displayterminal — entity + error rendering
    secret/                       # implementations of domain/secret.Resolver
      env/                        # package secretenv
      file/                       # package secretfile
      keyring/                    # package secretkeyring (+ OSKeyring provider)
      composite/                  # package secretcomposite (dispatch + redactor)
    fs/                           # filesystem code
      plugin/                     # package pluginfs — discovery, install, archive
      atomicfile/                 # package atomicfile — crash-safe write helper
```

Three top-level src dirs: `app/`, `domain/`, `infra/`. No `cmd/`, no
`shared/` — each binary's code is colocated under `app/<binary>/`, and
generic helpers either live next to their only consumer or in the
relevant `infra/<impl>/` directory.

## Allowed import directions

| From            | May import                                                            |
|-----------------|-----------------------------------------------------------------------|
| `domain/*`      | stdlib, other `domain/*`, `pkg/*`                                     |
| `infra/*/*`     | `domain/*`, other `infra/*/*`, `pkg/*`, third-party                   |
| `app/<binary>/` | `domain/*`, `infra/*/*`, `pkg/*`, third-party                         |
| `app/<binary>/main.go` | the binary's own subpackages + `infra` + `domain` (wiring only) |
| plugin modules | `src/infra/grpc/client` only (the daemon callback client)             |

**`infra` must never import `app`.** **`domain` must never import `app`
or `infra`.** If you find yourself reaching across, the code is in the
wrong layer — move it instead of breaking the rule.

## The implementation/subdomain naming rule

Every `infra/<impl>/<subdomain>/` package uses a concatenated package name
that puts the subdomain first and the implementation second:

| Disk path                       | Package name        | Reads as |
|---------------------------------|---------------------|----------|
| `infra/yaml/recurfile/`         | `recurfileyaml`     | the YAML recurfile parser |
| `infra/yaml/manifest/`          | `manifestyaml`      | the YAML manifest parser |
| `infra/jsonfile/state/`         | `statejsonfile`     | the JSON-file state persister |
| `infra/subprocess/executor/`    | `executorsubprocess` | the subprocess executor |
| `infra/os/process/`             | `processos`         | the OS process helpers |
| `infra/terminal/display/`       | `displayterminal`   | the terminal display |
| `infra/secret/env/`             | `secretenv`         | the env-var secret resolver |

The suffix is kept even when only one implementation exists. It marks
this package as an **Anti-Corruption Layer** — code that translates
between an external world (an OS, a wire format, a terminal) and the
domain. Reading `processos.PIDPath(...)` immediately conveys "this is
the OS-side implementation of process management." Reading
`process.PIDPath(...)` reads like a domain call and hides the boundary.

### Documented exceptions to the rule

A few `infra/<impl>/<subdomain>/` packages deviate deliberately:

- `infra/grpc/{server,client,v1}` — `server` and `client` are *roles*
  (inbound vs outbound ACL), not subdomains. The split-by-direction
  shape was chosen because each role owns its own proto↔domain
  conversion; that's cleaner than a separate `mapping/` folder that
  would name the mechanism instead of the role.
- `infra/secret/composite/` — the composite resolver *combines* the
  other resolvers; it's dispatch + redaction, not really "an
  implementation of a domain abstraction."
- `infra/fs/atomicfile/` — a low-level filesystem primitive used by
  *other* infra packages (`infra/yaml/config`, `infra/jsonfile/state`),
  not a subdomain. The dir-level umbrella (`infra/fs/`) groups it with
  the other filesystem code; the package name stays as `atomicfile`
  because there's no implementation choice to disambiguate.

These exceptions exist for good reasons. Copying their *shape* as a
default would be wrong — most new `infra/` packages should follow the
strict `<impl>/<subdomain>` pattern with a concatenated package name.

## "Where does this code go?"

```
new code …
│
├─ defines a struct that represents user data / config / an entity?
│  └─ domain/<aggregate>/         (one package per aggregate root)
│
├─ encodes a rule of the system that doesn't depend on I/O?
│  │  (alias resolution, option merging, validation, ID seeding,
│  │   the daemon<plugin<group<trigger precedence chain, …)
│  └─ domain/<aggregate>/
│
├─ reads or writes anything outside the program?
│  │  (filesystem, network, OS process, subprocess, syscall, stdout/err)
│  └─ infra/<impl>/<subdomain>/   (package <subdomain><impl>)
│
├─ converts a domain value to/from a wire format?
│  │  (proto messages, YAML, JSON, environment variables)
│  └─ infra/<format>/<subdomain>/  (eg. infra/grpc/server, infra/yaml/recurfile)
│
├─ formats a value for a human reader?
│  └─ infra/terminal/display/
│
├─ wires existing pieces together to satisfy a user request?
│  │  (a CLI command, a gRPC handler, a daemon subsystem)
│  └─ app/<binary>/<subpackage>/
│
├─ generic algorithm with no project knowledge?
│  │  (Levenshtein, atomic file rename)
│  ├─ used by one binary only?      app/<binary>/<helper>/
│  └─ used across infra packages?   infra/<impl>/<helper>/  (eg. infra/fs/atomicfile)
│
└─ a brand-new entry point?
   └─ src/app/<binary>/main.go + a per-binary subpackage tree
```

## Naming

| Convention                                                          |
|---------------------------------------------------------------------|
| Directory layout: `infra/<impl>/<subdomain>/`                       |
| Package name: `<subdomain><impl>` (e.g. `recurfileyaml`)             |
| Domain entity package: short name (e.g. `recurfile`, `plugin`)       |
| Parsed YAML shapes (in domain/recurfile): `RawFile`, `RawGroup`, … to disambiguate from entities |
| `PascalCase` for exported Go identifiers                            |
| `snake_case` for YAML keys (recurfiles, manifests, config)          |
| Spell out names; avoid abbreviations (`mutex` not `mu`, `config` not `cfg` in struct fields) |

## Plugin isolation

Plugins are standalone Go modules (each has its own `go.mod`) maintained in
their own repositories — they are no longer bundled in this repo. The daemon
treats them as third-party binaries.

**Plugins must not import anything from `src/` except
`src/infra/grpc/client`** (the daemon callback client). If a plugin
needs a helper that exists in `src/infra/<x>/`, copy the helper into
the plugin instead of importing across the module boundary. This rule
keeps plugins compilable and shippable independently, and prevents one
plugin's needs from quietly expanding `src/infra/` API surface.

## Cross-layer contracts that look unusual

A few seams break the pure top-down flow on purpose. None of them violate
the import rules — the indirection is what keeps them legal.

- **`domain/recurfile.RegisterParser`**: The merge logic in
  `domain/recurfile/merge.go` needs to parse YAML to validate inputs,
  but YAML parsing is infra. The infra parser calls `RegisterParser` at
  init to hand the domain a `func([]byte) (*RawFile, error)` callback.
  Domain holds the function pointer; infra owns the implementation.

- **`pkg/config`**: Generic config-store library that both
  `infra/yaml/config` and `domain/recurfile/builder.go` import. It has
  no project knowledge, so domain can safely depend on it.

## Verification

```sh
# domain has no app/infra imports
go list -deps -f '{{.ImportPath}} -> {{join .Imports " "}}' ./src/domain/... \
  | grep -E 'github.com/directedbits/recur/src/(app|infra)/' && echo VIOLATION

# infra has no app imports
go list -deps -f '{{.ImportPath}} -> {{join .Imports " "}}' ./src/infra/... \
  | grep 'github.com/directedbits/recur/src/app/' && echo VIOLATION

```

Both commands should print nothing.

The plugin import rule (plugins import only `src/infra/grpc/client` from
`src/`) is now enforced in each plugin's own repository, since first-party
plugins are no longer bundled here.
