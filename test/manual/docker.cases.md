# Docker Plugin - Manual Test Cases

Minimal set of manual tests to cover the plugin surface.

**Note:** Requires a running Docker daemon accessible at the default socket.

## Setup

```sh
task build:docker
mkdir -p ~/.config/recur/plugins/docker
cp bin/plugins/docker/* ~/.config/recur/plugins/docker/

# Pre-pull a lightweight image for tests
docker pull alpine:latest
```

## Test 1: ContainerStarted + ContainerStopped + context variables

Covers: both lifecycle triggers, default host, context variables (ContainerID, ContainerName, Image, Status, ExitCode).

```yaml
# ~/test-docker/recur.yaml
Lifecycle:
  on:
    - type: ContainerStarted
      do:
        - shell: "echo 'STARTED: {{.ContainerName}} image={{.Image}} id={{.ContainerID}} status={{.Status}}'"
    - type: ContainerStopped
      do:
        - shell: "echo 'STOPPED: {{.ContainerName}} status={{.Status}} exit={{.ExitCode}}'"
```

```sh
cd ~/test-docker && recur start --foreground &
recur register

docker run --rm --name recur-test-1 alpine:latest echo hello   # expect STARTED then STOPPED
docker run --rm --name recur-test-2 alpine:latest sh -c 'exit 1'  # expect STOPPED with exit=1
```

## Test 2: filter_name + filter_image + filter_label

Covers: all three filter options, ensuring non-matching containers are ignored.

```yaml
# ~/test-docker2/recur.yaml
Filtered:
  on:
    - type: ContainerStarted
      options:
        filter_name: "recur-match"
      do:
        - shell: "echo 'NAME-MATCH: {{.ContainerName}}'"
    - type: ContainerStarted
      options:
        filter_image: "alpine"
      do:
        - shell: "echo 'IMAGE-MATCH: {{.ContainerName}} image={{.Image}}'"
    - type: ContainerStopped
      options:
        filter_label: "env=test"
      do:
        - shell: "echo 'LABEL-MATCH: {{.ContainerName}}'"
```

```sh
cd ~/test-docker2 && recur start --foreground &
recur register

docker run --rm --name recur-match alpine:latest true      # expect NAME-MATCH + IMAGE-MATCH
docker run --rm --name recur-nomatch alpine:latest true     # expect IMAGE-MATCH only (name filter miss)
docker run --rm --name recur-busybox busybox:latest true    # expect NO events (neither name nor image match)
docker run --rm --label env=test --name recur-label alpine:latest true  # expect IMAGE-MATCH + LABEL-MATCH on stop
```

## Test 3: HealthChanged trigger

Covers: HealthChanged trigger, HealthStatus context variable, filter_name on health events.

```yaml
# ~/test-docker3/recur.yaml
Health:
  on:
    - type: HealthChanged
      options:
        filter_name: "recur-health"
      do:
        - shell: "echo 'HEALTH: {{.ContainerName}} status={{.HealthStatus}} image={{.Image}}'"
```

```sh
cd ~/test-docker3 && recur start --foreground &
recur register

# Start a container with a health check that initially fails then succeeds
docker run -d --rm --name recur-health \
  --health-cmd "cat /tmp/healthy || exit 1" \
  --health-interval 2s \
  --health-start-period 1s \
  --health-retries 1 \
  alpine:latest sleep 30

# Wait for unhealthy status
sleep 6                                      # expect HEALTH with status=unhealthy

# Make health check pass
docker exec recur-health touch /tmp/healthy
sleep 4                                      # expect HEALTH with status=healthy

docker stop recur-health
```

## Test 4: ContainerStart + ContainerStop + ContainerRestart actions (shorthand)

Covers: all three action types, shorthand syntax, action timeout option.

```yaml
# ~/test-docker4/recur.yaml
Actions:
  on:
    - type: ContainerStopped
      options:
        filter_name: "recur-restart-target"
      do:
        - ContainerStart: "recur-restart-target"
    - type: ContainerStarted
      options:
        filter_name: "recur-action-trigger"
      do:
        - ContainerStop:
            container: "recur-stop-target"
            timeout: 5
        - ContainerRestart: "recur-restart-me"
```

```sh
cd ~/test-docker4 && recur start --foreground &
recur register

# Create containers for action targets (stopped state)
docker create --name recur-restart-target alpine:latest sleep 60
docker create --name recur-stop-target alpine:latest sleep 60
docker create --name recur-restart-me alpine:latest sleep 60

# Start targets that need to be running
docker start recur-stop-target
docker start recur-restart-me

# Trigger: stop the restart-target -- action should auto-start it
docker start recur-restart-target
docker stop recur-restart-target   # expect ContainerStart action fires, container comes back up
sleep 2
docker ps --filter name=recur-restart-target --format '{{.Status}}'  # expect "Up"

# Trigger: start the action-trigger -- should stop and restart targets
docker run -d --rm --name recur-action-trigger alpine:latest sleep 5
sleep 2
docker ps --filter name=recur-stop-target --format '{{.Status}}'    # expect exited / not running
docker ps --filter name=recur-restart-me --format '{{.Status}}'     # expect "Up" (restarted)

# Cleanup
docker rm -f recur-restart-target recur-stop-target recur-restart-me 2>/dev/null
```

## What to verify

- [ ] ContainerStarted fires when containers start
- [ ] ContainerStopped fires when containers stop or die
- [ ] HealthChanged fires on health status transitions (starting, healthy, unhealthy)
- [ ] filter_name filters by container name substring
- [ ] filter_image filters by image name substring
- [ ] filter_label filters by key=value label
- [ ] Non-matching containers produce no events
- [ ] Context variables populated: ContainerID, ContainerName, Image, Status, ExitCode, HealthStatus
- [ ] ContainerStart action starts a stopped container
- [ ] ContainerStop action stops a running container (with timeout)
- [ ] ContainerRestart action restarts a container
- [ ] Shorthand action syntax works (`ContainerRestart: "name"`)
- [ ] Plugin shows up in `recur list plugins`
- [ ] `recur inspect plugin docker` shows 3 triggers and 3 actions
