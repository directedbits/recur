# App Bundles

## Description

An **app bundle** is a single-file, distributable unit of recur automation: a
`.recur` archive containing a recurfile plus any local scripts the automation
needs. Installing a bundle unpacks it into a managed per-app directory and
registers its recurfile with the daemon, so an "app" (for example a habit
tracker that prompts and records responses) can be shared and installed as one
file rather than a loose recurfile plus scattered scripts.

App bundles deliberately introduce **no recurfile schema change** and no new
manifest format; a bundle is just a recurfile and its assets in a zip.

## Bundle Format

- A bundle is a **zip archive** with the `.recur` extension.
- The **recurfile is the single YAML file at the archive root** (`*.yaml` or
  `*.yml`). Any meaningful filename is accepted — its stem is used to name the
  app — so the strict `recurfile.*` naming convention is *not* required inside a
  bundle.
- YAML files in subdirectories are treated as app assets, never as the
  recurfile.
- All other files (scripts, data, nested directories) are carried verbatim;
  directory layout and file modes (e.g. the executable bit on scripts) are
  preserved on both pack and unpack.

**Recurfile selection rules:**
- Exactly one root YAML → that file is the recurfile.
- Multiple root YAML files → if exactly one is named per the conventional
  `recurfile.*` form it is used as a tie-breaker; otherwise the bundle is
  rejected as ambiguous.
- No root YAML → the bundle is rejected.

## Installed Layout

- Installed apps live under `~/.config/recur/app/<name>/`.
- A bundle is unpacked **as-is** into its app directory, preserving layout.
- `<name>` is a single path segment (no separators, not `.` or `..`).

## `recur app install`

Accepts a local `.recur` path **or** an `http(s)` URL.

**Source resolution:**
- A URL is downloaded to a temporary file first. The URL host must be permitted
  by the `allowed_hosts` config (same gate as `recur install` for plugins);
  otherwise the command errors with guidance to add the host.
- A local path is used directly.

**Name resolution (highest priority first):**
1. `--name <name>` flag.
2. The recurfile's filename stem — unless it is the generic `recurfile`, whose
   stem carries no meaning.
3. The bundle's filename stem (with `.recur` removed).

**Behavior:**
- The bundle is unpacked into a staging directory and validated first, so an
  invalid bundle never clobbers an already-installed app.
- If `~/.config/recur/app/<name>/` already exists, the user is prompted to
  overwrite or abandon. `--force` overwrites without prompting; a
  non-interactive (closed) stdin is treated as abandon.
- After unpacking, the recurfile is registered:
  - **Daemon running:** registered immediately via the daemon.
  - **Daemon stopped:** the app is left unpacked and a hint is printed; the
    daemon registers it on next start (see startup scan below). Installation
    still succeeds.

## `recur app list`

Lists installed apps (directories under `~/.config/recur/app/` that contain a
recurfile). When the daemon is running, each app is annotated as `registered`
or `not registered` by matching its recurfile path against the daemon's
registered recurfiles. `--json` emits the structured form.

## `recur app remove <name>`

Best-effort deregisters the app's recurfile from the daemon if it is running,
then removes `~/.config/recur/app/<name>/`. Removal proceeds even if the daemon
is stopped or the app was never registered.

## `recur app pack <dir>`

Creates a `.recur` bundle from a directory. The directory must contain a
recurfile (a single root YAML) or packing fails. Output defaults to
`<dir>.recur`; `--output/-o` overrides.

## Daemon Startup Scan

On startup, after replaying persisted state, the daemon scans
`~/.config/recur/app/` and registers the recurfile of any app that is not
already registered. This lets apps installed while the daemon was stopped
register themselves without a manual `recur register`.

- Registration is idempotent: apps already restored from the state file (by
  physical file identity) are skipped.
- Directories without a recurfile, and dotfile-prefixed staging directories, are
  ignored.
- A malformed app recurfile is logged and skipped; it does not abort startup.

## Acceptance Criteria

- A directory with a root recurfile and scripts round-trips through
  `pack` → `install`, preserving nested files and executable bits.
- A bundle with no root YAML, or with an ambiguous set of root YAML files, is
  rejected by both `pack` and `install`.
- A traversal entry (e.g. `../escape`) in a crafted archive is rejected on
  unpack without writing outside the destination.
- Installing while the daemon is stopped unpacks the app and, on next daemon
  start, the app is registered automatically.
- Installing over an existing app prompts; answering no leaves the existing app
  intact; `--force` replaces it.
- `--name`, recurfile-stem, and bundle-stem name resolution behave in that
  priority order.
