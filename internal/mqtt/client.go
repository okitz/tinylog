package mqtt

import (
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type Config struct {
	Broker   string
	ClientID string
	Username string
	Password string
	Timeout  time.Duration
}

type client struct {
	client pahomqtt.Client
	Id     string
}

func NewClient(cfg Config) (Client, error) {
	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetAutoReconnect(true).
		SetClientID(cfg.ClientID).
		SetConnectTimeout(cfg.Timeout)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}
	c := pahomqtt.NewClient(opts)

	fmt.Printf("Connecting to MQTT broker at %s\n", cfg.Broker)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &client{client: c, Id: cfg.ClientID}, nil
}

func (c *client) Subscribe(topic string, qos byte, callback MessageHandler) Token {
	return &token{c.client.Subscribe(topic, qos, func(client pahomqtt.Client, msg pahomqtt.Message) {
		callback(c, &message{msg: msg})
	})}
}

func (c *client) SubscribeMultiple(topics map[string]byte, callback MessageHandler) Token {
	return &token{c.client.SubscribeMultiple(topics, func(client pahomqtt.Client, msg pahomqtt.Message) {
		callback(c, &message{msg: msg})
	})}
}

func (c *client) Unsubscribe(topics ...string) Token {
	return &token{c.client.Unsubscribe(topics...)}
}

func (c *client) Publish(topic string, qos byte, retained bool, payload interface{}) Token {
	return &token{c.client.Publish(topic, qos, retained, payload)}
}

func (c *client) Disconnect() {
	c.client.Disconnect(0)
}

type token struct {
	token pahomqtt.Token
}

func (t token) Wait() bool {
	return t.token.Wait()
}

func (t token) Error() error {
	return t.token.Error()
}

type message struct {
	msg pahomqtt.Message
}

func (m *message) Topic() string {
	return m.msg.Topic()
}

func (m *message) Payload() []byte {
	return m.msg.Payload()
}
