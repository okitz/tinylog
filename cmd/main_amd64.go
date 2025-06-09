//go:build !tinygo

package main

import "github.com/okitz/tinylog/pkg/app"

var (
	brokerAddr string = "tcp://mosquitto:1883"
)

func main() {
	nodeId := "node-03"
	peerIds := []string{"node-01", "node-02"}
	app.Run(brokerAddr, nodeId, peerIds)
}
