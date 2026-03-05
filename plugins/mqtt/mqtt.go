package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Message represents a received MQTT message.
type Message struct {
	Topic     string
	Payload   string
	QoS       byte
	Retained  bool
	MessageID uint16
}

// ClientConfig holds parsed MQTT connection settings.
type ClientConfig struct {
	Broker       string
	ClientID     string
	Username     string
	Password     string
	CleanSession bool
	KeepAlive    time.Duration
	QoS          byte
	Topic        string
}

// ParseBrokerURL ensures the broker URL has a scheme.
// Defaults to tcp:// if no scheme is present.
func ParseBrokerURL(raw string) string {
	if raw == "" {
		return raw
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "tcp://") ||
		strings.HasPrefix(lower, "ssl://") ||
		strings.HasPrefix(lower, "tls://") ||
		strings.HasPrefix(lower, "ws://") ||
		strings.HasPrefix(lower, "wss://") {
		return raw
	}
	return "tcp://" + raw
}

// BuildClientConfig merges options and config into a ClientConfig with defaults applied.
func BuildClientConfig(options map[string]any, config map[string]any) (*ClientConfig, error) {
	broker := optStr(options, "broker", "")
	if broker == "" {
		// Fall back to config-level broker
		broker = optStr(config, "broker", "")
	}
	if broker == "" {
		return nil, fmt.Errorf("broker is required")
	}

	topic := optStr(options, "topic", "")

	qosStr := optStr(options, "qos", "0")
	qos, err := strconv.Atoi(qosStr)
	if err != nil || qos < 0 || qos > 2 {
		return nil, fmt.Errorf("invalid qos %q: must be 0, 1, or 2", qosStr)
	}

	clientID := optStr(options, "client_id", "")
	if clientID == "" {
		clientID = optStr(config, "client_id", "")
	}
	if clientID == "" {
		clientID = generateClientID()
	}

	username := optStr(options, "username", "")
	if username == "" {
		username = optStr(config, "username", "")
	}
	password := optStr(options, "password", "")
	if password == "" {
		password = optStr(config, "password", "")
	}

	cleanSessionStr := optStr(options, "clean_session", "true")
	cleanSession := cleanSessionStr != "false"

	keepaliveStr := optStr(options, "keepalive", "30")
	keepalive, err := strconv.Atoi(keepaliveStr)
	if err != nil || keepalive < 0 {
		keepalive = 30
	}

	return &ClientConfig{
		Broker:       ParseBrokerURL(broker),
		ClientID:     clientID,
		Username:     username,
		Password:     password,
		CleanSession: cleanSession,
		KeepAlive:    time.Duration(keepalive) * time.Second,
		QoS:          byte(qos),
		Topic:        topic,
	}, nil
}

// Subscribe connects to the broker and subscribes to a topic.
// Returns a channel of received messages and a cleanup function.
func Subscribe(cfg *ClientConfig) (<-chan Message, func(), error) {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetCleanSession(cfg.CleanSession).
		SetKeepAlive(cfg.KeepAlive).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(30 * time.Second)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}

	messages := make(chan Message, 64)

	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		m := Message{
			Topic:     msg.Topic(),
			Payload:   string(msg.Payload()),
			QoS:       msg.Qos(),
			Retained:  msg.Retained(),
			MessageID: msg.MessageID(),
		}
		select {
		case messages <- m:
		default:
			// Drop if channel full
		}
	})

	client := clientFactory(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, nil, fmt.Errorf("connecting to %s: %w", cfg.Broker, err)
	}

	token = client.Subscribe(cfg.Topic, cfg.QoS, nil)
	token.Wait()
	if err := token.Error(); err != nil {
		client.Disconnect(250)
		return nil, nil, fmt.Errorf("subscribing to %s: %w", cfg.Topic, err)
	}

	cleanup := func() {
		client.Unsubscribe(cfg.Topic)
		client.Disconnect(250)
		close(messages)
	}

	return messages, cleanup, nil
}

// Publish connects to the broker, publishes a message, and disconnects.
func Publish(cfg *ClientConfig, payload string, retain bool) error {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetCleanSession(true)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}

	client := clientFactory(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("connecting to %s: %w", cfg.Broker, err)
	}
	defer client.Disconnect(250)

	token = client.Publish(cfg.Topic, cfg.QoS, retain, payload)
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("publishing to %s: %w", cfg.Topic, err)
	}

	return nil
}

// generateClientID creates a random client ID in the form "recur-XXXXXXXX".
func generateClientID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "recur-" + hex.EncodeToString(b)
}

// optStr extracts a string value from a map with a default fallback.
func optStr(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
