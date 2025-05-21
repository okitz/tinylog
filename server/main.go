package main

import (
	"fmt"
	// "machine"
	"math/rand"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	api "github.com/okitz/mqtt-log-pipeline/api"
	logpkg "github.com/okitz/mqtt-log-pipeline/server/log"

	// "tinygo.org/x/drivers/netlink"
	// "tinygo.org/x/drivers/netlink/probe"
	"tinygo.org/x/tinyfs"
	"tinygo.org/x/tinyfs/littlefs"
)

var (
	fs      *littlefs.LFS
	bd      *tinyfs.MemBlockDevice
	unmount func()
)

func main() {
	dir := "tmp"
	c := logpkg.Config{}
	c.Segment.MaxStoreBytes = 32
	if err := createFs(); err != nil {
		fmt.Println("Error creating filesystem:", err)
		return
	}
	defer unmount()
	log, err := logpkg.NewLog(fs, dir, c)
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
	sub()
	return nil
}

var (
	ssid   string
	pass   string
	broker string = "tcp://mosquitto:1883"
)

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Message %s received on topic %s\n", msg.Payload(), msg.Topic())
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

var connectionLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connection Lost: %s\n", err.Error())
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
	options.SetDefaultPublishHandler(messagePubHandler)
	options.OnConnect = connectHandler
	options.OnConnectionLost = connectionLostHandler

	fmt.Printf("Connecting to MQTT broker at %s\n", broker)
	client := mqtt.NewClient(options)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	topic := "log/#"
	token = client.Subscribe(topic, 1, nil)
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Printf("Subscribed to topic %s\n", topic)

	for i := 0; i < 10; i++ {
		freq := float32(1) / 1000000
		payload := fmt.Sprintf("%.02fMhz", freq)
		token = client.Publish(topic, 0, false, payload)
		if token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		time.Sleep(time.Second)
	}

	client.Disconnect(100)

	for {
		select {}
	}
}

// Returns an int >= min, < max
func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// Generate a random string of A-Z chars with len = l
func randomString(len int) string {
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(randomInt(65, 90))
	}
	return string(bytes)
}

// Wait for user to open serial console
// func waitSerial() {
// 	for !machine.Serial.DTR() {
// 		time.Sleep(100 * time.Millisecond)
// 	}
// }
