package main

import (
	"fmt"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	common "github.com/okitz/mqtt-log-pipeline/common"
)

func main() {
	broker := "tcp://mosquitto:1883"
	clientID := os.Getenv("MQTT_CLIENT_ID")
	if clientID == "" {
		clientID = "agent0"
	}
	topic := "logs/" + clientID
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Connection error:", token.Error())
		os.Exit(1)
	}

	fmt.Println("Connected to MQTT broker")

	// 10秒ごとにログを送信
	for i := 0; ; i++ {
		msg := common.RandomMetrics(clientID)
		token := client.Publish(topic, 0, false, msg)
		token.Wait()
		fmt.Println("Published:", msg)
		time.Sleep(10 * time.Second)
	}
}
