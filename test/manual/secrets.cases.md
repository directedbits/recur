# Secrets Handling - Manual Test Cases

Minimal set of manual tests to cover secrets parsing, resolution, template function, env var filtering, log redaction, API redaction, and config interpolation.

## Test 1: Env var secrets with {{secret}} template function

Covers: secrets section parsing, env var resolution, {{secret "name"}} in action commands.

```yaml
# ~/test-secrets1/recur.yaml
secrets:
  api_key: ${TEST_API_KEY}

Notify:
  on:
    - type: Cron
      options:
        schedule: "*/5 * * * *"
      do:
        - shell: "echo 'key={{secret \"api_key\"}}'"
```

```sh
export TEST_API_KEY="sk-test-12345"
cd ~/test-secrets1 && recur start --foreground &
recur register

recur test trigger Cron --set schedule="*/5 * * * *"
# expect: key=sk-test-12345
```

## Test 2: File-based secret

Covers: !file tag, file contents trimmed of whitespace.

```yaml
# ~/test-secrets2/recur.yaml
secrets:
  token: !file /tmp/test-token.txt

Deploy:
  on:
    - type: Cron
      options:
        schedule: "0 * * * *"
      do:
        - shell: "echo 'token={{secret \"token\"}}'"
```

```sh
echo -n "  file-secret-value  " > /tmp/test-token.txt
cd ~/test-secrets2 && recur start --foreground &
recur register

recur test trigger Cron
# expect: token=file-secret-value (whitespace trimmed)
```

## Test 3: Default and required env var secrets

Covers: ${VAR:-default} fallback, ${VAR:?error} required with custom message.

```yaml
# ~/test-secrets3/recur.yaml
secrets:
  optional: ${MAYBE_SET:-fallback_value}
  required: ${MUST_SET:?You must set MUST_SET}

Defaults:
  on:
    - type: Cron
      options:
        schedule: "0 * * * *"
      do:
        - shell: "echo 'opt={{secret \"optional\"}} req={{secret \"required\"}}'"
```

```sh
# Test with defaults
unset MAYBE_SET
export MUST_SET="provided"
cd ~/test-secrets3 && recur start --foreground &
recur register

recur test trigger Cron
# expect: opt=fallback_value req=provided

# Stop daemon, test missing required
recur stop
unset MUST_SET
recur start --foreground &
recur register
# expect: error during registration or execution: "You must set MUST_SET"
```

## Test 4: Sensitive options excluded from RECUR_OPT_* env vars

Covers: sensitive option values delivered via stdin JSON only, not env vars.

```yaml
# ~/test-secrets4/recur.yaml
secrets:
  hmac: ${WEBHOOK_SECRET}

SignedHook:
  on:
    - type: WebhookReceived
      options:
        port: "9095"
        path: "/hook"
        method: "POST"
        secret: '{{secret "hmac"}}'
      do:
        - shell: "echo 'fired'"
```

```sh
export WEBHOOK_SECRET="my-hmac-secret"
cd ~/test-secrets4 && recur start --foreground &
recur register

# Verify the secret option is NOT in /proc env vars
PID=$(pgrep -f "webhook")
cat /proc/$PID/environ | tr '\0' '\n' | grep RECUR_OPT
# expect: RECUR_OPT_SECRET should NOT appear
# expect: RECUR_OPT_PORT, RECUR_OPT_PATH, RECUR_OPT_METHOD should appear

# Verify the webhook still works (secret delivered via stdin JSON)
BODY='{"test":true}'
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "my-hmac-secret" | awk '{print $2}')
curl -X POST http://localhost:9095/hook \
  -H "X-Hub-Signature-256: sha256=$SIG" \
  -d "$BODY"
# expect: fired
```

## Test 5: Log redaction

Covers: secret values replaced with [REDACTED] in daemon log output.

```yaml
# ~/test-secrets5/recur.yaml
secrets:
  password: ${SECRET_PASS}

LogTest:
  on:
    - type: Cron
      options:
        schedule: "*/1 * * * *"
      do:
        - shell: "echo 'connecting with {{secret \"password\"}}'"
```

