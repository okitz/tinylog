package mqtt

// import (
// 	"fmt"
// 	"time"

// 	mqtt "github.com/eclipse/paho.mqtt.golang"
// )

// type Config struct {
// 	Broker   string
// 	ClientID string
// 	Username string
// 	Password string
// 	Timeout  time.Duration
// }

// type c struct {
// 	client mqtt.Client
// }

// func NewClient(cfg Config) (Client, error) {
// 	opts := mqtt.NewClientOptions().
// 		AddBroker(cfg.Broker).
// 		SetClientID(cfg.ClientID).
// 		SetConnectTimeout(cfg.Timeout)
// 	if cfg.Username != "" {
// 		opts.SetUsername(cfg.Username)
// 	}
// 	if cfg.Password != "" {
// 		opts.SetPassword(cfg.Password)
// 	}
// 	c := mqtt.NewClient(opts)

// 	fmt.Printf("Connecting to MQTT broker at %s\n", cfg.Broker)
// 	if token := c.Connect(); token.Wait() && token.Error() != nil {
// 		return nil, token.Error()
// 	}
// 	return &c, nil
// }

// func (w *Client) Subscribe(topic string, handler mqtt.MessageHandler) error {
// 	token := w.client.Subscribe(topic, 1, handler)
// 	token.Wait()
// 	return token.Error()
// }

// func (w *Client) Publish(topic string, payload []byte) error {
// 	token := w.client.Publish(topic, 1, false, payload)
// 	token.Wait()
// 	return token.Error()
// }
