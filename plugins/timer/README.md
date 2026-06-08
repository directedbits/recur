---
title: "Timer"
weight: 1
description: "Cron schedule and interval triggers"
---

# Timer Plugin

Time-driven triggers via cron schedules and fixed intervals.

## Triggers

### Cron

Fires on a cron schedule.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `expression` | yes | — | Cron expression or preset (`@hourly`, `@daily`, `@weekly`, `@monthly`, `@yearly`, `@every 1h`) |
| `timezone` | no | `Local` | IANA timezone (e.g., `UTC`, `America/New_York`) |
| `fire_on_start` | no | `false` | Fire one event immediately on activation |

### Interval

Fires on a fixed time interval.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `every` | yes | — | Go duration string (e.g., `30s`, `5m`, `1h30m`) |
| `fire_on_start` | no | `false` | Fire one event immediately on activation |

## Context Variables

Both triggers provide the same context variables to actions:

| Variable | Description |
|----------|-------------|
| `TickCount` | Cumulative fire count since activation |
| `TimeSinceStarted` | Duration since plugin started |

## Examples

### Nightly build

```yaml
NightlyBuild:
  on:
    - type: cron
      options:
        expression: "0 2 * * *"
        timezone: "UTC"
  do:
    - shell: "cd /opt/project && make build"
```

### Health check every 30 seconds

```yaml
HealthCheck:
  on:
    - type: interval
      options:
        every: "30s"
  do:
    - shell: "curl -sf http://localhost:8080/health || notify-send 'Service down'"
```

### Log rotation on the first of each month

```yaml
LogRotation:
  on:
    - type: cron
      options:
        expression: "0 0 1 * *"
  do:
    - shell: "logrotate /etc/logrotate.d/myapp"
```

### Fire immediately then repeat

```yaml
StartupSync:
  on:
    - type: interval
      options:
        every: "15m"
        fire_on_start: "true"
  do:
    - shell: "rsync -a /data/ /backup/"
```

### Template variables

```yaml
TickLogger:
  on:
    - type: interval
      options:
        every: "1m"
  do:
    - shell: "echo 'Tick #{{ .TickCount }} at {{ .TimeSinceStarted }}' >> /var/log/ticks.log"
```
