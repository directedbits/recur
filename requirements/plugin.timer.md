# Plugin: timer

External trigger plugin that provides time-driven triggers via cron schedules and
fixed intervals. Uses `robfig/cron/v3` for cron expression parsing and stdlib
`time.Ticker` for fixed intervals.

## Plugin Identity

| Field       | Value                                          |
|-------------|------------------------------------------------|
| Name        | `timer`                                        |
| Namespace   | `core.timer`                                   |
| Version     | `0.1.0`                                        |
| Binary      | `timer`                                        |

## Trigger Types

### Cron

Fires on a cron schedule. Supports standard 5-field cron expressions and presets.

**Options:**

| Option          | Type   | Default   | Description                                |
|-----------------|--------|-----------|--------------------------------------------|
| `expression`    | string | *required*| Cron expression or preset                  |
| `timezone`      | string | `"Local"` | IANA timezone name                         |
| `fire_on_start` | string | `"false"` | Fire one event immediately on startup      |

**Supported expressions:**

- Standard 5-field: `*/5 * * * *` (every 5 minutes)
- Presets: `@yearly`, `@monthly`, `@weekly`, `@daily`, `@hourly`
- Every shorthand: `@every 1h30m`

**Context Variables:**

| Variable           | Type   | Description                              |
|--------------------|--------|------------------------------------------|
| `TickCount`        | string | Cumulative fire count since activation   |
| `TimeSinceStarted` | string | Duration since plugin started (Go format)|

### Interval

Fires on a fixed time interval using Go duration strings.

**Options:**

| Option          | Type   | Default   | Description                                |
|-----------------|--------|-----------|--------------------------------------------|
| `every`         | string | *required*| Go duration string (e.g., `30s`, `5m`)    |
| `fire_on_start` | string | `"false"` | Fire one event immediately on startup      |

**Context Variables:**

| Variable           | Type   | Description                              |
|--------------------|--------|------------------------------------------|
| `TickCount`        | string | Cumulative fire count since activation   |
| `TimeSinceStarted` | string | Duration since plugin started (Go format)|

## fire_on_start Behavior

- `"false"` (default): first event fires on the first scheduled tick
- `"true"`: fire one immediate event on startup, then continue with the normal schedule
- Applies to both cron and interval
- No persisted state — the plugin does not track previous runs

## Protocol

This plugin follows the [trigger plugin protocol](plugin-protocol.md):

1. Reads trigger type and options from stdin JSON
2. Reads `RECUR_SOCKET` and `RECUR_TRIGGER_ID` from environment
3. Sets up cron scheduler or interval ticker
4. Optionally fires an immediate event if `fire_on_start` is `"true"`
5. Connects to daemon gRPC socket
6. Event loop: tick fires → build context → call `ReportTriggerEvent`
7. On SIGTERM: stop scheduler/ticker, close gRPC, exit 0

## Example Recurfile

```yaml
Hourly Backup:
  on:
    - type: cron
      options:
        expression: "0 * * * *"
        timezone: UTC
  do:
    - shell: /usr/local/bin/backup.sh

Health Check:
  on:
    - type: interval
      options:
        every: 30s
        fire_on_start: "true"
  do:
    - shell: >
        curl -sf http://localhost:8080/health
        || notify-send "Health check failed" "Tick {{.TickCount}}"
```
