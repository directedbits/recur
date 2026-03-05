# D-Bus Trigger Plugins Plan

## Approach: Thin Focused Plugins + Generic Escape Hatch

Instead of one massive D-Bus plugin with a device database, use small focused
plugins that each wrap a specific D-Bus service, plus a generic plugin for
advanced users.

### Focused Plugins

- **`devicemonitor`** — DeviceConnected, DeviceDisconnected (wraps UDisks2)
- **`networkmonitor`** — NetworkConnected, NetworkDisconnected, NetworkChanged (wraps NetworkManager)
- **`systemmonitor`** — SessionLocked, SessionUnlocked, SuspendPrepare, ResumeComplete (wraps logind)
- **`dbus`** — generic DBusSignal for advanced users (raw match rules)

Each is small (~200 lines), independently distributable, and hides D-Bus
internals from the user.

### User Experience

Focused plugin (typical usage):
```yaml
Backup:
  on:
    - type: DeviceConnected
      options:
        device_type: usb    # "usb", "block", "drive", or "all"
  do:
    - shell: "rsync -a /mnt/backup/ /media/{{.DeviceName}}/"
```

Generic escape hatch (power users):
```yaml
Custom:
  on:
    - type: DBusSignal
      options:
        bus: system
        interface: org.freedesktop.UDisks2.Drive
        member: PropertiesChanged
        body_match:
          key: "ConnectionBus"
          value: "usb"
  do:
    - shell: "echo device event"
```

## Namespace Convention

First-party plugins use the `core.*` prefix. Third-party plugins use a reversed
domain or other unique prefix. `core.*` is reserved — the daemon should reject
`install` of plugins with `core.*` namespaces from external sources.

First-party namespaces:
```
core.fileevents      → FileCreated, FileModified, FileDeleted, ...
core.devicemonitor   → DeviceConnected, DeviceDisconnected
core.networkmonitor  → NetworkConnected, NetworkDisconnected, NetworkChanged
core.systemmonitor   → SessionLocked, SessionUnlocked, SuspendPrepare, ...
core.dbus            → DBusSignal (generic)
core.cron            → CronSchedule, interval
core.calendar        → EventStarting, EventStarted, EventEnded
core.webhook         → WebhookReceived
```

Third parties would use their own reversed domain:
```
com.example.spotifymonitor → TrackChanged, PlaybackStarted, ...
io.github.user.imapmonitor → EmailReceived, ...
```

Conflict resolution in recurfiles via fully qualified names:
```yaml
# Unambiguous (typical)
on:
  - type: DeviceConnected

# Qualified (conflict resolution)
on:
  - type: core.devicemonitor.DeviceConnected
```

## devicemonitor Plugin (Build First)

- **Trigger types:** DeviceConnected, DeviceDisconnected
- **Options:** `device_type` (usb/block/drive/all)
- **Context vars:** DeviceName, DeviceType, DevicePath, MountPoint, TriggeredOn
- **Internally:** subscribes to UDisks2 D-Bus signals, one match rule

## Verification Targets for Generic dbus Plugin

Before release, verify the generic plugin works against these D-Bus services:

| Plugin | D-Bus Service | Signals | Complexity |
|---|---|---|---|
| networkmonitor | NetworkManager | StateChanged, device added/removed | Low |
| systemmonitor | logind | PrepareForSleep, Lock/Unlock, sessions | Low |
| bluetoothmonitor | BlueZ | Device connected/paired/removed | Medium |
| mediamonitor | MPRIS2 (Spotify, VLC) | PlaybackStatus, Metadata changed | Low |
| powermonitor | UPower | battery level, charging state | Low |

Recommended to verify against **networkmonitor** and **powermonitor** signals
in addition to devicemonitor. These use different D-Bus interfaces and confirm
the generic plugin handles varied event shapes.

## Implementation Order

```
1. Dispatch refactor     → done (Driver interface + external plugin driver)
2. Plugin protocol       → done (gRPC callback, env vars, stdin JSON)
3. devicemonitor plugin  → first D-Bus plugin, validates protocol + D-Bus basics
4. generic dbus plugin   → verify against networkmonitor + powermonitor signals
```
