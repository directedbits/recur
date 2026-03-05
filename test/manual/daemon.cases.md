# Daemon CLI - Manual Test Cases

Minimal set of manual tests to cover the core daemon CLI surface.

## Setup

```sh
task build
mkdir -p /tmp/recur-test && cd /tmp/recur-test
```

```yaml
# /tmp/recur-test/recur.yaml
Greeting:
  on:
    - type: timer
      name: "Minute tick"
      options:
        interval: 60s
      do:
        - type: shell
          name: "Say hello"
          options:
            command: "echo 'hello from {{.TriggeredOn}}'"
```

## Test 1: Start, Status, Stop, Restart

Covers: start (background + foreground), stop, restart, status, --json flag, --verbose flag, version.

```sh
recur version                       # expect version string
recur status                        # expect "not running"
recur status --json                 # expect {"running": false}
recur status --verbose              # expect paths (Config, State, Socket, PID File, Plugins)
recur start                         # expect "Daemon started (pid ...)"
recur start                         # expect error "daemon already running"
recur status                        # expect "running (pid ...)" with trigger/action counts
recur status --json                 # expect JSON with running, pid, uptime, active_triggers, etc.
recur restart                       # expect "Daemon stopped" then "Daemon started"
recur stop                          # expect "Daemon stopped"
recur stop                          # expect error "daemon is not running"

# Foreground mode (separate terminal)
recur start --foreground            # expect log output to stdout, Ctrl+C to stop
```

## Test 2: Register, Reload, Verify, Deregister

Covers: register (auto-discover + explicit path + by ID), verify, deregister, --json flag, reload on re-register.

```sh
recur start
cd /tmp/recur-test

recur verify                        # expect "Valid: .../recur.yaml" with Groups/Triggers/Actions counts
recur verify --json                 # expect JSON with valid, trigger_count, action_count
recur register                      # expect "Registered: ... (id: ...)" — note the ID
recur register                      # expect "Reloaded: ..." (re-register = atomic reload)
recur register /tmp/recur-test/recur.yaml  # expect same result with explicit path
recur list recurfiles               # expect one entry — note the recurfile ID
recur register <recurfile-id>       # expect "Reloaded" (register by ID)
recur deregister <recurfile-id>     # expect "Deregistered: ..." with trigger/action removal counts
recur deregister <recurfile-id>     # expect error "not found"
recur deregister /tmp/recur-test/recur.yaml  # re-register first, then deregister by path

recur stop
```

## Test 3: List Entities

Covers: list triggers, list actions, list groups, list plugins, list recurfiles, --json flag, --suspended filter.

```sh
recur start
cd /tmp/recur-test && recur register

recur list triggers                 # expect table with ID, name, group, status, plugin
recur list triggers --json          # expect JSON array
recur list actions                  # expect table of actions
recur list actions --json           # expect JSON array
recur list groups                   # expect "Greeting" group with trigger/action counts
recur list plugins                  # expect installed plugins with version and status
recur list recurfiles               # expect registered recurfile with path and counts

# With suspended filter (after suspending in Test 5)
recur list triggers --suspended     # expect only suspended triggers

recur stop
```

## Test 4: Inspect Entities

Covers: inspect (auto-detect type), inspect trigger, inspect action, inspect group, inspect plugin, inspect recurfile, --json flag, --verbose flag.

