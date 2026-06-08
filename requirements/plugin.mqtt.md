# Plugin: mqtt

External trigger+action plugin for MQTT messaging. Subscribes to topics and fires
events on incoming messages (trigger), and publishes messages to topics (action).
Uses `eclipse/paho.mqtt.golang`. Single binary handles both trigger and action modes.

## Plugin Identity

| Field       | Value                                          |
|-------------|------------------------------------------------|
| Name        | `mqtt`                                         |
| Namespace   | `core.mqtt`                                    |
| Version     | `0.1.0`                                        |
| Binary      | `mqtt`                                         |

## Trigger Types

### MessageReceived

Fires when an MQTT message arrives on a subscribed topic.

**Options:**

| Option          | Type   | Default   | Description                                  |
|-----------------|--------|-----------|----------------------------------------------|
| `broker`        | string | *required*| Broker URL (e.g. `tcp://localhost:1883`)      |
| `topic`         | string | *required*| Topic filter (supports `+` and `#` wildcards) |
| `qos`           | string | `"0"`     | QoS level (0, 1, or 2)                       |
| `username`      | string | *(optional)* | Username for authentication               |
| `password`      | string | *(optional)* | Password or token for authentication      |
| `client_id`     | string | *(auto)*  | Client ID (auto-generated if omitted)        |
| `clean_session` | string | `"true"`  | Start with a clean session                   |
| `keepalive`     | string | `"30"`    | Keepalive interval in seconds                |

**Context Variables:**

| Variable    | Type   | Description                                    |
|-------------|--------|------------------------------------------------|
| `Topic`     | string | Topic the message was received on              |
| `Payload`   | string | Message payload as string                      |
| `QoS`       | string | QoS level of the received message              |
| `Retained`  | string | Whether the message was retained ("true"/"false") |
| `MessageID` | string | Message ID (for QoS 1/2)                       |

## Action Types

### Publish

Publishes a message to an MQTT topic.

**Options:**

| Option      | Type   | Default   | Description                                  |
|-------------|--------|-----------|----------------------------------------------|
| `broker`    | string | *required*| Broker URL (e.g. `tcp://localhost:1883`)      |
| `topic`     | string | *required*| Topic to publish to (shorthand option)        |
| `payload`   | string | `""`      | Message payload                              |
| `qos`       | string | `"0"`     | QoS level (0, 1, or 2)                       |
| `retain`    | string | `"false"` | Retain the message on the broker             |
| `username`  | string | *(optional)* | Username for authentication               |
| `password`  | string | *(optional)* | Password or token for authentication      |
| `client_id` | string | *(auto)*  | Client ID (auto-generated if omitted)        |

## Behavior

### Trigger Mode

- **Connection:** Persistent MQTT connection with auto-reconnect enabled.
- **Subscription:** Subscribes to the configured topic at the specified QoS.
- **Events:** Reports each received message via gRPC `ReportTriggerEvent`.
- **Lifecycle:** Long-lived process — runs until SIGTERM/SIGINT.
- **Client ID:** If not specified, generates `recur-{8-random-hex}`.

### Action Mode

- **Connection:** Connect-publish-disconnect per invocation (one-shot).
- **Test mode:** Returns success without connecting to the broker.
- **Output:** JSON on stdout with `success`, `output`, and `error` fields.

### Broker URL

If no scheme is provided, `tcp://` is prepended automatically.
Supported schemes: `tcp://`, `ssl://`, `tls://` (ssl and tls are equivalent).

### Authentication

- **Username + Password:** Standard MQTT auth.
- **Token:** Set `password` without `username` — some brokers accept this for token auth.
- **None:** Omit both for anonymous access.
