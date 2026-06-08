package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// freePort asks the OS for an available port.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("finding free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return fmt.Sprintf("%d", port)
}

func startTestServer(t *testing.T, path, method string, maxBodySize int64) (*Server, string) {
	t.Helper()
	port := freePort(t)
	srv, err := StartServer(port, path, method, maxBodySize, "", "", "", "", 0, 1)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	t.Cleanup(func() { srv.Stop() })
	return srv, fmt.Sprintf("http://127.0.0.1:%s", port)
}

func startTestServerHMAC(t *testing.T, path, method string, secret, sigHeader string) (*Server, string) {
	t.Helper()
	port := freePort(t)
	srv, err := StartServer(port, path, method, 1048576, secret, sigHeader, "", "", 0, 1)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	t.Cleanup(func() { srv.Stop() })
	return srv, fmt.Sprintf("http://127.0.0.1:%s", port)
}

func startTestServerWithRateLimit(t *testing.T, path, method string, maxBodySize int64, rateLimit, retryAfter int) (*Server, string) {
	t.Helper()
	port := freePort(t)
	srv, err := StartServer(port, path, method, maxBodySize, "", "", "", "", rateLimit, retryAfter)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	t.Cleanup(func() { srv.Stop() })
	return srv, fmt.Sprintf("http://127.0.0.1:%s", port)
}

