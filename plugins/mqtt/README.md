---
title: "MQTT"
weight: 3
description: "MQTT subscribe and publish"
---

# MQTT Plugin

MQTT messaging — subscribe to topics (trigger) and publish messages (action).

## Triggers

### MessageReceived

Fires when an MQTT message arrives on a subscribed topic.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `broker` | yes | — | Broker URL (e.g., `tcp://localhost:1883`, `ssl://broker:8883`) |
| `topic` | yes | — | Topic filter (supports `+` and `#` wildcards) |
| `qos` | no | `0` | QoS level (0, 1, or 2) |
| `username` | no | — | Username for authentication |
| `password` | no | — | Password or token |
| `client_id` | no | *(auto)* | Client ID (auto-generated as `recur-XXXXXXXX` if omitted) |
| `clean_session` | no | `true` | Start with a clean session |
| `keepalive` | no | `30` | Keepalive interval in seconds |

Broker, username, password, and client_id can also be set via plugin config (see below), with per-trigger options taking priority.

## Actions

### publish

Publishes a message to an MQTT topic.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `broker` | yes | — | Broker URL |
| `topic` | yes | — | Topic to publish to (**shorthand option** — can use `publish: "my/topic"`) |
| `payload` | no | `""` | Message payload |
| `qos` | no | `0` | QoS level (0, 1, or 2) |
| `retain` | no | `false` | Retain the message on the broker |
| `username` | no | — | Username for authentication |
| `password` | no | — | Password or token |
| `client_id` | no | *(auto)* | Client ID |

## Context Variables (Trigger)

| Variable | Description |
|----------|-------------|
| `Topic` | Topic the message was received on |
| `Payload` | Message payload as string |
| `QoS` | QoS level of the received message |
| `Retained` | Whether the message was retained (`true`/`false`) |
| `MessageID` | Message ID (for QoS 1/2) |

## Plugin Configuration

Shared credentials can be set once in daemon config instead of repeating per trigger/action:

```sh
recur config set plugins.core.mqtt.broker "tcp://localhost:1883"
recur config set plugins.core.mqtt.username "admin"
recur config set plugins.core.mqtt.password "secret"
recur config set plugins.core.mqtt.client_id "recur-main"
```

Per-trigger/action options override plugin config when both are set.

## Examples

### Home automation relay

```yaml
HomeAuto:
  on:
    - type: MessageReceived
      options:
        broker: "tcp://192.168.1.10:1883"
        topic: "home/sensors/temperature"
  do:
    - shell: "echo 'Temperature: {{ .Payload }}' >> /var/log/sensors.log"
```

### Forward messages between topics

```yaml
MQTTBridge:
  on:
    - type: MessageReceived
      options:
        broker: "tcp://localhost:1883"
        topic: "source/#"
  do:
    - name: publish
      options:
        broker: "tcp://localhost:1883"
        topic: "archive/{{ .Topic }}"
        payload: "{{ .Payload }}"
```

### Shorthand publish

```yaml
Notify:
  on:
    - type: cron
      options:
        expression: "@hourly"
  do:
    - publish: "status/heartbeat"
```

### Authenticated broker with QoS

```yaml
SecureMQTT:
  on:
    - type: MessageReceived
      options:
        broker: "ssl://broker.example.com:8883"
        topic: "events/critical"
        qos: "1"
        username: "monitor"
        password: "token-123"
  do:
    - shell: "alert.sh '{{ .Payload }}'"
```
