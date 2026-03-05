# Roadmap

## Completed

- [x] Define requirements (specs split into focused files under `requirements/`)
- [x] Design initial File Events trigger plugin (`plugin.fileevents.requirements.md`)
- [x] Design initial shell action plugin (`plugin.shell.requirements.md`)
- [x] Design daemon (lifecycle, internals, API, persistence, config)
- [x] Design CLI (structure, commands, flags, output)
- [x] Domain layer — core types and repository interfaces
- [x] Config loading/persistence with atomic writes
- [x] Daemon core — PID management, signal handling, foreground mode
- [x] Plugin manifest parsing and discovery
- [x] gRPC proto definitions, server, and client
- [x] All 25 gRPC RPCs implemented
- [x] Recurfile parser with structural validation
- [x] Full CLI command tree wired to gRPC
- [x] In-memory registry with deterministic IDs
- [x] State persistence with crash recovery (atomic writes)
- [x] Action execution — subprocess isolation, template resolution, timeouts
- [x] File Events trigger engine — fsbroker integration, wired into daemon lifecycle
- [x] E2e test suite covering registration, triggers, actions, config, daemon lifecycle
- [x] Trigger plugin protocol — stdin JSON + env vars input, gRPC callback output (`requirements/plugin-protocol.md`)
- [x] External plugin driver + PluginEventRouter — bridges subprocess lifecycle into Driver interface
- [x] ReportTriggerEvent gRPC RPC — plugins report events back to daemon
- [x] Device monitor plugin — UDisks2 D-Bus on Linux, WMI polling on Windows (`plugin.devicemonitor.md`)
- [x] Timer plugin — cron schedules and fixed intervals (`plugin.timer.md`)
- [x] Calendar plugin — iCal/ICS polling with event filters (`plugin.calendar.md`)
- [x] Webhook plugin — HTTP/HTTPS listener with HMAC, TLS, rate limiting (`plugin.webhook.md`)
- [x] MQTT plugin — topic subscription (trigger) and message publishing (action) (`plugin.mqtt.md`)
- [x] Docker plugin — container lifecycle triggers and management actions (`plugin.docker.md`)
- [x] Daemon structured logging via `log/slog` with configurable log level
- [x] `RECUR_LOG_LEVEL` env var propagated to plugins
- [x] macOS support — POSIX-compatible `_unix.go` build tags, Darwin CI/release targets
- [x] Windows support — WMI-based device monitor, Windows release targets
- [x] Docker image — multi-stage build with configurable plugin set
- [x] FileEvents extracted to external plugin (consistency with other plugins)
- [x] Filter/glob support for file events trigger
- [x] Entity type filtering (file/directory/all) in file events trigger
- [x] Concurrency modes (queue, parallel, drop, abort) with debounce
- [x] Error threshold enforcement for triggers and actions
- [x] Group option inheritance and alias resolution
- [x] Config overlay system (`pkg/config/` standalone module with Store/MapStore)
- [x] Register-as-reload (atomic deregister + re-register)
- [x] Launch args persistence, restart command, status display
- [x] Entity index for unified identifier resolution
- [x] Action `name` → `type` rename, optional `name` label field
- [x] YAML anchor support in recurfile parsing

## Architecture

### Trigger Plugin Architecture

All trigger plugins are **out-of-process** — separate binaries communicating
with the daemon via the plugin protocol. This keeps the daemon a pure
orchestrator with no trigger-specific code compiled in.

**Exception:** FileEvents is currently an in-process driver (compiled into the
daemon). It may be extracted to an external plugin later for consistency.

The trigger engine uses the Driver interface internally. The external plugin
driver bridges the plugin protocol into that interface, so the engine doesn't
know or care whether a trigger is in-process or external.

**Protocol summary:** Daemon spawns plugin binary with `RECUR_*` env vars and
JSON on stdin. Plugin connects back to daemon's Unix socket and calls
`ReportTriggerEvent` gRPC when events fire. Daemon sends SIGTERM to stop,
SIGKILL after `shutdown_timeout`. See `requirements/plugin-protocol.md`.