```sh
export SECRET_PASS="super-secret-password-123"
cd ~/test-secrets5 && recur start --foreground 2>&1 | tee /tmp/daemon.log &
recur register

recur test trigger Cron

# Check daemon log for redaction
grep "super-secret-password-123" /tmp/daemon.log
# expect: NO matches — secret value should not appear in logs

grep "REDACTED" /tmp/daemon.log
# expect: [REDACTED] appears where the secret value would be
```

## Test 6: API redaction in inspect output

Covers: sensitive: true manifest field causes inspect to show *** for option values.

```yaml
# ~/test-secrets6/recur.yaml
SignedHook:
  on:
    - type: WebhookReceived
      options:
        port: "9096"
        path: "/hook"
        secret: "plaintext-secret-here"
      do:
        - shell: "echo 'ok'"
```

```sh
cd ~/test-secrets6 && recur start --foreground &
recur register

recur inspect trigger WebhookReceived
# expect: port = 9096
# expect: path = /hook
# expect: secret = *** (redacted because manifest declares sensitive: true)
```

## Test 7: Config env var interpolation

Covers: ${VAR} replacement in config.yaml plugin section values.

```sh
export MQTT_PASSWORD="broker-secret-456"

cat > ~/.config/recur/config.yaml << 'EOF'
plugins:
  core.mqtt:
    broker: "tcp://localhost:1883"
    username: "recur"
    password: "${MQTT_PASSWORD}"
EOF

recur config get plugins.core.mqtt.password
# expect: broker-secret-456 (interpolated from env var)

recur config get plugins.core.mqtt.broker
# expect: tcp://localhost:1883 (unchanged, no ${} reference)
```

## Test 8: Undefined secret error

Covers: explicit error when {{secret}} references a name not in the secrets section.

```yaml
# ~/test-secrets8/recur.yaml
secrets:
  defined: ${SOME_VAR}

BadRef:
  on:
    - type: Cron
      options:
        schedule: "0 * * * *"
      do:
        - shell: "echo '{{secret \"undefined_name\"}}'"
```

```sh
export SOME_VAR="value"
cd ~/test-secrets8 && recur start --foreground &
recur register

recur test trigger Cron
# expect: error mentioning 'undefined secret "undefined_name"'
# expect: action fails, does NOT silently produce empty string
```

## Test 9: Template injection regression

Covers: context variable containing {{secret "X"}} is treated as literal text, not evaluated.

```yaml
# ~/test-secrets9/recur.yaml
secrets:
  real_secret: ${REAL_SECRET}

Injection:
  on:
    - type: WebhookReceived
      options:
        port: "9097"
        path: "/inject"
      do:
        - shell: "echo 'body={{.RequestBody}}'"
```

```sh
export REAL_SECRET="actual-secret-value"
cd ~/test-secrets9 && recur start --foreground &
recur register

# Send a request body that looks like a secret reference
curl -X POST http://localhost:9097/inject \
  -d '{{secret "real_secret"}}'
# expect: body={{secret "real_secret"}} (literal text, NOT "actual-secret-value")
```

## What to verify

- [ ] `{{secret "name"}}` resolves env var secrets in action commands
- [ ] `!file` secrets are read and trimmed from file contents
- [ ] `${VAR:-default}` provides fallback when env var is unset
- [ ] `${VAR:?msg}` errors with custom message when env var is unset
- [ ] Sensitive options (`sensitive: true` in manifest) excluded from `RECUR_OPT_*` env vars
- [ ] Sensitive options still delivered via stdin JSON to plugins
- [ ] Secret values replaced with `[REDACTED]` in daemon logs
- [ ] `recur inspect` shows `***` for sensitive options
- [ ] `${VAR}` in config.yaml plugin values interpolated from environment
- [ ] Undefined secret reference produces explicit error, not empty string
- [ ] Context variables containing `{{secret "X"}}` are literal text, not evaluated
- [ ] Secrets resolved at execution time (fresh on each trigger fire)
