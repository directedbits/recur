---
title: "Docker"
weight: 4
description: "Run recur in a container"
---

Run recur in a container with multi-stage build, non-root user, and signal forwarding via tini.

## Quick Start

Build and run with default plugins (timer, webhook):

```sh
docker build -t recur .
docker run --rm -it recur
```

Or use Compose from this directory:

```sh
docker compose up --build
```

## Plugin Selection

Choose which plugins to include at build time with the `RECUR_PLUGINS` build arg:

```sh
# Timer and webhook only (default)
docker build -t recur .

# All plugins
docker build --build-arg RECUR_PLUGINS="timer webhook mqtt calendar" -t recur .

# Webhook only
docker build --build-arg RECUR_PLUGINS="webhook" -t recur .
```

Available plugins: `timer`, `webhook`, `mqtt`, `calendar`, `devicemonitor`.

## Version Injection

Pass a version string at build time:

```sh
docker build --build-arg VERSION="1.0.0" -t recur .
```

This sets the version reported by `recur version`.

## Volumes

| Path | Purpose |
|------|---------|
| `/home/recur/.recur/state` | Persistent state (JSON flat files). Mount a volume here to survive restarts. |
| `/home/recur/.recur/config.yaml` | Config file. Bind-mount your own config (read-only recommended). |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `RECUR_CONFIG_PATH` | Path to config file inside the container. |
| `RECUR_SOCKET` | Override daemon socket address. Set to a TCP address (e.g., `0.0.0.0:8080`) to expose the gRPC API over the network instead of a Unix socket. Useful for proxy setups. |

## Socket and Proxy Use

By default, recurd listens on a Unix socket inside the container. To expose the gRPC API over TCP (e.g., behind an Envoy or nginx proxy), set `RECUR_SOCKET`:

```sh
docker run --rm -it -e RECUR_SOCKET="0.0.0.0:8080" -p 8080:8080 recur
```

Or in `docker-compose.yml`:

```yaml
environment:
  RECUR_SOCKET: "0.0.0.0:8080"
```

## Signal Handling

The container uses [tini](https://github.com/krallin/tini) as PID 1 for proper signal forwarding. `docker stop` sends SIGTERM, which tini forwards to the daemon for a clean shutdown.

## Health Check

The image includes a health check that runs `recur status` every 30 seconds (with a 10-second start period). Check container health with:

```sh
docker inspect --format='{{.State.Health.Status}}' <container>
```

## Compose

The included `docker-compose.yml` shows:

- Building with selected plugins and version
- Exposing port 8080 for the webhook plugin
- Mounting a config file (read-only)
- Persisting state with a named volume
- Optional MQTT broker (commented out)

## devicemonitor Caveats

The `devicemonitor` plugin monitors D-Bus for device events. This requires:

- Running the container with `--privileged` or mounting the host D-Bus socket
- The host must be running a D-Bus daemon

This is generally not practical in a container. Consider running devicemonitor on the host and connecting to a containerized recur instance via TCP socket instead.
