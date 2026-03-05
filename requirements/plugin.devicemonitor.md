# Plugin: devicemonitor

External trigger plugin that watches for USB/block device connect/disconnect events
and reports them to the daemon using the `ReportTriggerEvent` gRPC callback. Uses
UDisks2 D-Bus signals on Linux and WMI polling on Windows. The plugin defines a
`DeviceSubscriber` interface that abstracts the platform-specific implementation.

## Plugin Identity

| Field       | Value                                          |
|-------------|------------------------------------------------|
| Name        | `devicemonitor`                                |
| Namespace   | `core.devicemonitor`                           |
| Version     | `0.1.0`                                        |
| Binary      | `devicemonitor`                                |
| Dependencies| `udisks2` (Linux only)                         |

## Platform Support

| Platform | Backend | Event Delivery |
|----------|---------|----------------|
| Linux    | UDisks2 D-Bus signals | Real-time (signal subscription) |
| Windows  | WMI (`Win32_LogicalDisk`, `Win32_DiskDrive`) | Polling (2-second interval) |
| macOS    | Not supported | — |

Windows support requires native Windows (not WSL2, which cannot access WMI).

## Trigger Types

### DeviceConnected

Fires when a block/USB device or drive is connected to the system.

**Options:**

| Option        | Type   | Default | Description                               |
|---------------|--------|---------|-------------------------------------------|
| `device_type` | string | `"all"` | Filter: `"usb"`, `"block"`, `"drive"`, or `"all"` |

**Context Variables:**

| Variable     | Type   | Description                              |
|--------------|--------|------------------------------------------|
| `DeviceName` | string | Kernel device name (e.g., `sdb1`)        |
| `DeviceType` | string | Classification: `usb`, `block`, or `drive` |
| `DevicePath` | string | UDisks2 D-Bus object path                |
| `MountPoint` | string | Mount path if mounted, empty otherwise   |

### DeviceDisconnected

Fires when a block/USB device or drive is disconnected from the system.

**Options:**

| Option        | Type   | Default | Description                               |
|---------------|--------|---------|-------------------------------------------|
| `device_type` | string | `"all"` | Filter: `"usb"`, `"block"`, `"drive"`, or `"all"` |

**Context Variables:**

| Variable     | Type   | Description                              |
|--------------|--------|------------------------------------------|
| `DeviceName` | string | Kernel device name                       |
| `DeviceType` | string | Classification: `usb`, `block`, or `drive` |
| `DevicePath` | string | UDisks2 D-Bus object path                |

Note: `MountPoint` is not available on disconnect since the device is already gone.

## DeviceSubscriber Interface

The plugin uses a `DeviceSubscriber` interface to abstract platform-specific device
monitoring. Each platform provides its own implementation via build tags:

```go
type DeviceSubscriber interface {
    Subscribe(deviceType string) (<-chan DeviceEvent, error)
    Close()
}
```

- **Linux** (`subscriber_linux.go`): Connects to system D-Bus, subscribes to UDisks2 signals.
- **Windows** (`subscriber_windows.go`): Polls WMI every 2 seconds, diffs drive snapshots.

## D-Bus Details (Linux)

### UDisks2 Object Manager

The plugin connects to the system D-Bus and subscribes to signals from the
`org.freedesktop.DBus.ObjectManager` interface on the UDisks2 object at
`/org/freedesktop/UDisks2`.

### Signals

**InterfacesAdded** — emitted when a new device appears:
- Signal: `org.freedesktop.DBus.ObjectManager.InterfacesAdded`
- Body: `(object_path, dict<string, dict<string, variant>>)`
- The plugin checks for `org.freedesktop.UDisks2.Block` or `org.freedesktop.UDisks2.Drive`
  interfaces in the properties dict.

**InterfacesRemoved** — emitted when a device disappears:
- Signal: `org.freedesktop.DBus.ObjectManager.InterfacesRemoved`
- Body: `(object_path, []string)`
- The plugin checks for `org.freedesktop.UDisks2.Block` or `org.freedesktop.UDisks2.Drive`
  in the removed interfaces list.

### Device Classification

- **`block`**: Object path contains `/block_devices/` and has `org.freedesktop.UDisks2.Block`
- **`drive`**: Object path contains `/drives/` and has `org.freedesktop.UDisks2.Drive`
- **`usb`**: A block device or drive whose `ConnectionBus` property is `"usb"`

### Device Name Extraction

The kernel device name is extracted from the D-Bus object path:
- `/org/freedesktop/UDisks2/block_devices/sdb1` → `sdb1`
- `/org/freedesktop/UDisks2/drives/WD_Elements_1234` → `WD_Elements_1234`

### Mount Point

For `InterfacesAdded` events on block devices, the plugin reads the
`org.freedesktop.UDisks2.Filesystem.MountPoints` property if the Filesystem
interface is present. Mount points are byte arrays (null-terminated); the plugin
decodes the first one.

## WMI Details (Windows)

### Polling Mechanism

The Windows subscriber polls WMI every 2 seconds using `Win32_LogicalDisk` queries,
comparing snapshots to detect added/removed drives. It also queries `Win32_DiskDrive`
for physical drive metadata (interface type, model).

### Device Classification (Windows)

- **`usb`**: `DriveType=2` (removable) or physical drive with `InterfaceType="USB"`
- **`block`**: `DriveType=3` (fixed local disk)
- **`drive`**: Other drive types

### Device Name (Windows)

The `DeviceName` context variable reports the volume name if available, otherwise the
drive description, falling back to the drive letter (e.g., `D:`).

### Device Path (Windows)

`DevicePath` contains the drive letter (e.g., `D:`). `MountPoint` for connected
devices also contains the drive letter.

## Protocol

This plugin follows the [trigger plugin protocol](plugin-protocol.md):

**Linux:**
1. Reads trigger type and options from stdin JSON
2. Reads `RECUR_SOCKET` and `RECUR_TRIGGER_ID` from environment
3. Connects to system D-Bus
4. Subscribes to UDisks2 InterfacesAdded/InterfacesRemoved signals
5. Connects to daemon gRPC socket
6. Event loop: receive D-Bus signal → parse → filter by `device_type` → call `ReportTriggerEvent`
7. On SIGTERM: disconnect D-Bus, close gRPC, exit 0

**Windows:**
1. Reads trigger type and options from stdin JSON
2. Reads `RECUR_SOCKET` and `RECUR_TRIGGER_ID` from environment
3. Takes initial WMI snapshot of logical disks
4. Connects to daemon gRPC socket
5. Poll loop (2s): query WMI → diff snapshots → filter by `device_type` → call `ReportTriggerEvent`
6. On SIGTERM: stop polling, close gRPC, exit 0

## Example Recurfile

```yaml
USB Backup:
  on:
    - type: DeviceConnected
      options:
        device_type: usb
  do:
    - shell: >
        echo "USB device {{.DeviceName}} connected at {{.MountPoint}}"
        >> ~/usb-events.log

Device Logger:
  on:
    - type: DeviceDisconnected
      options:
        device_type: all
  do:
    - shell: >
        notify-send "Device removed" "{{.DeviceName}} ({{.DeviceType}})"
```
