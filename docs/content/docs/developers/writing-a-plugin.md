---
title: "Writing a Plugin"
weight: 2
description: "Step-by-step guide to creating a new trigger or action plugin"
---

This tutorial walks through creating a new trigger plugin from scratch, using the **timer** plugin as the reference implementation. By the end you will have a working plugin that the daemon discovers, spawns, and receives events from.

## Step 1: Create the Directory and Manifest

Create `plugins/<name>/manifest.yaml`. The manifest tells the daemon what trigger (and/or action) types your plugin provides.

```yaml
name: timer
namespace: core.timer
version: "0.1.0"
description: "Time-driven triggers via cron schedules and fixed intervals"

triggers:
  - name: cron
    description: "Fires on a cron schedule"
    options:
      - name: expression
        type: string
        description: "Cron expression or preset (@hourly, @daily, etc.)"
      - name: timezone
        type: string
        default: "Local"
        description: "IANA timezone (e.g., UTC, America/New_York)"
      - name: fire_on_start
        type: string
        default: "false"
        description: "Fire one event immediately on startup"
    context:
      - name: TickCount
        type: string
        description: "Cumulative fire count since activation"
      - name: TimeSinceStarted
        type: string
        description: "Duration since plugin started"

  - name: interval
    description: "Fires on a fixed time interval"
    options:
      - name: every
        type: string
        description: "Go duration string (e.g., 30s, 5m, 1h30m)"
      - name: fire_on_start
        type: string
        default: "false"
        description: "Fire one event immediately on startup"
    context:
      - name: TickCount
        type: string
        description: "Cumulative fire count since activation"
      - name: TimeSinceStarted
        type: string
        description: "Duration since plugin started"
```

Key fields:

- **name** -- human-readable plugin name.
- **namespace** -- unique identifier; used for plugin config lookup under `plugins.<namespace>`.
- **triggers** -- list of trigger types. Each has a `name` (PascalCase), `options` (what the user configures in their recurfile), and `context` (variables the plugin sends back with each event, available in action templates).

## Step 2: Set Up the Go Module

```sh
cd plugins/<name>
go mod init github.com/directedbits/recur/plugins/<name>
```

Add the main module as a dependency (for the gRPC client):

```sh
go mod edit -require github.com/directedbits/recur@v0.0.0
go mod edit -replace github.com/directedbits/recur=../../
```

Then add your plugin to the workspace:

```sh
# In the repo root go.work file, add:
# use ./plugins/<name>
```

## Step 3: Implement parseInput

Every plugin reads a JSON payload from stdin. Define a struct and a `parseInput` function that reads from an `io.Reader` (this makes it testable with `strings.NewReader`).

```go
type pluginInput struct {
    TriggerType string         `json:"trigger_type"`
    Options     map[string]any `json:"options"`
    Config      map[string]any `json:"config"`
}

type parsedInput struct {
    Input *pluginInput
    // Add validated, typed fields here
    Expression string
}

func parseInput(r io.Reader) (*parsedInput, error) {
    var input pluginInput
    if err := json.NewDecoder(r).Decode(&input); err != nil {
        return nil, fmt.Errorf("reading stdin: %w", err)
    }

    // Validate trigger type
    switch input.TriggerType {
    case "YourTrigger":
        // extract and validate options
    default:
        return nil, fmt.Errorf("unsupported trigger_type: %s", input.TriggerType)
    }

    return &parsedInput{Input: &input, ...}, nil
}
```

The `optString` helper pattern (extract a string option with a default fallback) is common:

```go
func optString(opts map[string]any, key, fallback string) string {
    if v, ok := opts[key].(string); ok && v != "" {
        return v
    }
    return fallback
}
```

## Step 4: Implement the Event Source

The event source is where your plugin's core logic lives. Return a channel of events, a stop function, and any setup error:

```go
func StartMySource(config ...) (<-chan MyEvent, func(), error) {
    events := make(chan MyEvent, 16)
    done := make(chan struct{})

    // Set up your event source (connect to service, start polling, etc.)

    go func() {
        defer close(events)
        for {
            select {
            case <-done:
                return
            // case event from your source:
            //     events <- MyEvent{...}
            }
        }
    }()

    stop := func() {
        close(done)
        // Clean up resources
    }

    return events, stop, nil
}
```

See the timer plugin for concrete examples:

- `StartCron(expression, timezone, fireOnStart)` -- wraps `robfig/cron` and emits `TickEvent` on each cron fire.
- `StartInterval(every, fireOnStart)` -- wraps `time.NewTicker`.

Both return `(<-chan TickEvent, func(), error)`.

## Step 5: Wire Up main()

The `main()` function ties everything together:

```go
func main() {
    log.SetPrefix("<name>: ")
    log.SetFlags(0)

    // 1. Parse stdin
    parsed, err := parseInput(os.Stdin)
    if err != nil {
        log.Fatal(err)
    }

    // 2. Read env vars set by the daemon
    socketPath := os.Getenv("RECUR_SOCKET")
    triggerID := os.Getenv("RECUR_TRIGGER_ID")
    if socketPath == "" || triggerID == "" {
        log.Fatal("RECUR_SOCKET and RECUR_TRIGGER_ID must be set")
    }

    // 3. Start event source
    events, stop, err := StartMySource(parsed.Config...)
    if err != nil {
        log.Fatalf("starting source: %v", err)
    }
    defer stop()

    // 4. Connect to daemon gRPC socket
    client, err := watchgrpc.Dial(socketPath)
    if err != nil {
        log.Fatalf("connecting to daemon: %v", err)
    }
    defer client.Close()

    // 5. Signal handling
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    // 6. Event loop
    for {
        select {
        case event, ok := <-events:
            if !ok {
                return
            }

            ctxVars := map[string]string{
                "MyField": event.MyField,
            }

            resp, err := client.Service.ReportTriggerEvent(
                context.Background(),
                &recurv1.ReportTriggerEventRequest{
                    TriggerId: triggerID,
                    Context:   ctxVars,
                },
            )
            if err != nil {
                log.Printf("reporting event: %v", err)
                continue
            }
            if !resp.Accepted {
                log.Printf("event rejected: %s", resp.Error)
            }

        case sig := <-sigCh:
            fmt.Fprintf(os.Stderr, "received %v, shutting down\n", sig)
            return
        }
    }
}
```

Key points:

- **Stderr is your log output.** The daemon captures stderr and forwards it with a `[plugin-name]` prefix. Lines containing "error", "fatal", or "panic" are logged at error level; everything else at info level.
- **Context keys must match the manifest.** The daemon validates context keys in `ReportTriggerEvent` against your manifest's `context` definitions.
- **Handle SIGTERM gracefully.** The daemon sends SIGTERM first, then SIGKILL after the shutdown timeout (default 3 seconds).

## Step 6: Add to Taskfile.yml

Add a build task for your plugin and include it in the plugin test task:

```yaml
build:<name>:
  desc: Build the <name> plugin
  cmds:
    - go build -o {{.BIN_DIR}}/plugins/<name>/<name> ./plugins/<name>/
    - cp ./plugins/<name>/manifest.yaml {{.BIN_DIR}}/plugins/<name>/

test:plugins:
  cmds:
    - go test ./plugins/<name>/... # add to existing list
```

## Step 7: Build and Install

```sh
task build:<name>
recur install ./bin/plugins/<name>
```

Or copy the plugin directory (binary + manifest.yaml) to `~/.config/recur/plugins/<name>/`.

## Step 8: Write Tests

Follow the two-file convention:

### main_test.go -- parseInput tests

Use `strings.NewReader` to feed JSON to `parseInput`:

```go
func TestParseInput_ValidTrigger(t *testing.T) {
    jsonStr := `{"trigger_type":"MyTrigger","options":{"key":"value"},"config":{}}`
    parsed, err := parseInput(strings.NewReader(jsonStr))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if parsed.Input.TriggerType != "MyTrigger" {
        t.Errorf("TriggerType = %q", parsed.Input.TriggerType)
    }
}

func TestParseInput_InvalidJSON(t *testing.T) {
    _, err := parseInput(strings.NewReader("not json"))
    if err == nil {
        t.Fatal("expected error for invalid JSON")
    }
}

func TestParseInput_UnsupportedType(t *testing.T) {
    jsonStr := `{"trigger_type":"BadType","options":{},"config":{}}`
    _, err := parseInput(strings.NewReader(jsonStr))
    if err == nil {
        t.Fatal("expected error for unsupported type")
    }
}
```

### <core>_test.go -- event source tests

Test the event source directly with short durations and generous timeouts:

```go
func TestStartMySource_Fires(t *testing.T) {
    events, stop, err := StartMySource(shortConfig)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    defer stop()

    select {
    case event := <-events:
        // assert event fields
    case <-time.After(500 * time.Millisecond):
        t.Fatal("timed out waiting for event")
    }
}
```

Use table-driven tests for validation edge cases (invalid config, missing required fields, etc.).

## Step 9: Create a README

Add a `plugins/<name>/README.md` with Hugo front matter if the plugin should appear in the docs site, or a plain README for developer reference.

## Action Plugins

Action plugins are simpler than trigger plugins. They are short-lived processes:

1. The daemon spawns the binary and writes `ActionPluginInput` JSON to stdin:
   ```json
   {"action_type": "Publish", "options": {...}, "config": {...}, "test": false}
   ```
2. The plugin performs its work (or simulates it if `test` is true).
3. The plugin writes `ActionPluginOutput` JSON to stdout and exits:
   ```json
   {"success": true, "output": "published to topic/foo", "error": ""}
   ```

No gRPC callback is needed -- the daemon reads stdout after the process exits. The `plugins/mqtt` plugin demonstrates a dual trigger+action plugin: it checks whether `trigger_type` or `action_type` is present in stdin to decide which mode to run in.

Environment variables available to action plugins:

- `RECUR_ACTION_TYPE` -- the action type name
- `RECUR_LOG_LEVEL` -- daemon's configured log level
- `RECUR_TEST=true` -- set when running via `recur test action`
- `RECUR_OPT_<KEY>` -- one per resolved option
