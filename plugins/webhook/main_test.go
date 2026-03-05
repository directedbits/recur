package main

import (
	"strings"
	"testing"
)

func TestParseInput_Valid(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"8080","path":"/hook","method":"POST","max_body_size":"2048","secret":"s3cret","signature_header":"X-Sig"},"config":{}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Input.TriggerType != "WebhookReceived" {
		t.Errorf("TriggerType = %q", parsed.Input.TriggerType)
	}
	if parsed.Port != "8080" {
		t.Errorf("Port = %q", parsed.Port)
	}
	if parsed.Path != "/hook" {
		t.Errorf("Path = %q", parsed.Path)
	}
	if parsed.Method != "POST" {
		t.Errorf("Method = %q", parsed.Method)
	}
	if parsed.MaxBodySize != 2048 {
		t.Errorf("MaxBodySize = %d", parsed.MaxBodySize)
	}
	if parsed.Secret != "s3cret" {
		t.Errorf("Secret = %q", parsed.Secret)
	}
	if parsed.SignatureHeader != "X-Sig" {
		t.Errorf("SignatureHeader = %q", parsed.SignatureHeader)
	}
}

func TestParseInput_Defaults(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"9090"},"config":{}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Path != "/" {
		t.Errorf("Path = %q, want /", parsed.Path)
	}
	if parsed.Method != "all" {
		t.Errorf("Method = %q, want all", parsed.Method)
	}
	if parsed.MaxBodySize != 1048576 {
		t.Errorf("MaxBodySize = %d, want 1048576", parsed.MaxBodySize)
	}
	if parsed.SignatureHeader != "X-Hub-Signature-256" {
		t.Errorf("SignatureHeader = %q, want X-Hub-Signature-256", parsed.SignatureHeader)
	}
}

func TestParseInput_MissingPort(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for missing port")
	}
}

func TestParseInput_InvalidTriggerType(t *testing.T) {
	jsonStr := `{"trigger_type":"BadType","options":{"port":"8080"},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid trigger_type")
	}
}

func TestParseInput_InvalidMaxBodySize(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"8080","max_body_size":"bad"},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid max_body_size")
	}
}

func TestParseInput_InvalidJSON(t *testing.T) {
	_, err := parseInput(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseInput_RateLimitFromOptions(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"8080","rate_limit":"10","retry_after":"5"},"config":{}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.RateLimit != 10 {
		t.Errorf("RateLimit = %d, want 10", parsed.RateLimit)
	}
	if parsed.RetryAfter != 5 {
		t.Errorf("RetryAfter = %d, want 5", parsed.RetryAfter)
	}
}

func TestParseInput_RateLimitFromConfig(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"8080"},"config":{"rate_limit":"20","retry_after":"3"}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.RateLimit != 20 {
		t.Errorf("RateLimit = %d, want 20", parsed.RateLimit)
	}
	if parsed.RetryAfter != 3 {
		t.Errorf("RetryAfter = %d, want 3", parsed.RetryAfter)
	}
}

func TestParseInput_RateLimitDefaults(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"8080"},"config":{}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.RateLimit != 0 {
		t.Errorf("RateLimit = %d, want 0", parsed.RateLimit)
	}
	if parsed.RetryAfter != 1 {
		t.Errorf("RetryAfter = %d, want 1", parsed.RetryAfter)
	}
}

func TestParseInput_InvalidRateLimit(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"8080","rate_limit":"bad"},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid rate_limit")
	}
}

func TestParseInput_InvalidRetryAfter(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"8080","retry_after":"bad"},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid retry_after")
	}
}

func TestParseInput_OptionsOverrideConfig(t *testing.T) {
	jsonStr := `{"trigger_type":"WebhookReceived","options":{"port":"8080","rate_limit":"5"},"config":{"rate_limit":"100"}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.RateLimit != 5 {
		t.Errorf("RateLimit = %d, want 5 (options should override config)", parsed.RateLimit)
	}
}
