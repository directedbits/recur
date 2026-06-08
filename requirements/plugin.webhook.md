# Plugin: webhook

External trigger plugin that starts an HTTP server and fires events when incoming
requests match configured path and method filters. Uses `net/http` from stdlib with
no external dependencies. Validates the "inbound network" pattern for future plugins
(e.g., MQTT, generic TCP).

## Plugin Identity

| Field       | Value                                          |
|-------------|------------------------------------------------|
| Name        | `webhook`                                      |
| Namespace   | `core.webhook`                                 |
| Version     | `0.1.0`                                        |
| Binary      | `webhook`                                      |

## Trigger Types

### WebhookReceived

Fires when an HTTP request arrives that matches the configured path and method.

**Options:**

| Option          | Type   | Default     | Description                                  |
|-----------------|--------|-------------|----------------------------------------------|
| `port`          | string | *required*  | Port to listen on                            |
| `path`          | string | `"/"`       | URL path to match (exact match)              |
| `method`        | string | `"all"`     | HTTP method filter (`POST`, `GET`, `all`, etc.) |
| `max_body_size` | string | `"1048576"` | Max request body in bytes                    |
| `secret`        | string | *(optional)* | HMAC-SHA256 shared secret for signature verification |
| `signature_header` | string | `"X-Hub-Signature-256"` | Header containing the HMAC signature |
| `tls_cert`      | string | *(optional)* | Path to TLS certificate file (enables HTTPS) |
| `tls_key`       | string | *(optional)* | Path to TLS private key file (requires `tls_cert`) |
| `rate_limit`    | string | `"0"`       | Maximum requests per second (0 = unlimited). Returns HTTP 429 when exceeded. |
| `retry_after`   | string | `"1"`       | Value for the `Retry-After` header in 429 responses (seconds) |

**Context Variables:**

| Variable          | Type   | Description                                    |
|-------------------|--------|------------------------------------------------|
| `RequestMethod`   | string | HTTP method (GET, POST, etc.)                  |
| `RequestPath`     | string | URL path of the request                        |
| `RequestBody`     | string | Request body (up to max_body_size bytes)        |
| `QueryString`     | string | Raw query string (without leading `?`)         |
| `RemoteAddr`      | string | Remote address of the client                   |
| `ContentType`     | string | Content-Type header value                      |
| `Headers`         | string | JSON-encoded map of all request headers        |
| `UserAgent`       | string | User-Agent header value                        |
| `Referer`         | string | Referer header value                           |
| `XForwardedFor`   | string | X-Forwarded-For header value                   |

## HTTP Response Behavior

The handler returns immediately after validation — it does NOT wait for the daemon to
process the event or for actions to complete.

| Condition       | Status | Body                       |
|-----------------|--------|----------------------------|
| Match           | 200    | `{"status":"accepted"}`    |
| Path mismatch   | 404    | `{"error":"not found"}`    |
| Method mismatch | 405    | `{"error":"method not allowed"}` |
| Rate limit exceeded | 429 | `{"error":"rate limit exceeded"}` (with `Retry-After` header) |
| Body too large  | 413    | `{"error":"body too large"}` |
| Missing signature (when secret set) | 401 | `{"error":"missing signature"}` |
| Invalid signature (when secret set) | 401 | `{"error":"invalid signature"}` |
| Channel full (backpressure) | 429 | `{"error":"server busy"}` (with `Retry-After` header) |

## Graceful Shutdown

On SIGTERM, the plugin calls `http.Server.Shutdown()` with a 5-second context deadline,
allowing in-flight requests to complete before the server stops.

## Protocol

This plugin follows the [trigger plugin protocol](plugin-protocol.md):

1. Reads trigger type and options from stdin JSON
2. Reads `RECUR_SOCKET` and `RECUR_TRIGGER_ID` from environment
3. Parses options: `port` (required), `path`, `method`, `max_body_size`, `secret`, `signature_header`, `tls_cert`, `tls_key`, `rate_limit`, `retry_after`
4. Starts HTTP server on the configured port
5. Connects to daemon gRPC socket
6. Event loop: HTTP request received → validate → build context → call `ReportTriggerEvent`
7. On SIGTERM: graceful shutdown (5s deadline), close gRPC, exit 0

## Rate Limiting

When `rate_limit` is set to a value greater than 0, the plugin uses a token-bucket rate
limiter (`golang.org/x/time/rate`) to cap inbound requests per second. Requests exceeding
the limit receive HTTP 429 with a `Retry-After` header set to the configured `retry_after`
value. Additionally, if the internal event channel is full (backpressure from the daemon),
the handler returns 429 with `{"error":"server busy"}` instead of silently dropping events.

## TLS

When `tls_cert` and `tls_key` are both set, the server starts in HTTPS mode using the
provided certificate and key files. The listener is wrapped with `tls.NewListener`.

## Future Enhancements

## Example Recurfile

```yaml
GitHub Push:
  on:
    - type: WebhookReceived
      options:
        port: "9090"
        path: "/github"
        method: "POST"
  do:
    - shell: >
        echo "Webhook from {{.RemoteAddr}}: {{.RequestBody}}" >> ~/webhook-log.txt

Health Check:
  on:
    - type: WebhookReceived
      options:
        port: "9091"
        path: "/health"
        method: "GET"
  do:
    - shell: >
        echo "Health check at $(date)" >> ~/health-log.txt
```
