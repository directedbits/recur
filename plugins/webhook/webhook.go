package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// WebhookEvent represents a validated incoming HTTP request ready to report.
type WebhookEvent struct {
	Method        string
	Path          string
	Body          string
	QueryString   string
	RemoteAddr    string
	ContentType   string
	Headers       map[string]string
	UserAgent     string
	Referer       string
	XForwardedFor string
}

// Server listens for HTTP requests and emits WebhookEvents on match.
type Server struct {
	httpServer      *http.Server
	path            string
	method          string // "all" or specific like "POST"
	maxBodySize     int64
	secret          string // HMAC-SHA256 secret; empty disables verification
	signatureHeader string // header containing the HMAC signature
	limiter         *rate.Limiter // nil = no rate limit
	retryAfter      string        // value for Retry-After header
	events          chan WebhookEvent
	done            chan struct{}
}

// StartServer creates and starts an HTTP/HTTPS server. Events are sent to the returned
// channel. Call Stop() for graceful shutdown. If tlsCert and tlsKey are non-empty,
// the server uses TLS.
func StartServer(port, path, method string, maxBodySize int64, secret, signatureHeader, tlsCert, tlsKey string, rateLimit, retryAfter int) (*Server, error) {
	if port == "" {
		return nil, fmt.Errorf("port is required")
	}

	addr := net.JoinHostPort("", port)

	// Validate the address by attempting to listen.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	s := &Server{
		path:            path,
		method:          strings.ToUpper(method),
		maxBodySize:     maxBodySize,
		secret:          secret,
		signatureHeader: signatureHeader,
		retryAfter:      fmt.Sprintf("%d", retryAfter),
		events:          make(chan WebhookEvent, 64),
		done:            make(chan struct{}),
	}
	if rateLimit > 0 {
		s.limiter = rate.NewLimiter(rate.Limit(rateLimit), rateLimit)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handler)

	s.httpServer = &http.Server{
		Handler: mux,
	}

	go func() {
		var serveErr error
		if tlsCert != "" && tlsKey != "" {
			cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
			if err != nil {
				fmt.Printf("webhook: failed to load TLS cert/key: %v\n", err)
				close(s.done)
				return
			}
			tlsLn := tls.NewListener(ln, &tls.Config{
				Certificates: []tls.Certificate{cert},
			})
			serveErr = s.httpServer.Serve(tlsLn)
		} else {
			serveErr = s.httpServer.Serve(ln)
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			fmt.Printf("webhook: http server error: %v\n", serveErr)
		}
		close(s.done)
	}()

	return s, nil
}

// Stop performs a graceful shutdown with a 5-second deadline.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.httpServer.Shutdown(ctx)
	<-s.done
}

// Events returns the read-only event channel.
func (s *Server) Events() <-chan WebhookEvent {
	return s.events
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check path (exact match).
	if r.URL.Path != s.path {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
		return
	}

	// Check method.
	if s.method != "ALL" && r.Method != s.method {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"method not allowed"}`))
		return
	}

	// Check rate limit.
	if s.limiter != nil && !s.limiter.Allow() {
		w.Header().Set("Retry-After", s.retryAfter)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limit exceeded"}`))
		return
	}

	// Read body with size limit.
	limited := io.LimitReader(r.Body, s.maxBodySize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"reading body"}`))
		return
	}
	if int64(len(body)) > s.maxBodySize {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte(`{"error":"body too large"}`))
		return
	}

	// Verify HMAC signature if a secret is configured.
	if s.secret != "" {
		sig := r.Header.Get(s.signatureHeader)
		if sig == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"missing signature"}`))
			return
		}
		if !verifySignature(body, s.secret, sig) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid signature"}`))
			return
		}
	}

	// Flatten headers to single-value map.
	headers := make(map[string]string, len(r.Header))
	for k, vals := range r.Header {
		headers[k] = vals[0]
	}

	evt := WebhookEvent{
		Method:        r.Method,
		Path:          r.URL.Path,
		Body:          string(body),
		QueryString:   r.URL.RawQuery,
		RemoteAddr:    r.RemoteAddr,
		ContentType:   r.Header.Get("Content-Type"),
		Headers:       headers,
		UserAgent:     r.UserAgent(),
		Referer:       r.Referer(),
		XForwardedFor: r.Header.Get("X-Forwarded-For"),
	}

	// Non-blocking send — return 429 if channel is full.
	select {
	case s.events <- evt:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"accepted"}`))
	default:
		w.Header().Set("Retry-After", s.retryAfter)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"server busy"}`))
	}
}

// verifySignature checks an HMAC-SHA256 signature against the body.
// Accepts signatures with or without a "sha256=" prefix (GitHub-style).
func verifySignature(body []byte, secret, signature string) bool {
	signature = strings.TrimPrefix(signature, "sha256=")
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)

	return hmac.Equal(sigBytes, expected)
}

// computeSignature returns the HMAC-SHA256 hex digest with "sha256=" prefix.
func computeSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// encodeHeaders returns a JSON-encoded string of the header map.
func encodeHeaders(headers map[string]string) string {
	b, err := json.Marshal(headers)
	if err != nil {
		return "{}"
	}
	return string(b)
}
