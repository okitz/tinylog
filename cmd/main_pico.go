//go:build pico || pico_w

package main

import (
	"github.com/okitz/tinylog/internal/pico"
	"github.com/okitz/tinylog/pkg/app"
	// "tinygo.org/x/drivers/netlink"
	// "tinygo.org/x/drivers/netlink/probe"
)

var (
	brokerAddr string = "tcp://mosquitto:1883"
)

func main() {
	ledCh := make(chan struct{})
	go pico.SetupLED(ledCh)
	wifiReadyCh := make(chan struct{})
	wifiDisconnectCh := make(chan struct{})
	go pico.SetupWifi(wifiReadyCh, wifiDisconnectCh)

	<-wifiReadyCh
	app.Run(brokerAddr)
}