### Trigger Plugin Progression

Each step introduced one new concept to validate and shape the plugin protocol:

```
1. FileEvents (done)        → in-process driver (legacy, may extract later)
2. Plugin protocol (done)   → gRPC callback contract, generic external driver
3. Device monitor (done)    → first external plugin, event-driven (D-Bus) + poll-driven (WMI)
4. Cron/Timer (done)        → external plugin, time-driven (new axis)
5. Calendar (done)          → external plugin, poll-driven with filtering (new axis)
6. Webhook (done)           → external plugin, HTTP/HTTPS listener (new axis)
7. MQTT (done)              → external plugin, trigger + action in one binary
8. Docker (done)            → external plugin, Docker Engine API streaming + actions
```

#### 1. FileEvents (complete)
- In-process driver using fsbroker (wraps fsnotify)
- Supports FileCreated, FileModified, FileDeleted, FileMoved, FileAttributeChanged
- Event-driven: subscribe to OS notifications, react to events
- Validates: basic trigger-to-action wiring, context variables, suspend/resume

#### 2. Plugin Protocol (complete)
- Protocol spec: `requirements/plugin-protocol.md`
- Input: JSON on stdin + `RECUR_*` env vars (equivalent representations)
- Output: plugin connects to daemon gRPC socket, calls `ReportTriggerEvent`
- Lifecycle: SIGTERM → `shutdown_timeout` → SIGKILL; unexpected exit → trigger errored
- External driver (`src/app/trigger/external.go`) + PluginEventRouter (`src/app/trigger/router.go`)
- Daemon wires ExternalPluginFactory per discovered plugin with trigger types
- Validates: protocol design before building any external plugins

#### 3. Device Monitor (complete)
- First external trigger plugin — separate binary
- Linux: UDisks2 D-Bus signals for instant device connect/disconnect events
- Windows: WMI polling for drive changes (2-second interval)
- `DeviceSubscriber` interface abstracts platform differences
- Validates: plugin protocol handles real-world trigger types, cross-platform plugins

#### 4. Cron/Timer (complete)
- External plugin with built-in scheduler (`robfig/cron/v3`)
- Schedule expressions (`*/5 * * * *`) and simple intervals (`30s`)
- No external event source — time-driven
- Validates: plugins aren't limited to "watchers"; time-based triggers work
- Cross-platform, no systemd dependency

#### 5. Calendar (complete)
- External plugin polling iCal/ICS sources via `apognu/gocal`
- Triggers on: upcoming event, event started, event ended
- Event filtering: 7 filter options (include/exclude by title, location, description, category)
- `EventCategories` context variable for category-aware actions
- Validates: poll loop pattern with complex filtering

#### 6. Webhook (complete)
- External plugin with its own HTTP/HTTPS server
- HMAC-SHA256 signature verification for secure webhooks
- TLS support via `tls_cert`/`tls_key` options
- Token-bucket rate limiting with configurable RPS and `Retry-After` headers
- Backpressure: channel-full returns 429 instead of silently dropping
- Validates: plugins can manage their own listeners/servers

#### 7. MQTT (complete)
- Trigger + action in a single binary
- Trigger: persistent subscription with auto-reconnect
- Action: connect-publish-disconnect per invocation
- Validates: combined trigger+action plugin pattern

#### 8. Docker (complete)
- Trigger: streams Docker Engine API events for container lifecycle
- Action: container management (start, stop, restart) via Docker API
- Supports Unix socket and TCP connections to Docker Engine
- Container filtering by name, image, and label
- Validates: streaming API event source + action plugin in one binary

## Upcoming

- [ ] State persistence of runtime metrics (execution history beyond last-fired)
- [ ] Plugin `secret` field on options (mark options as sensitive, redact in logs/inspect)
- [ ] Recurfile validation against plugin manifests (injected rule interfaces)

## Future

- [ ] Plugin registry and distribution
- [ ] Plugin security — signature verification
- [ ] Additional connection types beyond Unix socket
- [ ] macOS devicemonitor support (DiskArbitration)
- [ ] Documentation and project cleanup
