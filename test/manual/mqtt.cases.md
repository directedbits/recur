# MQTT Plugin - Manual Test Cases

Minimal set of manual tests to cover the plugin surface.

## Setup

```sh
task build:mqtt
mkdir -p ~/.config/recur/plugins/mqtt
cp bin/plugins/mqtt/* ~/.config/recur/plugins/mqtt/

# Start a local Mosquitto broker (or any MQTT broker on tcp://localhost:1883)
docker run -d --name mosquitto -p 1883:1883 eclipse-mosquitto:2 \
  mosquitto -c /mosquitto-no-auth.conf
```

## Test 1: Basic MessageReceived + context variables

Covers: trigger fires on message, all context variables (Topic, Payload, QoS, Retained, MessageID), default QoS 0.

```yaml
# ~/test-mqtt/recur.yaml
MQTTBasic:
  on:
    - type: MessageReceived
      options:
        broker: "tcp://localhost:1883"
        topic: "test/hello"
      do:
        - shell: "echo 'MSG: topic={{.Topic}} payload={{.Payload}} qos={{.QoS}} retained={{.Retained}} id={{.MessageID}}'"
```

```sh
cd ~/test-mqtt && recur start --foreground &
recur register

mosquitto_pub -h localhost -t test/hello -m "world"       # expect MSG with payload=world
mosquitto_pub -h localhost -t test/hello -m '{"key":"v"}'  # expect MSG with JSON payload
mosquitto_pub -h localhost -t test/other -m "nope"         # expect NO event (wrong topic)
```

## Test 2: Wildcard topics

Covers: `+` single-level wildcard, `#` multi-level wildcard, Topic variable reflects actual topic.

```yaml
# ~/test-mqtt2/recur.yaml
Wildcards:
  on:
    - type: MessageReceived
      options:
        broker: "tcp://localhost:1883"
        topic: "home/+/temperature"
      do:
        - shell: "echo 'SINGLE: {{.Topic}} = {{.Payload}}'"
    - type: MessageReceived
      options:
        broker: "tcp://localhost:1883"
        topic: "logs/#"
      do:
        - shell: "echo 'MULTI: {{.Topic}} = {{.Payload}}'"
```

```sh
cd ~/test-mqtt2 && recur start --foreground &
recur register

mosquitto_pub -h localhost -t home/kitchen/temperature -m "22"  # expect SINGLE
mosquitto_pub -h localhost -t home/bedroom/temperature -m "19"  # expect SINGLE
mosquitto_pub -h localhost -t home/kitchen/humidity -m "60"     # expect NO event
mosquitto_pub -h localhost -t logs/app/error -m "fail"          # expect MULTI
mosquitto_pub -h localhost -t logs -m "root"                    # expect MULTI (# matches zero levels)
```

## Test 3: QoS levels + publish action

Covers: QoS 1 trigger, publish action with explicit options, payload template, retain flag.

```yaml
# ~/test-mqtt3/recur.yaml
Bridge:
  on:
    - type: MessageReceived
      options:
        broker: "tcp://localhost:1883"
        topic: "source/events"
        qos: "1"
      do:
        - type: publish
          options:
            broker: "tcp://localhost:1883"
            topic: "archive/{{.Topic}}"
            payload: "archived: {{.Payload}}"
            qos: "1"
            retain: "true"
```

```sh
cd ~/test-mqtt3 && recur start --foreground &
recur register

# Subscribe to the output topic in background
mosquitto_sub -h localhost -t "archive/#" -v &

mosquitto_pub -h localhost -t source/events -q 1 -m "event1"
# expect mosquitto_sub to print: archive/source/events archived: event1

# Verify retained message survives
mosquitto_sub -h localhost -t "archive/source/events" -C 1
# expect to receive the retained message immediately
```

## Test 4: publish shorthand + clean_session + keepalive

Covers: shorthand publish syntax, clean_session=false, custom keepalive, custom client_id.

```yaml
# ~/test-mqtt4/recur.yaml
Shorthand:
  on:
    - type: MessageReceived
      options:
        broker: "tcp://localhost:1883"
        topic: "trigger/ping"
        client_id: "recur-test-sub"
        clean_session: "false"
        keepalive: "10"
      do:
        - publish: "status/pong"
```

```sh
cd ~/test-mqtt4 && recur start --foreground &
recur register

mosquitto_sub -h localhost -t "status/pong" -v &
mosquitto_pub -h localhost -t trigger/ping -m "ping"
# expect: status/pong (empty payload from shorthand)
```

## Test 5: Plugin config (shared broker credentials)

Covers: plugin-level broker/username/password config, per-trigger override takes priority.

```sh
recur config set plugins.core.mqtt.broker "tcp://localhost:1883"
recur config set plugins.core.mqtt.username "admin"
recur config set plugins.core.mqtt.password "secret"
```

```yaml
# ~/test-mqtt5/recur.yaml
PluginConfig:
  on:
    - type: MessageReceived
      options:
        topic: "cfg/test"
      do:
        - shell: "echo 'FROM PLUGIN CFG: {{.Payload}}'"

OverrideConfig:
  on:
    - type: MessageReceived
      options:
        broker: "tcp://localhost:1883"
        topic: "cfg/override"
      do:
        - shell: "echo 'OVERRIDE: {{.Payload}}'"
```

```sh
cd ~/test-mqtt5 && recur start --foreground &
recur register

# If broker has auth, both should work; if no auth, both connect fine
mosquitto_pub -h localhost -t cfg/test -m "shared"       # expect FROM PLUGIN CFG
mosquitto_pub -h localhost -t cfg/override -m "explicit"  # expect OVERRIDE
```

## Cleanup

```sh
docker rm -f mosquitto
```

## What to verify

- [ ] MessageReceived fires on matching topic
- [ ] Messages on non-matching topics are ignored
- [ ] `+` and `#` wildcards match correctly
- [ ] All context variables (Topic, Payload, QoS, Retained, MessageID) are populated
- [ ] publish action delivers messages to the correct topic
- [ ] publish shorthand syntax works (topic only, empty payload)
- [ ] QoS 1 delivery works for both trigger and action
- [ ] retain=true causes message to persist on broker
- [ ] clean_session, keepalive, and client_id options are accepted without error
- [ ] Plugin-level config provides default broker/credentials
- [ ] Per-trigger options override plugin-level config
- [ ] Plugin shows up in `recur list plugins`
- [ ] `recur inspect plugin mqtt` shows 1 trigger and 1 action
- [ ] State persists across daemon restarts (LastFired timestamp)
