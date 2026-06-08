# Plugin: shell

> **Built into the daemon — not packaged as an external plugin.** Unlike
> every other action plugin in this directory, `shell` is registered
> directly inside `recurd` (`src/domain/plugin/validate.go` lists it in
> `BuiltinActionNames`) and dispatched by `actionDispatcher` →
> `shellExecutor` rather than spawned as a subprocess via the
> external-plugin protocol. The action option contract documented below
> matches the manifest model used by external plugins so authors get a
> consistent surface, but there is no `manifest.yaml`, no namespace, and
> no separate binary on disk. Treat this file as the spec for the
> built-in action.

Executes shell commands and inline scripts as a recurfile action. The
default action — when a recurfile lists an action without specifying a
type, `shell` is what runs.

## Action

### shell

Execute a shell command or script.

**Options:**

| Name | Type | Default | Notes |
|------|------|---------|-------|
| `command` | string | — (required, shorthand) | Inline command or path to a script file. Auto-detected: if the value is a path to an existing file, it is executed as a script; otherwise treated as an inline command. |
| `shell` | string | daemon `default_shell` | Shell used to execute `command`. Defaults to the daemon's `default_shell` config (`sh -c` on Unix, `powershell.exe -Command` on Windows). |
| `params` | list | `[]` | Additional parameters appended to the command in order. |
| `working_dir` | string | recurfile's directory | Working directory for the spawned process. |
| `env` | map | `{}` | Extra environment variables. Merged with trigger-context variables; explicit `env` values take precedence on conflict. Shadowed context keys produce a registration-time warning. |
| `timeout` | string (Go duration) | `"0"` | Maximum execution time before the process is killed. `"0"` means no timeout. Format: `30s`, `5m`, `1h30m`. |

**Behavior:**

- Template substitution (`{{.FilePath}}`, etc.) is applied to `command`
  and each entry in `params` before the process is spawned, using the
  trigger event's context as the data set.
- Standard exit-code semantics: zero is success; non-zero counts as a
  failure and increments the action's error counter. The
  `allowed_exit_codes` field on the recurfile action entry, when set,
  expands the success set beyond `{0}`.

**Shorthand form** (only the `command` option supplied):

```yaml
actions:
  - shell: "echo {{.FilePath}}"
```

is equivalent to:

```yaml
actions:
  - type: shell
    options:
      command: "echo {{.FilePath}}"
```

## Future considerations

- Shell-style variable expansion (Docker-Compose-flavored
  `${VAR:-default}`, `${VAR:?required}`) inside `command` — deferred;
  for now use the secret resolution system to inject values.
