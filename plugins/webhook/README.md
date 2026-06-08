---
title: "Webhook"
weight: 2
description: "HTTP/HTTPS webhook receiver with HMAC verification"
---

# Webhook Plugin

HTTP/HTTPS webhook trigger that fires when incoming requests match a configured path and method.

## Triggers

### WebhookReceived

Fires when an HTTP request matches the configured path and method.

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `port` | yes | — | Port to listen on |
| `path` | no | `/` | URL path to match (exact match) |
| `method` | no | `all` | HTTP method filter (`POST`, `GET`, `all`, etc.) |
| `max_body_size` | no | `1048576` | Maximum request body size in bytes (default 1 MB) |
| `secret` | no | — | HMAC-SHA256 shared secret for signature verification |
| `signature_header` | no | `X-Hub-Signature-256` | Header containing the HMAC signature |
| `tls_cert` | no | — | Path to TLS certificate file (enables HTTPS) |
| `tls_key` | no | — | Path to TLS private key file |
| `rate_limit` | no | `0` | Maximum requests per second (0 = unlimited) |
| `retry_after` | no | `1` | Value for the Retry-After header in 429 responses (seconds) |

When `secret` is set, requests must include a valid HMAC-SHA256 signature in the `signature_header`. The signature can be prefixed with `sha256=` (GitHub-style) or be a raw hex digest.

## Context Variables

| Variable | Description |
|----------|-------------|
| `RequestMethod` | HTTP method of the request |
| `RequestPath` | URL path |
| `RequestBody` | Request body (up to `max_body_size` bytes) |
| `QueryString` | Raw query string without leading `?` |
| `RemoteAddr` | Client address |
| `ContentType` | Content-Type header |
| `Headers` | JSON-encoded map of all request headers |
| `UserAgent` | User-Agent header |
| `Referer` | Referer header |
| `XForwardedFor` | X-Forwarded-For header |

## Examples

### GitHub webhook

```yaml
GitHubDeploy:
  on:
    - type: WebhookReceived
      options:
        port: "9090"
        path: "/deploy"
        method: "POST"
        secret: "my-webhook-secret"
  do:
    - shell: "cd /opt/app && git pull && make deploy"
```

### Generic JSON webhook

```yaml
Ingest:
  on:
    - type: WebhookReceived
      options:
        port: "8080"
        path: "/api/events"
        method: "POST"
        max_body_size: "524288"
  do:
    - shell: "echo '{{ .RequestBody }}' | jq . >> /var/log/events.json"
```

### HTTPS with TLS

```yaml
SecureHook:
  on:
    - type: WebhookReceived
      options:
        port: "8443"
        path: "/hook"
        tls_cert: "/etc/recur/cert.pem"
        tls_key: "/etc/recur/key.pem"
  do:
    - shell: "process-event.sh"
```

## Rate Limiting

When `rate_limit` is set, the server returns HTTP 429 (Too Many Requests) when the
request rate exceeds the configured limit. The `Retry-After` header indicates how
long the caller should wait before retrying.

The server also returns 429 when its internal event buffer is full (64 events),
regardless of the rate limit setting. This prevents silent event loss -- callers
receive explicit backpressure feedback.

Note: The rate limit is per server instance. If multiple triggers share the same
port (via different paths), they share a single rate limit.

### Accept any method

```yaml
CatchAll:
  on:
    - type: WebhookReceived
      options:
        port: "9090"
        path: "/log"
        method: "all"
  do:
    - shell: "echo '{{ .RequestMethod }} {{ .RequestPath }} from {{ .RemoteAddr }}' >> /var/log/hooks.log"
```
