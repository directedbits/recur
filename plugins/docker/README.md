---
title: "Docker"
weight: 6
description: "Container lifecycle triggers and management actions"
---

# Docker Plugin

Docker container lifecycle triggers and management actions.

## Triggers

### ContainerStarted

Fires when a container starts.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `host` | no | `unix:///var/run/docker.sock` | Docker Engine API endpoint (unix socket or `tcp://host:port`) |
| `filter_name` | no | -- | Filter by container name (substring match) |
| `filter_image` | no | -- | Filter by image name (substring match) |
| `filter_label` | no | -- | Filter by label (`key=value`) |

### ContainerStopped

Fires when a container stops or dies.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `host` | no | `unix:///var/run/docker.sock` | Docker Engine API endpoint |
| `filter_name` | no | -- | Filter by container name |
| `filter_image` | no | -- | Filter by image name |
| `filter_label` | no | -- | Filter by label (`key=value`) |

### HealthChanged

Fires when a container's health status changes.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `host` | no | `unix:///var/run/docker.sock` | Docker Engine API endpoint |
| `filter_name` | no | -- | Filter by container name |
| `filter_image` | no | -- | Filter by image name |

## Actions

### ContainerStart

Start a stopped container.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `host` | no | `unix:///var/run/docker.sock` | Docker Engine API endpoint |
| `container` | yes | -- | Container name or ID to start (**shorthand option** -- can use `ContainerStart: "myapp"`) |

### ContainerStop

Stop a running container.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `host` | no | `unix:///var/run/docker.sock` | Docker Engine API endpoint |
| `container` | yes | -- | Container name or ID to stop (**shorthand option** -- can use `ContainerStop: "myapp"`) |
| `timeout` | no | `10` | Seconds to wait before killing |

### ContainerRestart

Restart a container.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `host` | no | `unix:///var/run/docker.sock` | Docker Engine API endpoint |
| `container` | yes | -- | Container name or ID to restart (**shorthand option** -- can use `ContainerRestart: "myapp"`) |
| `timeout` | no | `10` | Seconds to wait before killing |

## Context Variables

### ContainerStarted / ContainerStopped

| Variable | Description |
|----------|-------------|
| `ContainerID` | Full container ID |
| `ContainerName` | Container name (without leading `/`) |
| `Image` | Image name |
| `Status` | Event status (`start`, `stop`, `die`, etc.) |
| `ExitCode` | Container exit code (ContainerStopped only, for `die` events) |

### HealthChanged

| Variable | Description |
|----------|-------------|
| `ContainerID` | Full container ID |
| `ContainerName` | Container name |
| `Image` | Image name |
| `HealthStatus` | New health status (`healthy`, `unhealthy`, `starting`) |

## Examples

### Auto-restart unhealthy containers

```yaml
RestartUnhealthy:
  on:
    - type: HealthChanged
      options:
        filter_name: "webapp"
  do:
    - ContainerRestart: "{{ .ContainerName }}"
```

### Log container lifecycle events

```yaml
LogLifecycle:
  on:
    - type: ContainerStopped
      options:
        filter_image: "myapp"
  do:
    - shell: "echo '{{ .ContainerName }} {{ .Status }} (exit={{ .ExitCode }})' >> /var/log/containers.log"
```

### Restart dependent services when a database container stops

```yaml
RestartOnDBDown:
  on:
    - type: ContainerStopped
      options:
        filter_name: "postgres"
  do:
    - ContainerRestart: "webapp"
    - ContainerRestart: "api-server"
```

### Shorthand action usage

```yaml
RestartApp:
  on:
    - type: cron
      options:
        expression: "@daily"
  do:
    - ContainerRestart: "myapp"
```

### Using a remote Docker host

```yaml
RemoteMonitor:
  on:
    - type: ContainerStarted
      options:
        host: "tcp://192.168.1.100:2375"
        filter_label: "env=production"
  do:
    - shell: "notify-send 'Container {{ .ContainerName }} started on remote host'"
```
