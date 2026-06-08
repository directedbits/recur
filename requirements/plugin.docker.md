# Plugin: docker

External trigger+action plugin for Docker container lifecycle monitoring and management.
Connects to the Docker Engine API (via Unix socket or TCP) to stream container events
(triggers) and perform container operations (actions). Uses `net/http` with no external
Docker SDK dependency — communicates directly with the Engine API.

## Plugin Identity

| Field       | Value                                          |
|-------------|------------------------------------------------|
| Name        | `docker`                                       |
| Namespace   | `core.docker`                                  |
| Version     | `0.1.0`                                        |
| Binary      | `docker`                                       |

## Trigger Types

### ContainerStarted

Fires when a Docker container starts.

**Options:**

| Option          | Type   | Default                          | Description                                  |
|-----------------|--------|----------------------------------|----------------------------------------------|
| `host`          | string | `"unix:///var/run/docker.sock"`  | Docker Engine API endpoint (unix socket or `tcp://host:port`) |
| `filter_name`   | string | *(optional)*                     | Filter by container name (substring match)   |
| `filter_image`  | string | *(optional)*                     | Filter by image name (substring match)       |
| `filter_label`  | string | *(optional)*                     | Filter by label (`key=value`)                |

**Context Variables:**

| Variable          | Type   | Description                              |
|-------------------|--------|------------------------------------------|
| `ContainerID`     | string | Full container ID                        |
| `ContainerName`   | string | Container name (without leading `/`)     |
| `Image`           | string | Image name                               |
| `Status`          | string | Event status (`start`)                   |

### ContainerStopped

Fires when a Docker container stops or dies.

**Options:**

| Option          | Type   | Default                          | Description                                  |
|-----------------|--------|----------------------------------|----------------------------------------------|
| `host`          | string | `"unix:///var/run/docker.sock"`  | Docker Engine API endpoint                   |
| `filter_name`   | string | *(optional)*                     | Filter by container name (substring match)   |
| `filter_image`  | string | *(optional)*                     | Filter by image name (substring match)       |
| `filter_label`  | string | *(optional)*                     | Filter by label (`key=value`)                |

**Context Variables:**

| Variable          | Type   | Description                              |
|-------------------|--------|------------------------------------------|
| `ContainerID`     | string | Full container ID                        |
| `ContainerName`   | string | Container name                           |
| `Image`           | string | Image name                               |
| `Status`          | string | Event status (`stop`, `die`, etc.)       |
| `ExitCode`        | string | Container exit code (for `die` events)   |

### HealthChanged

Fires when a container's health status changes. Requires the container to have a
`HEALTHCHECK` defined.

**Options:**

| Option          | Type   | Default                          | Description                                  |
|-----------------|--------|----------------------------------|----------------------------------------------|
| `host`          | string | `"unix:///var/run/docker.sock"`  | Docker Engine API endpoint                   |
| `filter_name`   | string | *(optional)*                     | Filter by container name (substring match)   |
| `filter_image`  | string | *(optional)*                     | Filter by image name (substring match)       |

**Context Variables:**

| Variable          | Type   | Description                              |
|-------------------|--------|------------------------------------------|
| `ContainerID`     | string | Full container ID                        |
| `ContainerName`   | string | Container name                           |
| `Image`           | string | Image name                               |
| `HealthStatus`    | string | New health status (`healthy`, `unhealthy`, `starting`) |

## Action Types

### ContainerStart

Start a stopped container.

**Options:**

| Option      | Type   | Default                          | Description                                  |
|-------------|--------|----------------------------------|----------------------------------------------|
| `host`      | string | `"unix:///var/run/docker.sock"`  | Docker Engine API endpoint                   |
| `container` | string | *required*                       | Container name or ID to start (shorthand option) |

Shorthand form: `ContainerStart: "myapp"`

### ContainerStop

Stop a running container.

**Options:**

| Option      | Type   | Default                          | Description                                  |
|-------------|--------|----------------------------------|----------------------------------------------|
| `host`      | string | `"unix:///var/run/docker.sock"`  | Docker Engine API endpoint                   |
| `container` | string | *required*                       | Container name or ID to stop (shorthand option) |
| `timeout`   | string | `"10"`                           | Seconds to wait before killing               |

