//go:build !tinygo

package main

import "github.com/okitz/tinylog/pkg/app"

var (
	brokerAddr string = "tcp://mosquitto:1883"
)

func main() {
	app.Run(brokerAddr)
}
