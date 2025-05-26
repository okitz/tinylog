package mqtt

type Client interface {
	Subscribe(topic string, qos byte, callback MessageHandler) Token
	SubscribeMultiple(topics map[string]byte, callback MessageHandler) Token
	Unsubscribe(topics ...string) Token
	Publish(topic string, qos byte, retained bool, payload interface{}) Token
}

type Message interface {
	Topic() string
	Payload() []byte
}

type MessageHandler func(client Client, msg Message)

type Token interface {
	Wait() bool
	Error() error
}
