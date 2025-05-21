package main

import (
	"fmt"
	// "machine"

	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	// "tinygo.org/x/drivers/netlink"
	// "tinygo.org/x/drivers/netlink/probe"
	api "github.com/okitz/mqtt-log-pipeline/api"
	logpkg "github.com/okitz/mqtt-log-pipeline/server/log"
	"tinygo.org/x/tinyfs"
	"tinygo.org/x/tinyfs/littlefs"
)

var (
	fs      *littlefs.LFS
	bd      *tinyfs.MemBlockDevice
	unmount func()
	broker  string = "tcp://mosquitto:1883"
	log     *logpkg.Log
)

func main() {
	dir := "tmp"
	c := logpkg.Config{}
	c.Segment.MaxStoreBytes = 1024
	c.Segment.MaxIndexBytes = 1024
	err := createFs()
	if err != nil {
		fmt.Println("Error creating filesystem:", err)
		return
	}
	defer unmount()
	log, err = logpkg.NewLog(fs, dir, c)
	if err != nil {
		fmt.Println("Error creating log:", err)
		return
	}
	defer log.Close()
	append := &api.Record{
		Value: []byte("hello world"),
	}
	off, err := log.Append(append)
	if err != nil {
		fmt.Println("Error appending record:", err)
		return
	}
	read, err := log.Read(off)
	if err != nil {
		fmt.Println("Error reading record:", err)
		return
	}
	fmt.Printf("Read record at offset %d: %s\n", off, string(read.Value))
	sub()

}

func createFs() error {
	// create/format/mount the filesystem
	bd = tinyfs.NewMemoryDevice(64, 256, 2048)
	fs = littlefs.New(bd).Configure(&littlefs.Config{
		//	ReadSize:      16,
		//	ProgSize:      16,
		//	BlockSize:     512,
		//	BlockCount:    1024,
		CacheSize:     128,
		LookaheadSize: 128,
		BlockCycles:   500,
	})
	if err := fs.Format(); err != nil {
		return err
	}
	if err := fs.Mount(); err != nil {
		return err
	}
	unmount = func() {
		if err := fs.Unmount(); err != nil {
			fmt.Println("Could not unmount", err)
		}
	}
	return nil
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

var connectionLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connection Lost: %s\n", err.Error())
}

var messageHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()
	fmt.Printf("Received: %s on topic: %s\n", payload, msg.Topic())
	metrics := api.Metrics{}
	if err := metrics.UnmarshalJSON(payload); err != nil {
		fmt.Printf("Error unmarshalling JSON: %s\n", err)
		return
	}
	fmt.Printf("SensorId: %s, Temperature: %f, Illuminance: %f, Status: %s\n",
		metrics.SensorId, metrics.Temperature, metrics.Illuminance, metrics.Status)
	record := &api.Record{
		Value: payload,
	}
	off, err := log.Append(record)
	if err != nil {
		fmt.Println("Error appending record:", err)
		return
	}
	fmt.Printf("Appended record at offset %d\n", off)
	if off > 10 {
		// 10こ前のレコードを読み出す
		read, err := log.Read(off - 10)
		if err != nil {
			fmt.Println("Error reading record:", err)
			return
		}
		fmt.Printf("Read record at offset %d: %s\n", off-10, string(read.Value))
	}
}

func sub() {
	// link, _ := probe.Probe()

	// err := link.NetConnect(&netlink.ConnectParams{
	// 	Ssid:       ssid,
	// 	Passphrase: pass,
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }
	clientId := os.Getenv("MQTT_CLIENT_ID")
	if clientId == "" {
		clientId = "server0"
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

	topic := "log/#"
	token = client.Subscribe(topic, 1, messageHandler)
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Printf("Subscribed to topic %s\n", topic)

	select {}

	// client.Disconnect(250)
}

// Wait for user to open serial console
// func waitSerial() {
// 	for !machine.Serial.DTR() {
// 		time.Sleep(100 * time.Millisecond)
// 	}
// }