```sh
recur start
cd /tmp/recur-test && recur register

# Get IDs from list commands
TRIGGER_ID=$(recur list triggers --json | jq -r '.[0].id')
ACTION_ID=$(recur list actions --json | jq -r '.[0].id')
GROUP_NAME="Greeting"
WATCHFILE_ID=$(recur list recurfiles --json | jq -r '.[0].id')

# Auto-detect entity type
recur inspect $TRIGGER_ID           # expect trigger details (type, group, status, options)
recur inspect "Minute tick"         # expect same trigger (lookup by name)
recur inspect "Say hello"           # expect action (lookup by name)
recur inspect $GROUP_NAME           # expect group details (triggers, actions)

# Explicit subcommands
recur inspect trigger $TRIGGER_ID   # expect full trigger detail with options + context vars
recur inspect trigger $TRIGGER_ID --json     # expect JSON output
recur inspect trigger $TRIGGER_ID --verbose  # expect Plugin Dir path
recur inspect action $ACTION_ID     # expect action detail with trigger ID + execution info
recur inspect group $GROUP_NAME     # expect group with member triggers/actions
recur inspect recurfile $WATCHFILE_ID        # expect recurfile path with groups/triggers/actions
recur inspect recurfile /tmp/recur-test/recur.yaml  # expect same (path resolution)
recur inspect plugin timer          # expect plugin manifest: triggers, actions, config

recur inspect nonexistent           # expect error "entity not found"

recur stop
```

## Test 5: Suspend, Resume, Test

Covers: suspend trigger, suspend action, resume trigger, resume action, test trigger (with --set), test action (with --set).

```sh
recur start
cd /tmp/recur-test && recur register

TRIGGER_ID=$(recur list triggers --json | jq -r '.[0].id')
ACTION_ID=$(recur list actions --json | jq -r '.[0].id')

# Suspend / Resume
recur suspend trigger $TRIGGER_ID   # expect "Trigger ... suspended."
recur inspect trigger $TRIGGER_ID   # expect Status: suspended
recur list triggers --suspended     # expect the suspended trigger in the list
recur resume trigger $TRIGGER_ID    # expect "Trigger ... resumed."
recur inspect trigger $TRIGGER_ID   # expect Status: active

recur suspend action $ACTION_ID     # expect "Action ... suspended."
recur resume action $ACTION_ID      # expect "Action ... resumed."

# Manual test fire
recur test trigger $TRIGGER_ID      # expect "Trigger ... fired." with action results
recur test trigger $TRIGGER_ID --set TriggeredOn=2026-01-01T00:00:00Z  # expect custom context
recur test trigger $TRIGGER_ID --json  # expect JSON with results array
recur test action $ACTION_ID        # expect "Action ...: success" with output
recur test action $ACTION_ID --set key=value  # expect action runs with custom context

recur stop
```

## Test 6: Config Get, Set, Delete

Covers: config get (all + single key), config set, config delete, --json flag, source annotations.

```sh
recur start

recur config get                    # expect all config keys with values and source annotations
recur config get --json             # expect JSON object of all config
recur config get shutdown_timeout   # expect value like "30s (inherited from default)"
recur config set shutdown_timeout 60s  # expect "shutdown_timeout = 60s"
recur config get shutdown_timeout   # expect "60s" (no inherited annotation)
recur config delete shutdown_timeout   # expect "shutdown_timeout reverted to default"
recur config get shutdown_timeout   # expect default value with inherited annotation

# Works without daemon too
recur stop
recur config get                    # expect config read from file directly
recur config set log_level debug    # expect "log_level = debug"
recur config delete log_level       # expect reverted to default
```

## Test 7: Add Group

Covers: add with trigger type, add with --triggers/--actions, --stub flag, --user flag, auto-register.

```sh
recur start
cd /tmp/recur-test

# Add by positional args (group + trigger type)
recur add MyGroup interval             # expect "Wrote: .../recur.yaml" + auto-register
recur list groups                   # expect MyGroup in the list

# Add with flags
rm -f /tmp/recur-test2/recur.yaml && mkdir -p /tmp/recur-test2 && cd /tmp/recur-test2
recur add --triggers interval --actions shell  # expect group "Local" added with trigger + action

# Stub mode (pre-populates options from manifests)
rm -f /tmp/recur-test3/recur.yaml && mkdir -p /tmp/recur-test3 && cd /tmp/recur-test3
recur add MyStub interval --stub       # expect YAML with timer options pre-filled

# User scope
recur add --user UserGroup interval    # expect writes to ~/.config/recur/recur.yaml

recur stop
```

