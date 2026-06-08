package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func TestParseBrokerURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"localhost:1883", "tcp://localhost:1883"},
		{"tcp://localhost:1883", "tcp://localhost:1883"},
		{"ssl://broker.example.com:8883", "ssl://broker.example.com:8883"},
		{"tls://broker.example.com:8883", "tls://broker.example.com:8883"},
		{"TCP://localhost:1883", "TCP://localhost:1883"},
		{"ws://localhost:8080", "ws://localhost:8080"},
		{"wss://localhost:8080", "wss://localhost:8080"},
		{"192.168.1.1:1883", "tcp://192.168.1.1:1883"},
		{"", ""},
	}

	for _, tt := range tests {
		got := ParseBrokerURL(tt.input)
		if got != tt.want {
			t.Errorf("ParseBrokerURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildClientConfig(t *testing.T) {
	t.Run("basic options", func(t *testing.T) {
		opts := map[string]any{
			"broker": "localhost:1883",
			"topic":  "test/topic",
			"qos":    "1",
		}
		cfg, err := BuildClientConfig(opts, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Broker != "tcp://localhost:1883" {
			t.Errorf("broker = %q, want %q", cfg.Broker, "tcp://localhost:1883")
		}
		if cfg.Topic != "test/topic" {
			t.Errorf("topic = %q, want %q", cfg.Topic, "test/topic")
		}
		if cfg.QoS != 1 {
			t.Errorf("qos = %d, want 1", cfg.QoS)
		}
		if cfg.ClientID == "" {
			t.Error("expected auto-generated client ID")
		}
		if cfg.CleanSession != true {
			t.Error("expected clean_session = true by default")
		}
	})

	t.Run("broker required", func(t *testing.T) {
		_, err := BuildClientConfig(map[string]any{}, nil)
		if err == nil {
			t.Fatal("expected error for missing broker")
		}
	})

	t.Run("invalid qos", func(t *testing.T) {
		_, err := BuildClientConfig(map[string]any{
			"broker": "localhost:1883",
			"qos":    "3",
		}, nil)
		if err == nil {
			t.Fatal("expected error for invalid qos")
		}
	})

	t.Run("auth username+password", func(t *testing.T) {
		opts := map[string]any{
			"broker":   "localhost:1883",
			"username": "user",
			"password": "pass",
		}
		cfg, err := BuildClientConfig(opts, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Username != "user" {
			t.Errorf("username = %q, want %q", cfg.Username, "user")
		}
		if cfg.Password != "pass" {
			t.Errorf("password = %q, want %q", cfg.Password, "pass")
		}
	})

	t.Run("auth token only", func(t *testing.T) {
		opts := map[string]any{
			"broker":   "localhost:1883",
			"password": "my-token",
		}
		cfg, err := BuildClientConfig(opts, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Username != "" {
			t.Errorf("username = %q, want empty", cfg.Username)
		}
		if cfg.Password != "my-token" {
			t.Errorf("password = %q, want %q", cfg.Password, "my-token")
		}
	})

	t.Run("config fallback for broker", func(t *testing.T) {
		opts := map[string]any{"topic": "test"}
		config := map[string]any{"broker": "config-broker:1883"}
		cfg, err := BuildClientConfig(opts, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Broker != "tcp://config-broker:1883" {
			t.Errorf("broker = %q, want %q", cfg.Broker, "tcp://config-broker:1883")
		}
	})

	t.Run("config fallback for credentials", func(t *testing.T) {
		opts := map[string]any{"broker": "localhost:1883"}
		config := map[string]any{
			"username": "cfg-user",
			"password": "cfg-pass",
		}
		cfg, err := BuildClientConfig(opts, config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Username != "cfg-user" {
			t.Errorf("username = %q, want %q", cfg.Username, "cfg-user")
		}
	})

	t.Run("clean_session false", func(t *testing.T) {
		opts := map[string]any{
			"broker":        "localhost:1883",
			"clean_session": "false",
		}
		cfg, err := BuildClientConfig(opts, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.CleanSession != false {
			t.Error("expected clean_session = false")
		}
	})

	t.Run("custom keepalive", func(t *testing.T) {
		opts := map[string]any{
			"broker":    "localhost:1883",
			"keepalive": "60",
		}
		cfg, err := BuildClientConfig(opts, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.KeepAlive.Seconds() != 60 {
			t.Errorf("keepalive = %v, want 60s", cfg.KeepAlive)
		}
	})

	t.Run("explicit client_id", func(t *testing.T) {
		opts := map[string]any{
			"broker":    "localhost:1883",
			"client_id": "my-client",
		}
		cfg, err := BuildClientConfig(opts, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientID != "my-client" {
			t.Errorf("client_id = %q, want %q", cfg.ClientID, "my-client")
		}
	})
}

func TestModeDispatch(t *testing.T) {
	t.Run("trigger mode detection", func(t *testing.T) {
		input := pluginInput{TriggerType: "MessageReceived"}
		if input.TriggerType == "" {
			t.Error("expected trigger_type to be set")
		}
		if input.ActionType != "" {
			t.Error("expected action_type to be empty")
		}
	})

	t.Run("action mode detection", func(t *testing.T) {
		input := pluginInput{ActionType: "Publish"}
		if input.ActionType == "" {
			t.Error("expected action_type to be set")
		}
		if input.TriggerType != "" {
			t.Error("expected trigger_type to be empty")
		}
	})
}

func TestActionOutputJSON(t *testing.T) {
	out := actionOutput{
		Success: true,
		Output:  "published to test/topic",
		Error:   "",
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed actionOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !parsed.Success {
		t.Error("expected success = true")
	}
	if parsed.Output != "published to test/topic" {
		t.Errorf("output = %q, want %q", parsed.Output, "published to test/topic")
	}
	if parsed.Error != "" {
		t.Errorf("error = %q, want empty", parsed.Error)
	}
}

func TestActionOutputErrorJSON(t *testing.T) {
	out := actionOutput{
		Success: false,
		Error:   "connection refused",
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed actionOutput
	json.Unmarshal(data, &parsed)

	if parsed.Success {
		t.Error("expected success = false")
	}
	if parsed.Error != "connection refused" {
		t.Errorf("error = %q, want %q", parsed.Error, "connection refused")
	}
}

func TestPluginInputJSON(t *testing.T) {
	// Verify the JSON format matches what the daemon sends
	jsonStr := `{
		"action_type": "Publish",
		"options": {"broker": "localhost:1883", "topic": "test", "payload": "hello"},
		"config": {"username": "admin"},
		"test": true
	}`

	var input pluginInput
	if err := json.Unmarshal([]byte(jsonStr), &input); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if input.ActionType != "Publish" {
		t.Errorf("action_type = %q, want %q", input.ActionType, "Publish")
	}
	if input.TriggerType != "" {
		t.Errorf("trigger_type = %q, want empty", input.TriggerType)
	}
	if !input.Test {
		t.Error("expected test = true")
	}
	if input.Options["broker"] != "localhost:1883" {
		t.Errorf("options.broker = %v, want %q", input.Options["broker"], "localhost:1883")
	}
}

func TestGenerateClientID(t *testing.T) {
	id1 := generateClientID()
	id2 := generateClientID()

	if id1 == id2 {
		t.Error("expected unique client IDs")
	}
	if len(id1) != 14 { // "recur-" (6) + 8 hex chars
		t.Errorf("client ID length = %d, want 14", len(id1))
	}
	if id1[:6] != "recur-" {
		t.Errorf("client ID prefix = %q, want %q", id1[:6], "recur-")
	}
}

func TestOptStr(t *testing.T) {
	m := map[string]any{
		"key1": "value1",
		"key2": "",
		"key3": 42,
	}

	if got := optStr(m, "key1", "default"); got != "value1" {
		t.Errorf("key1 = %q, want %q", got, "value1")
	}
	if got := optStr(m, "key2", "default"); got != "default" {
		t.Errorf("key2 (empty) = %q, want %q", got, "default")
	}
	if got := optStr(m, "key3", "default"); got != "default" {
		t.Errorf("key3 (int) = %q, want %q", got, "default")
	}
	if got := optStr(m, "missing", "default"); got != "default" {
		t.Errorf("missing = %q, want %q", got, "default")
	}
	if got := optStr(nil, "key", "default"); got != "default" {
		t.Errorf("nil map = %q, want %q", got, "default")
	}
}

func TestSubscribe_Success(t *testing.T) {
	mock := &mockClient{}

	// Override factory
	original := clientFactory
	clientFactory = func(opts *mqtt.ClientOptions) MQTTClient {
		return mock
	}
	defer func() { clientFactory = original }()

	cfg := &ClientConfig{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Topic:    "test/topic",
		QoS:      0,
	}

	messages, cleanup, err := Subscribe(cfg)
	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}
	defer cleanup()

	if messages == nil {
		t.Fatal("messages channel should not be nil")
	}

	// Verify subscribe was called
	if len(mock.subscribed) != 1 {
		t.Fatalf("expected 1 subscribe call, got %d", len(mock.subscribed))
	}
	if mock.subscribed[0].topic != "test/topic" {
		t.Errorf("subscribed topic = %q, want %q", mock.subscribed[0].topic, "test/topic")
	}
}

func TestSubscribe_ConnectError(t *testing.T) {
	original := clientFactory
	clientFactory = func(opts *mqtt.ClientOptions) MQTTClient {
		return &mockClient{connectErr: fmt.Errorf("connection refused")}
	}
	defer func() { clientFactory = original }()

	cfg := &ClientConfig{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Topic:    "test/topic",
	}

	_, _, err := Subscribe(cfg)
	if err == nil {
		t.Fatal("expected error for connect failure")
	}
	if !strings.Contains(err.Error(), "connecting") {
		t.Errorf("error = %q, want to contain 'connecting'", err.Error())
	}
}

func TestSubscribe_SubscribeError(t *testing.T) {
	original := clientFactory
	clientFactory = func(opts *mqtt.ClientOptions) MQTTClient {
		return &mockClient{subscribeErr: fmt.Errorf("not authorized")}
	}
	defer func() { clientFactory = original }()

	cfg := &ClientConfig{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Topic:    "test/topic",
	}

	_, _, err := Subscribe(cfg)
	if err == nil {
		t.Fatal("expected error for subscribe failure")
	}
	if !strings.Contains(err.Error(), "subscribing") {
		t.Errorf("error = %q, want to contain 'subscribing'", err.Error())
	}
}

func TestPublish_Success(t *testing.T) {
	mock := &mockClient{}
	original := clientFactory
	clientFactory = func(opts *mqtt.ClientOptions) MQTTClient {
		return mock
	}
	defer func() { clientFactory = original }()

	cfg := &ClientConfig{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Topic:    "test/topic",
		QoS:      1,
	}

	err := Publish(cfg, "hello world", true)
	if err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	if len(mock.published) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(mock.published))
	}
	if mock.published[0].topic != "test/topic" {
		t.Errorf("topic = %q", mock.published[0].topic)
	}
	if mock.published[0].payload != "hello world" {
		t.Errorf("payload = %q", mock.published[0].payload)
	}
	if !mock.published[0].retained {
		t.Error("expected retained = true")
	}
}

func TestPublish_ConnectError(t *testing.T) {
	original := clientFactory
	clientFactory = func(opts *mqtt.ClientOptions) MQTTClient {
		return &mockClient{connectErr: fmt.Errorf("connection refused")}
	}
	defer func() { clientFactory = original }()

	cfg := &ClientConfig{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Topic:    "test/topic",
	}

	err := Publish(cfg, "hello", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPublish_PublishError(t *testing.T) {
	original := clientFactory
	clientFactory = func(opts *mqtt.ClientOptions) MQTTClient {
		return &mockClient{publishErr: fmt.Errorf("publish failed")}
	}
	defer func() { clientFactory = original }()

	cfg := &ClientConfig{
		Broker:   "tcp://localhost:1883",
		ClientID: "test-client",
		Topic:    "test/topic",
	}

	err := Publish(cfg, "hello", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSubscribe_WithAuth(t *testing.T) {
	mock := &mockClient{}
	original := clientFactory
	clientFactory = func(opts *mqtt.ClientOptions) MQTTClient {
		return mock
	}
	defer func() { clientFactory = original }()

	cfg := &ClientConfig{
		Broker:       "tcp://localhost:1883",
		ClientID:     "test-client",
		Topic:        "test/topic",
		Username:     "user",
		Password:     "pass",
		CleanSession: false,
		KeepAlive:    60 * time.Second,
	}

	_, cleanup, err := Subscribe(cfg)
	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}
	cleanup()
}