func TestMatchingRequest(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 1048576)

	resp, err := http.Post(base+"/hook", "application/json", strings.NewReader(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "accepted" {
		t.Fatalf("expected accepted, got %v", body)
	}

	select {
	case evt := <-srv.Events():
		if evt.Method != "POST" {
			t.Errorf("Method = %q, want POST", evt.Method)
		}
		if evt.Path != "/hook" {
			t.Errorf("Path = %q, want /hook", evt.Path)
		}
		if evt.Body != `{"hello":"world"}` {
			t.Errorf("Body = %q, want {\"hello\":\"world\"}", evt.Body)
		}
		if evt.ContentType != "application/json" {
			t.Errorf("ContentType = %q, want application/json", evt.ContentType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestPathMismatch(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 1048576)

	resp, err := http.Post(base+"/wrong", "text/plain", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	// No event should be emitted.
	select {
	case evt := <-srv.Events():
		t.Fatalf("unexpected event: %+v", evt)
	case <-time.After(100 * time.Millisecond):
		// ok
	}
}

func TestMethodMismatch(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 1048576)

	req, _ := http.NewRequest("GET", base+"/hook", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 405 {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		t.Fatalf("unexpected event: %+v", evt)
	case <-time.After(100 * time.Millisecond):
		// ok
	}
}

func TestMethodAll(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "all", 1048576)

	// GET should be accepted.
	req, _ := http.NewRequest("GET", base+"/hook", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("GET expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Method != "GET" {
			t.Errorf("Method = %q, want GET", evt.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	// PUT should also be accepted.
	req, _ = http.NewRequest("PUT", base+"/hook", strings.NewReader("data"))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("PUT expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Method != "PUT" {
			t.Errorf("Method = %q, want PUT", evt.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBodyTooLarge(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 10) // 10 bytes max

	body := strings.Repeat("x", 11) // 11 bytes
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 413 {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		t.Fatalf("unexpected event: %+v", evt)
	case <-time.After(100 * time.Millisecond):
		// ok
	}
}

func TestBodyWithinLimit(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 10)

	body := strings.Repeat("x", 10) // exactly at limit
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Body != body {
			t.Errorf("Body = %q, want %q", evt.Body, body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestQueryString(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "all", 1048576)

	resp, err := http.Get(base + "/hook?foo=bar&baz=1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	select {
	case evt := <-srv.Events():
		if evt.QueryString != "foo=bar&baz=1" {
			t.Errorf("QueryString = %q, want foo=bar&baz=1", evt.QueryString)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHeadersCaptured(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "all", 1048576)

	req, _ := http.NewRequest("GET", base+"/hook", nil)
	req.Header.Set("X-Custom", "test-value")
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Referer", "http://example.com")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	select {
	case evt := <-srv.Events():
		if evt.Headers["X-Custom"] != "test-value" {
			t.Errorf("Headers[X-Custom] = %q, want test-value", evt.Headers["X-Custom"])
		}
		if evt.UserAgent != "test-agent" {
			t.Errorf("UserAgent = %q, want test-agent", evt.UserAgent)
		}
		if evt.Referer != "http://example.com" {
			t.Errorf("Referer = %q, want http://example.com", evt.Referer)
		}
		if evt.XForwardedFor != "10.0.0.1" {
			t.Errorf("XForwardedFor = %q, want 10.0.0.1", evt.XForwardedFor)
		}

		// Verify encodeHeaders produces valid JSON.
		encoded := encodeHeaders(evt.Headers)
		var decoded map[string]string
		if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
			t.Errorf("encodeHeaders produced invalid JSON: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestRemoteAddrAndContentType(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 1048576)

	resp, err := http.Post(base+"/hook", "application/xml", strings.NewReader("<data/>"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	select {
	case evt := <-srv.Events():
		if evt.RemoteAddr == "" {
			t.Error("RemoteAddr should not be empty")
		}
		if evt.ContentType != "application/xml" {
			t.Errorf("ContentType = %q, want application/xml", evt.ContentType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestStartServerInvalidPort(t *testing.T) {
	_, err := StartServer("99999", "/", "all", 1048576, "", "", "", "", 0, 1)
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestStartServerEmptyPort(t *testing.T) {
	_, err := StartServer("", "/", "all", 1048576, "", "", "", "", 0, 1)
	if err == nil {
		t.Fatal("expected error for empty port")
	}
}

func TestServerStopClean(t *testing.T) {
	port := freePort(t)
	srv, err := StartServer(port, "/", "all", 1048576, "", "", "", "", 0, 1)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	// Stop should not hang or panic.
	srv.Stop()

	// After stop, the port should be released. Verify by listening again.
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatalf("port not released after Stop: %v", err)
	}
	ln.Close()
}

// --- HMAC signature tests ---

func TestHMACValidSignature(t *testing.T) {
	secret := "test-secret-key"
	srv, base := startTestServerHMAC(t, "/hook", "POST", secret, "X-Hub-Signature-256")

	body := `{"event":"push"}`
	sig := computeSignature([]byte(body), secret)

	req, _ := http.NewRequest("POST", base+"/hook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Body != body {
			t.Errorf("Body = %q, want %q", evt.Body, body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHMACInvalidSignature(t *testing.T) {
	srv, base := startTestServerHMAC(t, "/hook", "POST", "correct-secret", "X-Hub-Signature-256")

	body := `{"event":"push"}`
	sig := computeSignature([]byte(body), "wrong-secret")

	req, _ := http.NewRequest("POST", base+"/hook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		t.Fatalf("unexpected event: %+v", evt)
	case <-time.After(100 * time.Millisecond):
		// ok
	}
}

func TestHMACMissingSignature(t *testing.T) {
	srv, base := startTestServerHMAC(t, "/hook", "POST", "my-secret", "X-Hub-Signature-256")

	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		t.Fatalf("unexpected event: %+v", evt)
	case <-time.After(100 * time.Millisecond):
		// ok
	}
}

func TestHMACNoSecretSkipsValidation(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 1048576) // no secret

	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 (no secret = no validation), got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Body != "body" {
			t.Errorf("Body = %q", evt.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHMACCustomHeader(t *testing.T) {
	secret := "custom-secret"
	srv, base := startTestServerHMAC(t, "/hook", "POST", secret, "X-My-Signature")

	body := `test payload`
	sig := computeSignature([]byte(body), secret)

	req, _ := http.NewRequest("POST", base+"/hook", strings.NewReader(body))
	req.Header.Set("X-My-Signature", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Body != body {
			t.Errorf("Body = %q", evt.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHMACSignatureWithoutPrefix(t *testing.T) {
	secret := "test-secret"
	srv, base := startTestServerHMAC(t, "/hook", "POST", secret, "X-Hub-Signature-256")

	body := `raw body`
	// Compute signature without "sha256=" prefix.
	sig := computeSignature([]byte(body), secret)
	sigWithoutPrefix := strings.TrimPrefix(sig, "sha256=")

	req, _ := http.NewRequest("POST", base+"/hook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sigWithoutPrefix)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 (sig without prefix should work), got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Body != body {
			t.Errorf("Body = %q", evt.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

// --- TLS tests ---

// generateTestCert creates a self-signed cert and key in the given directory.
func generateTestCert(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(1 * time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating certificate: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	certFile, _ := os.Create(certPath)
	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certFile.Close()

	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPath = filepath.Join(dir, "key.pem")
	keyFile, _ := os.Create(keyPath)
	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyFile.Close()

	return certPath, keyPath
}

func TestTLSServer(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir)

	port := freePort(t)
	srv, err := StartServer(port, "/hook", "POST", 1048576, "", "", certPath, keyPath, 0, 1)
	if err != nil {
		t.Fatalf("StartServer with TLS: %v", err)
	}
	t.Cleanup(func() { srv.Stop() })

	// Allow time for TLS listener to start
	time.Sleep(50 * time.Millisecond)

	// Use a client that trusts the self-signed cert
	certPEM, _ := os.ReadFile(certPath)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}

	base := fmt.Sprintf("https://127.0.0.1:%s", port)
	resp, err := client.Post(base+"/hook", "text/plain", strings.NewReader("tls-body"))
	if err != nil {
		t.Fatalf("HTTPS POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Body != "tls-body" {
			t.Errorf("Body = %q, want tls-body", evt.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestPathRoot(t *testing.T) {
	srv, base := startTestServer(t, "/", "all", 1048576)

	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Path != "/" {
			t.Errorf("Path = %q, want /", evt.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestEmptyBody(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 1048576)

	resp, err := http.Post(base+"/hook", "", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Body != "" {
			t.Errorf("Body = %q, want empty", evt.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestDeleteMethod(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "DELETE", 1048576)

	req, _ := http.NewRequest("DELETE", base+"/hook", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Method != "DELETE" {
			t.Errorf("Method = %q, want DELETE", evt.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestZeroBodyLimit(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 0)

	// Empty body should still work with 0 limit
	resp, err := http.Post(base+"/hook", "", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Body != "" {
			t.Errorf("Body = %q, want empty", evt.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	// Any body should be rejected with 0 limit
	resp, err = http.Post(base+"/hook", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 413 {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
}

func TestVerifySignature(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		secret    string
		signature string
		want      bool
	}{
		{"valid with prefix", "hello", "secret", computeSignature([]byte("hello"), "secret"), true},
		{"valid without prefix", "hello", "secret", strings.TrimPrefix(computeSignature([]byte("hello"), "secret"), "sha256="), true},
		{"wrong secret", "hello", "secret", computeSignature([]byte("hello"), "wrong"), false},
		{"invalid hex", "hello", "secret", "sha256=notvalidhex", false},
		{"empty signature", "hello", "secret", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := verifySignature([]byte(tc.body), tc.secret, tc.signature)
			if got != tc.want {
				t.Errorf("verifySignature() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestComputeSignature(t *testing.T) {
	sig := computeSignature([]byte("test"), "secret")
	if !strings.HasPrefix(sig, "sha256=") {
		t.Errorf("signature should start with sha256=, got %q", sig)
	}
	// Same input should produce same output
	sig2 := computeSignature([]byte("test"), "secret")
	if sig != sig2 {
		t.Error("same input should produce same signature")
	}
	// Different input should produce different output
	sig3 := computeSignature([]byte("other"), "secret")
	if sig == sig3 {
		t.Error("different input should produce different signature")
	}
}

func TestEncodeHeaders_Empty(t *testing.T) {
	result := encodeHeaders(map[string]string{})
	if result != "{}" {
		t.Errorf("encodeHeaders({}) = %q, want {}", result)
	}
}

func TestEncodeHeaders_WithValues(t *testing.T) {
	headers := map[string]string{"X-Custom": "value"}
	result := encodeHeaders(headers)
	var decoded map[string]string
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded["X-Custom"] != "value" {
		t.Errorf("X-Custom = %q", decoded["X-Custom"])
	}
}

func TestMultipleConcurrentRequests(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 1048576)

	// Send 5 requests in parallel
	for i := 0; i < 5; i++ {
		go func(n int) {
			body := fmt.Sprintf(`{"n":%d}`, n)
			http.Post(base+"/hook", "application/json", strings.NewReader(body))
		}(i)
	}

	// Collect all 5 events
	received := 0
	timeout := time.After(3 * time.Second)
	for received < 5 {
		select {
		case <-srv.Events():
			received++
		case <-timeout:
			t.Fatalf("timed out after receiving %d of 5 events", received)
		}
	}
}

func TestPatchMethod(t *testing.T) {
	srv, base := startTestServer(t, "/api", "PATCH", 1048576)

	req, _ := http.NewRequest("PATCH", base+"/api", strings.NewReader(`{"update":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if evt.Method != "PATCH" {
			t.Errorf("Method = %q, want PATCH", evt.Method)
		}
		if evt.Body != `{"update":true}` {
			t.Errorf("Body = %q", evt.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestNoQueryString(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "all", 1048576)

	resp, err := http.Get(base + "/hook")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	select {
	case evt := <-srv.Events():
		if evt.QueryString != "" {
			t.Errorf("QueryString = %q, want empty", evt.QueryString)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestLargeBody(t *testing.T) {
	srv, base := startTestServer(t, "/hook", "POST", 1048576)

	// 100KB body should be fine with 1MB limit
	body := strings.Repeat("x", 100*1024)
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case evt := <-srv.Events():
		if len(evt.Body) != 100*1024 {
			t.Errorf("Body length = %d, want %d", len(evt.Body), 100*1024)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestOptString(t *testing.T) {
	opts := map[string]any{
		"port":   "8080",
		"empty":  "",
		"number": 42,
	}

	if got := optString(opts, "port", "default"); got != "8080" {
		t.Errorf("got %q, want %q", got, "8080")
	}
	if got := optString(opts, "missing", "default"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
	if got := optString(opts, "empty", "default"); got != "default" {
		t.Errorf("got %q for empty string, want fallback %q", got, "default")
	}
	if got := optString(opts, "number", "default"); got != "default" {
		t.Errorf("got %q for non-string, want fallback %q", got, "default")
	}
}

func TestPluginInputParsing(t *testing.T) {
	jsonStr := `{
		"trigger_type": "WebhookReceived",
		"options": {"port": "9090", "path": "/hook", "method": "POST"},
		"config": {}
	}`

	var input pluginInput
	if err := json.NewDecoder(strings.NewReader(jsonStr)).Decode(&input); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if input.TriggerType != "WebhookReceived" {
		t.Errorf("TriggerType = %q, want %q", input.TriggerType, "WebhookReceived")
	}
	if p, ok := input.Options["port"].(string); !ok || p != "9090" {
		t.Errorf("Options[port] = %v, want %q", input.Options["port"], "9090")
	}
	if m, ok := input.Options["method"].(string); !ok || m != "POST" {
		t.Errorf("Options[method] = %v, want %q", input.Options["method"], "POST")
	}
}

func TestTLSServerRejectsPlainHTTP(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateTestCert(t, dir)

	port := freePort(t)
	srv, err := StartServer(port, "/hook", "POST", 1048576, "", "", certPath, keyPath, 0, 1)
	if err != nil {
		t.Fatalf("StartServer with TLS: %v", err)
	}
	t.Cleanup(func() { srv.Stop() })

	time.Sleep(50 * time.Millisecond)

	// Plain HTTP to HTTPS server should either fail or return a non-200 response.
	// The Go HTTP server responds with a 400 "Client sent an HTTP request to an HTTPS server".
	base := fmt.Sprintf("http://127.0.0.1:%s", port)
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		return // connection error is acceptable
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("plain HTTP should not get 200 from TLS server")
	}
}

// --- Rate limit tests ---

func TestRateLimitExceeded(t *testing.T) {
	srv, base := startTestServerWithRateLimit(t, "/hook", "POST", 1048576, 2, 1)

	// First 2 requests should succeed (burst = 2).
	for i := 0; i < 2; i++ {
		resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
		// Drain event
		<-srv.Events()
	}

	// Next request should be rate limited.
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("rate-limited request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") != "1" {
		t.Errorf("Retry-After = %q, want 1", resp.Header.Get("Retry-After"))
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "rate limit exceeded" {
		t.Errorf("error = %q, want 'rate limit exceeded'", body["error"])
	}
}

func TestRateLimitZeroDisabled(t *testing.T) {
	srv, base := startTestServerWithRateLimit(t, "/hook", "POST", 1048576, 0, 1)

	// With rate limit disabled, many rapid requests should all succeed.
	for i := 0; i < 20; i++ {
		resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
		// Drain event
		<-srv.Events()
	}
}

func TestChannelFullReturns429(t *testing.T) {
	// Create server with no rate limit; channel buffer is 64.
	_, base := startTestServerWithRateLimit(t, "/hook", "POST", 1048576, 0, 3)

	// Fill the channel buffer (64 events) without draining.
	for i := 0; i < 64; i++ {
		resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
	}

	// Next request should get 429 (channel full).
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("overflow request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "server busy" {
		t.Errorf("error = %q, want 'server busy'", body["error"])
	}
	if resp.Header.Get("Retry-After") != "3" {
		t.Errorf("Retry-After = %q, want 3", resp.Header.Get("Retry-After"))
	}
}

func TestRetryAfterHeader(t *testing.T) {
	srv, base := startTestServerWithRateLimit(t, "/hook", "POST", 1048576, 1, 5)

	// Use the one token in the burst.
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	resp.Body.Close()
	<-srv.Events()

	// Second request should be rate limited with Retry-After: 5
	resp, err = http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") != "5" {
		t.Errorf("Retry-After = %q, want 5", resp.Header.Get("Retry-After"))
	}
}

func TestRateLimitDifferentRetryAfter(t *testing.T) {
	srv, base := startTestServerWithRateLimit(t, "/hook", "POST", 1048576, 1, 10)

	// Use the one token.
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	resp.Body.Close()
	<-srv.Events()

	// Exceed rate limit.
	resp, err = http.Post(base+"/hook", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") != "10" {
		t.Errorf("Retry-After = %q, want 10", resp.Header.Get("Retry-After"))
	}
}

func TestChannelFull429Body(t *testing.T) {
	// Verify that channel-full returns "server busy" (not "rate limit exceeded").
	_, base := startTestServerWithRateLimit(t, "/hook", "POST", 1048576, 0, 1)

	// Fill the buffer.
	for i := 0; i < 64; i++ {
		resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("x"))
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
	}

	// Overflow.
	resp, err := http.Post(base+"/hook", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatalf("overflow: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "server busy" {
		t.Errorf("error = %q, want 'server busy'", body["error"])
	}
}
