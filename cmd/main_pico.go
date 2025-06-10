//go:build pico || pico_w

package main

import (
	"log/slog"
	"machine"
	"time"

	"github.com/okitz/tinylog/internal/pico"
	"github.com/okitz/tinylog/pkg/app"
	// "tinygo.org/x/drivers/netlink"
	// "tinygo.org/x/drivers/netlink/probe"
)

var (
	brokerIP string
)
var (
	loggerHandler *slog.TextHandler
	Hostname      = []byte("tinygo-mqtt")
)

func main() {
	time.Sleep(10 * time.Second)

	logger := slog.New(slog.NewTextHandler(machine.Serial, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	brokerAddr := brokerIP + ":1883"
	_, stack, dev, err := pico.SetupWithDHCP(pico.SetupConfig{
		Hostname: string(Hostname),
		Logger:   logger,
		TCPPorts: 1, // For HTTP over TCP.
		UDPPorts: 1, // For DNS.
	})
	if err != nil {
		println("setup DHCP:" + err.Error())
	}

	ledCh := make(chan struct{})
	go pico.SetupLED(dev, ledCh)
	wifiReadyCh := make(chan struct{})
	wifiDisconnectCh := make(chan struct{})
	defer func() {
		wifiDisconnectCh <- struct{}{}
		close(wifiReadyCh)
		close(wifiDisconnectCh)
		close(ledCh)
	}()
	go pico.SetupWifi(stack, logger, brokerAddr, wifiReadyCh, wifiDisconnectCh)

	<-wifiReadyCh
	ledCh <- struct{}{}
	println("WiFi is ready!")

	nodeId := "node-00"
	peerIds := []string{"node-01", "node-02"}
	app.Run(brokerAddr, nodeId, peerIds)

}
