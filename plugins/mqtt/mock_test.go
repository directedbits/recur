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

// mockClient implements MQTTClient for testing.
type mockClient struct {
	mu           sync.Mutex
	connectErr   error
	subscribeErr error
	publishErr   error
	published    []publishCall
	subscribed   []subscribeCall
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