Shorthand form: `ContainerStop: "myapp"`

### ContainerRestart

Restart a container.

**Options:**

| Option      | Type   | Default                          | Description                                  |
|-------------|--------|----------------------------------|----------------------------------------------|
| `host`      | string | `"unix:///var/run/docker.sock"`  | Docker Engine API endpoint                   |
| `container` | string | *required*                       | Container name or ID to restart (shorthand option) |
| `timeout`   | string | `"10"`                           | Seconds to wait before killing               |

Shorthand form: `ContainerRestart: "myapp"`

## Docker Engine API

The plugin communicates directly with the Docker Engine API over HTTP, without using
the Docker SDK. This keeps the binary small and avoids the heavy dependency tree.

### Connection

The `host` option determines the transport:

- `unix:///var/run/docker.sock` (default) — connects via Unix socket using a custom
  `http.Transport` with `DialContext` overridden to dial the socket path.
- `tcp://host:port` — connects via standard HTTP to the specified address.

### Event Streaming (Triggers)

Triggers use the `GET /events` endpoint with a `filters` query parameter to stream
container lifecycle events as newline-delimited JSON. The Docker API keeps the
connection open and pushes events as they occur.

Events are filtered server-side to `type=container`. Client-side filtering applies
`filter_name`, `filter_image`, and `filter_label` to the parsed event attributes.

### Container Operations (Actions)

Actions use standard Docker API endpoints:

| Action            | Endpoint                            | Method | Success Codes |
|-------------------|-------------------------------------|--------|---------------|
| ContainerStart    | `/containers/{id}/start`            | POST   | 204, 304 (already started) |
| ContainerStop     | `/containers/{id}/stop?t={timeout}` | POST   | 204, 304 (already stopped) |
| ContainerRestart  | `/containers/{id}/restart?t={timeout}` | POST | 204           |

### Event Parsing

Raw Docker events are JSON objects with `Type`, `Action`, and `Actor` fields. The plugin
extracts container metadata from `Actor.Attributes` (name, image, exitCode). Container
names are stripped of the leading `/` prefix.

Health status events have the action format `health_status: <status>`. The plugin splits
on `: ` to extract the health status string and normalizes the `Status` context variable
to `health_status`.

## Protocol

This plugin follows the [trigger plugin protocol](plugin-protocol.md):

**Trigger mode:**
1. Reads trigger type and options from stdin JSON
2. Reads `RECUR_SOCKET` and `RECUR_TRIGGER_ID` from environment
3. Creates Docker client from `host` option
4. Opens streaming connection to Docker Events API
5. Connects to daemon gRPC socket
6. Event loop: receive Docker event → parse → filter → call `ReportTriggerEvent`
7. On SIGTERM: cancel context (closes event stream), close gRPC, exit 0

**Action mode:**
1. Reads action type and options from stdin JSON
2. Creates Docker client from `host` option
3. Executes the appropriate container operation
4. Writes JSON result to stdout (`success`, `output`, `error`)
5. Exits

## Example Recurfile

```yaml
RestartUnhealthy:
  on:
    - type: HealthChanged
      options:
        filter_name: "webapp"
  do:
    - ContainerRestart: "{{.ContainerName}}"

LogLifecycle:
  on:
    - type: ContainerStopped
      options:
        filter_image: "myapp"
  do:
    - shell: >
        echo "{{.ContainerName}} {{.Status}} (exit={{.ExitCode}})" >> /var/log/containers.log

RestartOnDBDown:
  on:
    - type: ContainerStopped
      options:
        filter_name: "postgres"
  do:
    - ContainerRestart: "webapp"
    - ContainerRestart: "api-server"

RemoteMonitor:
  on:
    - type: ContainerStarted
      options:
        host: "tcp://192.168.1.100:2375"
        filter_label: "env=production"
  do:
    - shell: >
        notify-send "Container {{.ContainerName}} started on remote host"
```
