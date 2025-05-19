package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	common "github.com/okitz/mqtt-log-pipeline/common"
)

func main() {
	broker := "tcp://mosquitto:1883"
	clientID := os.Getenv("MQTT_CLIENT_ID")
	if clientID == "" {
		clientID = "server0"
	}
	topic := "logs/#"
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	client := mqtt.NewClient(opts)

	logfile_name := fmt.Sprintf("%s.log", time.Now().Format("2006-01-02"))
	logfile, err := os.OpenFile("./log/"+logfile_name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logfile, "", log.LstdFlags)
	if err != nil {
		log.Fatalf("Cannot open log file: %v", err)
	}
	defer logfile.Close()

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Connection error:", token.Error())
		os.Exit(1)
	}
	fmt.Println("Connected to MQTT broker")

	messageHandler := func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Received log record: %s from topic: %s\n", msg.Payload(), msg.Topic())
		var met common.Metrics
		err := json.Unmarshal(msg.Payload(), &met)
		if err != nil {
			fmt.Println("Error unmarshalling JSON:", err)
			return
		}
		logger.Printf("%s %s %f %f %s\n", met.Timestamp, met.SensorID, met.Temperature, met.Illuminance, met.Status)
	}

	// Subscribe to the topic
	if token := client.Subscribe(topic, 1, messageHandler); token.Wait() && token.Error() != nil {
		fmt.Println("Subscription error:", token.Error())
		os.Exit(1)
	}
	fmt.Println("Subscribed to topic:", topic)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	fmt.Println("Exiting...")
}
