package main

import (
	"fmt"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	api "github.com/okitz/mqtt-log-pipeline/api"
)

func main() {
	pub()
}

var (
	broker string = "tcp://mosquitto:1883"
)

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

var connectionLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connection Lost: %s\n", err.Error())
}

func pub() {
	clientId := os.Getenv("MQTT_CLIENT_ID")
	if clientId == "" {
		clientId = "agent0"
	}
	fmt.Printf("ClientId: %s\n", clientId)

	options := mqtt.NewClientOptions()
	options.AddBroker(broker)
	options.SetClientID(clientId)
	options.OnConnect = connectHandler
	options.OnConnectionLost = connectionLostHandler

	fmt.Printf("Connecting to MQTT broker at %s\n", broker)
	client := mqtt.NewClient(options)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	topic := "log/" + clientId
	for i := 0; i < 1000000; i++ {
		payload := api.RandomMetricsJson(clientId)
		token = client.Publish(topic, 1, false, payload)
		fmt.Printf("Publishing %s to topic %s\n", payload, topic)
		if token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		time.Sleep(time.Second)
	}

	client.Disconnect(250)
}
