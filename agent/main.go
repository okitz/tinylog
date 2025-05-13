package main

import (
	"fmt"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	broker := "tcp://mosquitto:1883"
	clientID := "agent1"
	topic := "logs/agent1"
	time.Sleep(10 * time.Second)
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Connection error:", token.Error())
		os.Exit(1)
	}

	fmt.Println("Connected to MQTT broker")

	// 10秒ごとにログを送信
	for i := 0; ; i++ {
		msg := fmt.Sprintf("Log #%d from Agent1 at %s", i, time.Now().Format(time.RFC3339))
		token := client.Publish(topic, 0, false, msg)
		token.Wait()
		fmt.Println("Published:", msg)
		time.Sleep(10 * time.Second)
	}
}
