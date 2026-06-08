# Timer Plugin - Manual Test Cases

Minimal set of manual tests to cover the plugin surface.

## Setup

```sh
task build:timer
mkdir -p ~/.config/recur/plugins/timer
cp bin/plugins/timer/* ~/.config/recur/plugins/timer/
```

## Test 1: Interval + fire_on_start + context variables

Covers: interval trigger, every option, fire_on_start=true, TickCount and TimeSinceStarted context variables.

```yaml
# ~/test-timer/recur.yaml
IntervalTick:
  on:
    - type: interval
      options:
        every: "5s"
        fire_on_start: "true"
      do:
        - shell: "echo 'TICK #{{.TickCount}} elapsed={{.TimeSinceStarted}}'"
```

```sh
cd ~/test-timer && recur start --foreground &
recur register
# expect immediate TICK #1 (fire_on_start)
# expect TICK #2 after ~5s
# expect TICK #3 after ~10s
# verify TickCount increments and TimeSinceStarted grows
```

## Test 2: Cron expression + timezone

Covers: cron trigger, explicit cron expression, timezone option.

```yaml
# ~/test-timer2/recur.yaml
CronMinute:
  on:
    - type: cron
      options:
        expression: "* * * * *"
        timezone: "UTC"
      do:
        - shell: "echo 'CRON FIRED at $(date -u) tick={{.TickCount}}'"
```

```sh
cd ~/test-timer2 && recur start --foreground &
recur register
# expect CRON FIRED once per minute
# wait ~2 minutes to confirm repeated firing
```

## Test 3: Cron preset + fire_on_start

Covers: cron preset shorthand (@every), fire_on_start with cron trigger.

```yaml
# ~/test-timer3/recur.yaml
CronPreset:
  on:
    - type: cron
      options:
        expression: "@every 10s"
        fire_on_start: "true"
      do:
        - shell: "echo 'PRESET tick={{.TickCount}}'"
```

```sh
cd ~/test-timer3 && recur start --foreground &
recur register
# expect immediate PRESET tick=1 (fire_on_start)
# expect PRESET tick=2 after ~10s
```

## Test 4: Interval without fire_on_start (default)

Covers: fire_on_start defaults to false, no immediate event on activation.

```yaml
# ~/test-timer4/recur.yaml
DelayedStart:
  on:
    - type: interval
      options:
        every: "5s"
      do:
        - shell: "echo 'DELAYED tick={{.TickCount}}'"
```

```sh
cd ~/test-timer4 && recur start --foreground &
recur register
# expect NO immediate event
# expect first DELAYED tick=1 after ~5s
```

## What to verify

- [ ] interval trigger fires at the correct cadence
- [ ] cron trigger fires on schedule with explicit expressions
- [ ] Cron presets (`@every`, `@hourly`, etc.) are accepted
- [ ] `timezone` option shifts cron evaluation to the specified IANA zone
- [ ] `fire_on_start: "true"` produces an immediate event for both cron and interval
- [ ] `fire_on_start` defaults to false (no immediate event)
- [ ] Template variables (TickCount, TimeSinceStarted) are populated and increment correctly
- [ ] Plugin shows up in `recur list plugins`
- [ ] `recur inspect plugin timer` shows both cron and interval triggers
- [ ] State persists across daemon restarts (LastFired timestamp)
