//go:build !tinygo

package main

import (
	"fmt"

	env "github.com/caarlos0/env/v11"
	"github.com/okitz/tinylog/pkg/app"
)

type config struct {
	BrokerIP string `env:"BROKER_IP"`
	NodeNum  int    `env:"NODE_NUM"`
	NodeIdx  int    `env:"NODE_INDEX"`
}

func main() {
	var cfg = config{
		// Default values
		BrokerIP: "tcp://mosquitto",
		NodeNum:  3,
		NodeIdx:  0,
	}
	if err := env.Parse(&cfg); err != nil {
		fmt.Println(err)
	}
	if cfg.NodeIdx < 0 || cfg.NodeIdx >= cfg.NodeNum {
		fmt.Printf("node index must be in range [0, %d), got %d\n", cfg.NodeNum, cfg.NodeIdx)
		return
	}

	brokerAddr := cfg.BrokerIP + ":1883"
	var nodeId string
	var peerIds []string
	for i := 0; i < cfg.NodeNum; i++ {
		idStr := fmt.Sprintf("node-%02d", i)
		if i == cfg.NodeIdx {
			nodeId = idStr
		} else {
			peerIds = append(peerIds, idStr)
		}
	}

	app.Run(brokerAddr, nodeId, peerIds)
}
