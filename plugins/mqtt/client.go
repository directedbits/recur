package main

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient abstracts the MQTT client operations needed by Subscribe and Publish.
type MQTTClient interface {
	Connect() mqtt.Token
	Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token
	Unsubscribe(topics ...string) mqtt.Token
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
	Disconnect(quiesce uint)
}

// MQTTClientFactory creates an MQTTClient from options.
type MQTTClientFactory func(opts *mqtt.ClientOptions) MQTTClient

// defaultClientFactory wraps the real MQTT library.
func defaultClientFactory(opts *mqtt.ClientOptions) MQTTClient {
	return mqtt.NewClient(opts)
}

// clientFactory is the package-level factory, overridden in tests.
var clientFactory MQTTClientFactory = defaultClientFactory
