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

func setupLog(cfg logpkg.Config) (*logpkg.Log, error) {
	fs, unmount, err := filesys.NewFileSystem()
	if err != nil {
		fmt.Println("Error creating filesystem:", err)
		return nil, err
	}
	defer unmount()

	dir := "tmp"
	log, err := logpkg.NewLog(fs, dir, cfg)
	if err != nil {
		fmt.Println("Error creating log:", err)
		return nil, err
	}
	return log, nil
}

func setupRaftNode(brokerAddr string, nodeId string, peerIds []string, log *logpkg.Log) (*raft.Raft, error) {
	mqttClt, err := mqtt.NewClient(mqtt.Config{
		Broker:   brokerAddr,
		ClientID: nodeId,
	})
	if err != nil {
		fmt.Println("Error creating MQTT client:", err)
		return nil, err
	}

	rpcClt := rpc.NewRPCClient(mqttClt, nodeId)
	commitChan := make(chan raft_v1.CommitEntry, 100)
	raftNode := raft.NewRaft(nodeId, log, peerIds, rpcClt, commitChan)
	rpcClt.Start()
	methodNameRV := "raft.RequestVote"
	rpc.RegisterProtoHandler(
		rpcClt,
		&raft_v1.RequestVoteRequest{},
		raftNode.HandleRequestVoteRequest,
		methodNameRV,
	)

	methodNameAE := "raft.AppendEntries"
	rpc.RegisterProtoHandler(
		rpcClt,
		&raft_v1.AppendEntriesRequest{},
		raftNode.HandleAppendEntriesRequest,
		methodNameAE,
	)

	return raftNode, nil
}

func Run(brokerAddr, nodeId string, peerIds []string) {
	logCfg := logpkg.Config{}
	logCfg.Segment.MaxStoreBytes = 1024
	logCfg.Segment.MaxIndexBytes = 1024
	log, err := setupLog(logCfg)
	if err != nil {
		fmt.Println("Error setting up log:", err)
		return
	}
	defer log.Close()

	_, err = setupRaftNode(brokerAddr, nodeId, peerIds, log)
	if err != nil {
		fmt.Println("Error setting up Raft node:", err)
		return
	}

	select {}
}