## Test 8: Plugin Install, Uninstall

Covers: install from directory, install --link, uninstall, daemon hot-load.

```sh
recur start

# Install from built plugin directory
recur install bin/plugins/timer     # expect "Installed (copied): timer (...) v..."
recur list plugins                  # expect timer plugin listed
recur inspect plugin timer          # expect manifest details

# Install with --link (symlink)
recur uninstall timer               # expect "Plugin timer uninstalled."
recur install --link bin/plugins/timer  # expect "Installed (linked): timer ..."
recur list plugins                  # expect timer in list

# Uninstall
recur uninstall timer               # expect "Plugin timer uninstalled."
recur list plugins                  # expect timer gone

recur stop
```

## Test 9: Ambiguous Identifier Resolution

Covers: cross-type ambiguity, intra-type ambiguity, compact vs. wide output, JSON output, exit code 2.

Setup: create a recurfile where trigger and action share the same name.

```sh
mkdir -p /tmp/recur-ambiguous && cat > /tmp/recur-ambiguous/recur.yaml << 'EOF'
Build:
  on:
    - type: Shell
      name: Shell
  do:
    - type: Shell
      name: Shell
Deploy:
  on:
    - type: Shell
      name: Shell
  do:
    - type: Shell
      name: Shell
EOF

recur start
cd /tmp/recur-ambiguous && recur register

# Cross-type ambiguity: "Shell" matches triggers + actions
recur inspect Shell           # expect "Ambiguous identifier" listing candidates with full IDs
echo $?                      # expect 1

# Disambiguate with type subcommand
recur inspect trigger Shell   # expect trigger detail (still may be ambiguous if 2 triggers named Shell)

# Intra-type ambiguity: two triggers both named "Shell" in different groups
# (Build.Shell and Deploy.Shell) — expect wide output with group + recurfile columns
recur suspend trigger Shell   # expect ambiguous error with group= and recurfile= columns

# Disambiguate with full ID
TRIGGER_ID=$(recur list triggers --json | jq -r '.[0].id')
recur inspect $TRIGGER_ID     # expect single match, trigger detail

# JSON ambiguity output
recur inspect Shell --json    # expect JSON with error, identifier, candidates[] on stdout
echo $?                      # expect 2

# Suspend with explicit ID
recur suspend $TRIGGER_ID     # expect "Trigger ... suspended."

recur stop
```

## What to verify

- [ ] start/stop/restart lifecycle works cleanly (no stale PID files)
- [ ] status shows correct running state and entity counts
- [ ] status --json returns parseable JSON with all expected fields
- [ ] status --verbose shows file paths (Config, State, Socket, PID, Plugins)
- [ ] register auto-discovers recur.yaml in CWD
- [ ] register with explicit path and by recurfile ID both work
- [ ] re-register performs atomic reload (shows "Reloaded")
- [ ] verify validates without registering
- [ ] deregister by ID and by path both work
- [ ] list triggers/actions/groups/plugins/recurfiles all produce output
- [ ] list --suspended filters correctly
- [ ] list --json returns JSON arrays
- [ ] inspect auto-detects entity type from identifier
- [ ] inspect resolves entities by optional name field
- [ ] inspect subcommands (trigger/action/group/plugin/recurfile) work
- [ ] inspect --json and --verbose produce expected extra output
- [ ] suspend/resume toggles trigger and action status
- [ ] test trigger fires and shows action results
- [ ] test action executes and shows output
- [ ] test --set passes custom context variables
- [ ] config get shows all keys with source annotations
- [ ] config set persists values (with and without daemon)
- [ ] config delete reverts to default
- [ ] add creates group in recurfile and auto-registers
- [ ] add --stub pre-populates options from manifests
- [ ] install copies/links plugin and daemon hot-loads it
- [ ] uninstall removes from daemon and disk
- [ ] --json flag works on all commands that support it
- [ ] --quiet flag suppresses output where applicable
- [ ] errors are clear when daemon is not running
