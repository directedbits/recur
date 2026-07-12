// webhook is an external trigger plugin that starts an HTTP server and fires
// events when incoming requests match the configured path and method filters.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	sdk "github.com/directedbits/recur/pkg/plugin-sdk"
)

// pluginInput is the JSON payload read from stdin.
type pluginInput struct {
	TriggerType string         `json:"trigger_type"`
	Options     map[string]any `json:"options"`
	Config      map[string]any `json:"config"`
}

// parsedWebhookInput holds the validated fields extracted from a pluginInput.
type parsedWebhookInput struct {
	Input           *pluginInput
	Port            string
	Path            string
	Method          string
	MaxBodySize     int64
	Secret          string
	SignatureHeader string
	TLSCert         string
	TLSKey          string
	RateLimit       int
	RetryAfter      int
}

// parseInput reads stdin JSON and validates the webhook trigger configuration.
func parseInput(r io.Reader) (*parsedWebhookInput, error) {
	var input pluginInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}

	if input.TriggerType != "WebhookReceived" {
		return nil, fmt.Errorf("unsupported trigger_type: %s", input.TriggerType)
	}

	port := optString(input.Options, "port", "")
	if port == "" {
		return nil, fmt.Errorf("port option is required")
	}
	path := optString(input.Options, "path", "/")
	method := optString(input.Options, "method", "all")

	maxBodySizeStr := optString(input.Options, "max_body_size", "1048576")
	maxBodySize, err := strconv.ParseInt(maxBodySizeStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid max_body_size %q: %w", maxBodySizeStr, err)
	}

	rateLimitStr := optString(input.Options, "rate_limit", "")
	if rateLimitStr == "" {
		rateLimitStr = optString(input.Config, "rate_limit", "0")
	}
	rateLimit, err := strconv.Atoi(rateLimitStr)
	if err != nil {
		return nil, fmt.Errorf("invalid rate_limit %q: %w", rateLimitStr, err)
	}

	retryAfterStr := optString(input.Options, "retry_after", "")
	if retryAfterStr == "" {
		retryAfterStr = optString(input.Config, "retry_after", "1")
	}
	retryAfter, err := strconv.Atoi(retryAfterStr)
	if err != nil {
		return nil, fmt.Errorf("invalid retry_after %q: %w", retryAfterStr, err)
	}

	return &parsedWebhookInput{
		Input:           &input,
		Port:            port,
		Path:            path,
		Method:          method,
		MaxBodySize:     maxBodySize,
		Secret:          optString(input.Options, "secret", ""),
		SignatureHeader: optString(input.Options, "signature_header", "X-Hub-Signature-256"),
		TLSCert:         optString(input.Options, "tls_cert", ""),
		TLSKey:          optString(input.Options, "tls_key", ""),
		RateLimit:       rateLimit,
		RetryAfter:      retryAfter,
	}, nil
}

func main() {
	log.SetPrefix("webhook: ")
	log.SetFlags(0)

	parsed, err := parseInput(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	// Read required env vars
	socketPath := os.Getenv("RECUR_SOCKET")
	triggerID := os.Getenv("RECUR_TRIGGER_ID")
	if socketPath == "" || triggerID == "" {
		log.Fatal("RECUR_SOCKET and RECUR_TRIGGER_ID must be set")
	}

	port := parsed.Port
	path := parsed.Path
	method := parsed.Method
	maxBodySize := parsed.MaxBodySize
	secret := parsed.Secret
	signatureHeader := parsed.SignatureHeader
	tlsCert := parsed.TLSCert
	tlsKey := parsed.TLSKey
	rateLimit := parsed.RateLimit
	retryAfter := parsed.RetryAfter

	// Start HTTP/HTTPS server
	server, err := StartServer(port, path, method, maxBodySize, secret, signatureHeader, tlsCert, tlsKey, rateLimit, retryAfter)
	if err != nil {
		log.Fatalf("starting server: %v", err)
	}
	defer server.Stop()

	proto := "http"
	if tlsCert != "" && tlsKey != "" {
		proto = "https"
	}
	log.Printf("started: %s port=%s path=%s method=%s max_body_size=%d", proto, port, path, method, maxBodySize)

	// Connect to daemon gRPC socket
	client, err := sdk.Connect(socketPath)
	if err != nil {
		log.Fatalf("connecting to daemon: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Set up signal handler for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Event loop
	for {
		select {
		case evt, ok := <-server.Events():
			if !ok {
				log.Print("event channel closed, exiting")
				return
			}

			ctxVars := map[string]string{
				"RequestMethod": evt.Method,
				"RequestPath":   evt.Path,
				"RequestBody":   evt.Body,
				"QueryString":   evt.QueryString,
				"RemoteAddr":    evt.RemoteAddr,
				"ContentType":   evt.ContentType,
				"Headers":       encodeHeaders(evt.Headers),
				"UserAgent":     evt.UserAgent,
				"Referer":       evt.Referer,
				"XForwardedFor": evt.XForwardedFor,
			}

			resp, err := client.Service.ReportTriggerEvent(context.Background(), &sdk.ReportTriggerEventRequest{
				TriggerId: triggerID,
				Context:   ctxVars,
			})
			if err != nil {
				log.Printf("reporting event: %v", err)
				continue
			}
			if !resp.Accepted {
				log.Printf("event rejected: %s", resp.Error)
				continue
			}

			log.Printf("event reported: %s %s", evt.Method, evt.Path)

		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "received %v, shutting down\n", sig)
			return
		}
	}
}

// optString extracts a string option with a default fallback.
func optString(opts map[string]any, key, fallback string) string {
	if v, ok := opts[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
