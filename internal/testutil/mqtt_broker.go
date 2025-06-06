package testutil

import (
	"sync"
	"time"

	mqtt "github.com/okitz/tinylog/internal/mqtt"
)

// MQTTテスト用のモック・インメモリブローカーの実装
type Broker struct {
	mu       sync.RWMutex
	handlers map[string][]mqtt.MessageHandler
}

func NewBroker() *Broker {
	return &Broker{
		handlers: make(map[string][]mqtt.MessageHandler),
	}
}

type MockMQTTClient struct {
	broker *Broker
	id     string
}

func NewMockMQTTClient(b *Broker, clientID string) *MockMQTTClient {
	return &MockMQTTClient{broker: b, id: clientID}
}

func (c *MockMQTTClient) Subscribe(topic string, qos byte, handler mqtt.MessageHandler) mqtt.Token {
	c.broker.mu.Lock()
	defer c.broker.mu.Unlock()
	c.broker.handlers[topic] = append(c.broker.handlers[topic], handler)
	return &mockMQTTToken{}
}

func (c *MockMQTTClient) SubscribeMultiple(topics map[string]byte, handler mqtt.MessageHandler) mqtt.Token {
	c.broker.mu.Lock()
	defer c.broker.mu.Unlock()
	for t := range topics {
		c.broker.handlers[t] = append(c.broker.handlers[t], handler)
	}
	return &mockMQTTToken{}
}

func (c *MockMQTTClient) Unsubscribe(topics ...string) mqtt.Token {
	c.broker.mu.Lock()
	defer c.broker.mu.Unlock()
	for _, t := range topics {
		delete(c.broker.handlers, t)
	}
	return &mockMQTTToken{}
}

func (c *MockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	c.broker.mu.RLock()
	handlers := append([]mqtt.MessageHandler(nil), c.broker.handlers[topic]...)
	c.broker.mu.RUnlock()

	msg := &mockMQTTMessage{topic: topic, payload: payload.([]byte)}
	for _, h := range handlers {
		h(c, msg)
	}
	return &mockMQTTToken{}
}

func (c *MockMQTTClient) Disconnect() {
	c.broker.mu.Lock()
	defer c.broker.mu.Unlock()
	c.broker.handlers = make(map[string][]mqtt.MessageHandler)
}

type mockMQTTToken struct{}

func (m *mockMQTTToken) Wait() bool                       { return true }
func (m *mockMQTTToken) WaitTimeout(_ time.Duration) bool { return true }
func (m *mockMQTTToken) Error() error                     { return nil }

type mockMQTTMessage struct {
	topic   string
	payload []byte
}

func (m *mockMQTTMessage) Duplicate() bool   { return false }
func (m *mockMQTTMessage) Qos() byte         { return 0 }
func (m *mockMQTTMessage) Retained() bool    { return false }
func (m *mockMQTTMessage) Topic() string     { return m.topic }
func (m *mockMQTTMessage) MessageID() uint16 { return 0 }
func (m *mockMQTTMessage) Payload() []byte   { return m.payload }
func (m *mockMQTTMessage) Ack()              {}
