---
title: "Testing Guide"
weight: 3
description: "Unit test patterns, mock strategies, and coverage enforcement"
---

## Running Tests

All test commands use [Task](https://taskfile.dev):

| Command | What it runs |
|---------|-------------|
| `task test` | Unit tests for `./src/...` |
| `task test:plugins` | Unit tests for all plugins (`./plugins/*/...`) |
| `task test:e2e` | End-to-end tests (`./test/e2e/`) |
| `task test:all` | All of the above, sequentially |

Run a single package:

```sh
go test ./src/app/trigger/...
go test ./plugins/timer/...
```

## Unit Test Patterns

### Temporary directories

Use `t.TempDir()` for any test that needs filesystem access. It is automatically cleaned up:

```go
dir := t.TempDir()
testFile := filepath.Join(dir, "test.txt")
os.WriteFile(testFile, []byte("hello"), 0644)
```

### Table-driven tests

Use table-driven tests for validating multiple input/output combinations:

```go
func TestStartInterval_Errors(t *testing.T) {
    cases := []struct{ name, every string }{
        {"invalid", "not-a-duration"},
        {"negative", "-5s"},
        {"zero", "0s"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            _, _, err := StartInterval(tc.every, false)
            if err == nil {
                t.Fatal("expected error")
            }
        })
    }
}
```

### Time-based tests

For tests involving timers and event channels:

- Use **short durations** for the thing being tested (10ms-20ms intervals).
- Use **generous timeouts** for assertions (500ms-5s) to avoid flaky failures.

```go
events, stop, err := StartInterval("20ms", false)
defer stop()

select {
case tick := <-events:
    // assert
case <-time.After(500 * time.Millisecond):
    t.Fatal("timed out waiting for tick")
}
```

## Mock Patterns

The codebase uses three main mock strategies, all without external mock libraries.

### Injectable function variables

Package-level function variables that default to real implementations but can be swapped in tests. Defined in `src/app/cli/dial.go`:

```go
var (
    dialFunc      = watchgrpc.Dial
    dialOrNilFunc = watchgrpc.DialOrNil
)
```

The MQTT plugin uses the same pattern with `clientFactory` in `plugins/mqtt/client.go`:

```go
var clientFactory MQTTClientFactory = defaultClientFactory
```

Tests override the variable, then restore it in cleanup:

```go
origFactory := clientFactory
clientFactory = func(opts *mqtt.ClientOptions) MQTTClient {
    return &mockClient{}
}
defer func() { clientFactory = origFactory }()
```

### Interface mocks

Define a narrow interface for the external dependency, then implement it as a mock in tests.

**DBusConn** (`plugins/devicemonitor/conn.go`):

```go
type DBusConn interface {
    AddMatchSignal(options ...dbus.MatchOption) error
    Signal(ch chan<- *dbus.Signal)
}
```

**MQTTClient** (`plugins/mqtt/client.go`):

```go
type MQTTClient interface {
    Connect() mqtt.Token
    Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token
    Unsubscribe(topics ...string) mqtt.Token
    Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
    Disconnect(quiesce uint)
}
```

**TriggerLookup** (`src/app/trigger/engine.go`):

```go
type TriggerLookup interface {
    GetTrigger(id string) *trigger.Trigger
    GetActionsForTrigger(triggerID string) []*action.Action
}
```

Test implementations (e.g., `mockLookup` in `engine_test.go`) use simple maps:

```go
type mockLookup struct {
    triggers map[string]*trigger.Trigger
    actions  map[string][]*action.Action
}
```

### Mock gRPC service with bufconn

The CLI tests use `google.golang.org/grpc/test/bufconn` to create an in-memory gRPC server. Defined in `src/app/cli/mock_test.go`:

```go
type mockService struct {
    recurv1.UnimplementedRecurServiceServer
    listTriggersResp *recurv1.ListTriggersResponse
    getStatusResp    *recurv1.GetStatusResponse
    // ... one field per RPC response
}
```

Each method returns the pre-set response (or a sensible default if nil).

## CLI Testing with bufconn

The `startMockDaemon` helper in `src/app/cli/mock_test.go` wires everything together:

1. Creates a `bufconn.Listen(bufSize)` in-memory listener.
2. Starts a `grpc.NewServer()` with the `mockService` registered.
3. Overrides `dialFunc` and `dialOrNilFunc` to return clients connected through the bufconn.
4. Returns a cleanup function that stops the server and restores the original dial functions.

Usage in a test:

```go
func TestListTriggers(t *testing.T) {
    svc := &mockService{
        listTriggersResp: &recurv1.ListTriggersResponse{
            Triggers: []*recurv1.TriggerSummary{{Id: "abc123", Name: "Cron"}},
        },
    }
    cleanup := startMockDaemon(t, svc)
    defer cleanup()

    // Now run the CLI command -- it will connect to the mock server
    // ...
}
```

## E2E Tests

End-to-end tests live in `test/e2e/` and exercise the full stack (build binaries, start daemon, run CLI commands, verify output).

### TestMain

`TestMain` in `test/e2e/helpers_test.go` builds both `recur` and `recurd` binaries once into a shared temp directory before any tests run:

```go
func TestMain(m *testing.M) {
    tmp, _ := os.MkdirTemp("", "recur-e2e-*")
    defer os.RemoveAll(tmp)
    binDir = tmp
    // go build -o tmp/recur ../../src/app/recur
    // go build -o tmp/recurd ../../src/app/recurd
    os.Exit(m.Run())
}
```

### startDaemonForTest

Each test that needs a running daemon calls `startDaemonForTest(t)`. It:

1. Creates an isolated `HOME` via `t.TempDir()` so tests do not interfere with each other.
2. Runs `recur start` to launch the daemon.
3. Returns the recur binary path, home directory, and a cleanup function that runs `recur stop`.

### Test isolation

Every E2E test gets its own `HOME` directory. This means:

- Config, state, PID, and socket files are isolated per test.
- Tests can run in parallel without conflicting.

## Coverage Enforcement

CI enforces a **75% coverage threshold** on the core codebase (`./src/...`).

### Exclusions

The following are filtered out of the coverage report:

- `src/cmd/` -- entry point `main()` functions
- `src/app/cli/` -- tested via E2E and bufconn mocks, not unit-measurable
- `src/infra/grpc/v1/` -- generated protobuf code

### Local coverage check

```sh
go test -coverprofile=coverage.out ./src/...

# Filter exclusions
head -1 coverage.out > coverage-filtered.out
grep -vE 'src/cmd/|src/app/cli/|src/infra/grpc/v1/' coverage.out | tail -n +2 >> coverage-filtered.out

# Check total
go tool cover -func=coverage-filtered.out | grep '^total:'
```

### CI step

The coverage step in `.github/workflows/ci.yml` runs the same filtering, compares the total against the 75% threshold, and fails the build if it drops below.

## Plugin Test Conventions

Each plugin follows a two-file test pattern:

| File | What it tests |
|------|--------------|
| `main_test.go` | `parseInput` -- JSON parsing, validation, edge cases (invalid JSON, missing fields, unsupported types). Uses `strings.NewReader` to feed stdin. |
| `<core>_test.go` | Event source logic -- channel behavior, start/stop, error conditions, configuration validation. For timer: `timer_test.go` tests `StartCron` and `StartInterval`. |

This separation keeps input parsing tests fast and deterministic, while event source tests can use short durations for real timing behavior.
