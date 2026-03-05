# Webhook Plugin - Manual Test Cases

Minimal set of manual tests to cover the plugin surface.

## Setup

```sh
task build:webhook
mkdir -p ~/.config/recur/plugins/webhook
cp bin/plugins/webhook/* ~/.config/recur/plugins/webhook/
```

## Test 1: Basic POST webhook + context variables

Covers: port, path, method filter, RequestBody, RequestMethod, RequestPath, ContentType, QueryString context variables.

```yaml
# ~/test-webhook/recur.yaml
PostHook:
  on:
    - type: WebhookReceived
      options:
        port: "9090"
        path: "/hook"
        method: "POST"
      do:
        - shell: "echo 'METHOD={{.RequestMethod}} PATH={{.RequestPath}} BODY={{.RequestBody}} CT={{.ContentType}} QS={{.QueryString}}'"
```

```sh
cd ~/test-webhook && recur start --foreground &
recur register

curl -X POST http://localhost:9090/hook?foo=bar \
  -H "Content-Type: application/json" \
  -d '{"event":"test"}'
# expect: METHOD=POST PATH=/hook BODY={"event":"test"} CT=application/json QS=foo=bar

curl -X GET http://localhost:9090/hook
# expect: NO event (method filtered to POST)

curl -X POST http://localhost:9090/other -d 'x'
# expect: NO event (path doesn't match)
```

## Test 2: HMAC-SHA256 signature verification

Covers: secret, signature_header (default X-Hub-Signature-256), valid/invalid signatures, GitHub-style sha256= prefix.

```yaml
# ~/test-webhook2/recur.yaml
SignedHook:
  on:
    - type: WebhookReceived
      options:
        port: "9091"
        path: "/signed"
        method: "POST"
        secret: "test-secret"
      do:
        - shell: "echo 'VERIFIED: {{.RequestBody}}'"
```

```sh
cd ~/test-webhook2 && recur start --foreground &
recur register

# compute valid signature
BODY='{"deploy":true}'
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "test-secret" | awk '{print $2}')

# valid signature (raw hex)
curl -X POST http://localhost:9091/signed \
  -H "X-Hub-Signature-256: $SIG" \
  -d "$BODY"
# expect: VERIFIED: {"deploy":true}

# valid signature (GitHub-style sha256= prefix)
curl -X POST http://localhost:9091/signed \
  -H "X-Hub-Signature-256: sha256=$SIG" \
  -d "$BODY"
# expect: VERIFIED: {"deploy":true}

# invalid signature
curl -X POST http://localhost:9091/signed \
  -H "X-Hub-Signature-256: deadbeef" \
  -d "$BODY"
# expect: HTTP 403, NO event fired
```

## Test 3: method=all + header context variables

Covers: method=all accepts any HTTP method, Headers, UserAgent, Referer, XForwardedFor, RemoteAddr context variables.

```yaml
# ~/test-webhook3/recur.yaml
CatchAll:
  on:
    - type: WebhookReceived
      options:
        port: "9092"
        path: "/log"
        method: "all"
      do:
        - shell: "echo '{{.RequestMethod}} from={{.RemoteAddr}} ua={{.UserAgent}} ref={{.Referer}} xff={{.XForwardedFor}}'"
```

```sh
cd ~/test-webhook3 && recur start --foreground &
recur register

curl -X GET http://localhost:9092/log \
  -H "User-Agent: test-agent" \
  -H "Referer: https://example.com" \
  -H "X-Forwarded-For: 10.0.0.1"
# expect: GET from=127.0.0.1:... ua=test-agent ref=https://example.com xff=10.0.0.1

curl -X PUT http://localhost:9092/log -d 'data'
# expect: PUT event fires (method=all)

curl -X DELETE http://localhost:9092/log
# expect: DELETE event fires (method=all)
```

## Test 4: Rate limiting + max_body_size

Covers: rate_limit, retry_after, HTTP 429 response, max_body_size enforcement.

```yaml
# ~/test-webhook4/recur.yaml
RateLimited:
  on:
    - type: WebhookReceived
      options:
        port: "9093"
        path: "/limited"
        method: "POST"
        rate_limit: "2"
        retry_after: "5"
        max_body_size: "64"
      do:
        - shell: "echo 'OK: {{.RequestBody}}'"
```

```sh
cd ~/test-webhook4 && recur start --foreground &
recur register

# rapid-fire requests to exceed rate limit (2/sec)
for i in 1 2 3 4 5; do
  curl -s -o /dev/null -w "req$i: %{http_code}\n" \
    -X POST http://localhost:9093/limited -d "hit-$i"
done
# expect: first 2 return 200, subsequent return 429
# expect: 429 responses include Retry-After: 5 header

# verify Retry-After header
curl -s -D - -X POST http://localhost:9093/limited -d "x" | grep -i retry-after
# expect: Retry-After: 5 (if rate limited)

# body exceeding max_body_size (64 bytes)
curl -s -o /dev/null -w "%{http_code}\n" \
  -X POST http://localhost:9093/limited \
  -d "$(python3 -c 'print("A" * 100)')"
# expect: HTTP 413 or truncated body, NO crash
```

## Test 5: Default path (/) + default method (all)

Covers: omitting path defaults to /, omitting method defaults to all, minimal required config (port only).

```yaml
# ~/test-webhook5/recur.yaml
Defaults:
  on:
    - type: WebhookReceived
      options:
        port: "9094"
      do:
        - shell: "echo 'DEFAULT: {{.RequestMethod}} {{.RequestPath}}'"
```

```sh
cd ~/test-webhook5 && recur start --foreground &
recur register

curl http://localhost:9094/
# expect: DEFAULT: GET /

curl -X POST http://localhost:9094/
# expect: DEFAULT: POST /

curl http://localhost:9094/other
# expect: NO event (path doesn't match /)
```

## What to verify

- [ ] Trigger fires only for matching path + method combinations
- [ ] method=all accepts GET, POST, PUT, DELETE, etc.
- [ ] Default path (/) and default method (all) work when omitted
- [ ] HMAC verification accepts valid signatures (raw hex and sha256= prefix)
- [ ] HMAC verification rejects invalid signatures with HTTP 403
- [ ] Rate limiting returns HTTP 429 with correct Retry-After header
- [ ] max_body_size limits accepted body length
- [ ] All context variables are populated (RequestMethod, RequestPath, RequestBody, QueryString, RemoteAddr, ContentType, Headers, UserAgent, Referer, XForwardedFor)
- [ ] Plugin shows up in `recur list plugins`
- [ ] `recur inspect plugin webhook` shows WebhookReceived trigger
- [ ] State persists across daemon restarts (LastFired timestamp)
