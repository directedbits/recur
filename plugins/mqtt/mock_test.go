package main

import (
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// mockToken implements mqtt.Token for test use.
type mockToken struct {
	err error
}

func (t *mockToken) Wait() bool                         { return true }
func (t *mockToken) WaitTimeout(d time.Duration) bool   { return true }
func (t *mockToken) Done() <-chan struct{}               { ch := make(chan struct{}); close(ch); return ch }
func (t *mockToken) Error() error                       { return t.err }

// mockMessage implements mqtt.Message for test use.
type mockMessage struct {
	topic     string
	payload   []byte
	qos       byte
	retained  bool
	messageID uint16
}

func (m *mockMessage) Duplicate() bool   { return false }
func (m *mockMessage) Qos() byte         { return m.qos }
func (m *mockMessage) Retained() bool    { return m.retained }
func (m *mockMessage) Topic() string     { return m.topic }
func (m *mockMessage) MessageID() uint16 { return m.messageID }
func (m *mockMessage) Payload() []byte   { return m.payload }
func (m *mockMessage) Ack()              {}

// mockClient implements MQTTClient for testing.
type mockClient struct {
	mu             sync.Mutex
	connectErr     error
	subscribeErr   error
	publishErr     error
	publishHandler mqtt.MessageHandler // captured from options
	published      []publishCall
	subscribed     []subscribeCall
}

type publishCall struct {
	topic    string
	qos      byte
	retained bool
	payload  string
}

type subscribeCall struct {
	topic string
	qos   byte
}

func (c *mockClient) Connect() mqtt.Token {
	return &mockToken{err: c.connectErr}
}

func (c *mockClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribed = append(c.subscribed, subscribeCall{topic: topic, qos: qos})
	return &mockToken{err: c.subscribeErr}
}

func (c *mockClient) Unsubscribe(topics ...string) mqtt.Token {
	return &mockToken{}
}

func (c *mockClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := ""
	if s, ok := payload.(string); ok {
		p = s
	}
	c.published = append(c.published, publishCall{topic: topic, qos: qos, retained: retained, payload: p})
	return &mockToken{err: c.publishErr}
}

func (c *mockClient) Disconnect(quiesce uint) {}
