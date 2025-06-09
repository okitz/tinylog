package app

import (
	"fmt"

	raft_v1 "github.com/okitz/tinylog/api/raft"
	"github.com/okitz/tinylog/internal/filesys"
	logpkg "github.com/okitz/tinylog/internal/log"
	"github.com/okitz/tinylog/internal/mqtt"
	"github.com/okitz/tinylog/internal/raft"
	"github.com/okitz/tinylog/internal/rpc"
)

func Run(broker string) {
	fs, unmount, err := filesys.NewFileSystem()
	if err != nil {
		fmt.Println("Error creating filesystem:", err)
		return
	}
	defer unmount()

	dir := "tmp"
	c := logpkg.Config{}
	c.Segment.MaxStoreBytes = 1024
	c.Segment.MaxIndexBytes = 1024
	log, err := logpkg.NewLog(fs, dir, c)
	if err != nil {
		fmt.Println("Error creating log:", err)
		return
	}
	defer log.Close()

	nodeIds := []string{"node1", "node2", "node3"}
	nodeId := nodeIds[1]
	mqttClt, err := mqtt.NewClient(mqtt.Config{
		Broker:   broker,
		ClientID: nodeId,
	})
	if err != nil {
		fmt.Println("Error creating MQTT client:", err)
		return
	}
	rpcClt := rpc.NewRPCClient(mqttClt, nodeId)
	commitChan := make(chan raft_v1.CommitEntry, 100)
	raft.NewRaft(nodeId, log, nodeIds, rpcClt, commitChan)
	select {}

}
