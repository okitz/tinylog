package main

import (
	"fmt"
	"os"
	"os/signal"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	broker := "tcp://mosquitto:1883"
	clientID := "server1"
	topic := "logs/agent1"
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Connection error:", token.Error())
		os.Exit(1)
	}

	fmt.Println("Connected to MQTT broker")

	messageHandler := func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	}

	// Subscribe to the topic
	if token := client.Subscribe(topic, 1, messageHandler); token.Wait() && token.Error() != nil {
		fmt.Println("Subscription error:", token.Error())
		os.Exit(1)
	}
	fmt.Println("Subscribed to topic:", topic)

	// 終了シグナルを待つ
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	fmt.Println("Exiting...")
}
