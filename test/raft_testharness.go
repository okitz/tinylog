package test

import (
	"fmt"
	"testing"
	"time"

	raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
	logpkg "github.com/okitz/mqtt-log-pipeline/internal/log"
	mqtt "github.com/okitz/mqtt-log-pipeline/internal/mqtt"
	"github.com/okitz/mqtt-log-pipeline/internal/raft"
	"github.com/okitz/mqtt-log-pipeline/internal/rpc"
	tutl "github.com/okitz/mqtt-log-pipeline/internal/testutil"
	"github.com/stretchr/testify/require"
)

type Harness struct {
	raftNodes  []*raft.Raft
	rpcClients []*rpc.RPCClient
	connected  []bool
	nodeIds    []string
	n          int
	t          *testing.T
}

func NewHarness(t *testing.T, n int) *Harness {
	raftNodes := make([]*raft.Raft, n)
	rpcClients := make([]*rpc.RPCClient, n)
	connected := make([]bool, n)
	nodeIds := make([]string, n)

	for i := 0; i < n; i++ {
		nodeIds[i] = fmt.Sprintf("node-%02d", i+1)
	}

	for i, nodeId := range nodeIds {
		cfg := mqtt.Config{
			Broker:   "tcp://mosquitto:1883",
			ClientID: nodeId,
			Timeout:  time.Second * 30,
		}
		mqttClient, err := mqtt.NewClient(cfg)
		if err != nil {
			t.Fatalf("MQTT接続エラー: %v", err)
		}

		rpcClient := rpc.NewRPCClient(mqttClient, nodeId)
		if err := rpcClient.Start(); err != nil {
			t.Fatalf("RPCクライアントの起動に失敗: %v", err)
		}
		var peers []string
		for j, peerId := range nodeIds {
			if nodeId == peerId {
				peers = append(append([]string{}, nodeIds[:j]...), nodeIds[j+1:]...)
				break
			}
		}

		raft := raft.NewRaft(nodeId, (*logpkg.Log)(nil), peers, rpcClient)

		methodNameRV := "raft.RequestVote"
		handlerRV := rpc.MakeProtoRPCHandler(
			&raft_v1.RequestVoteRequest{},
			raft.HandleRequestVoteRequest,
			methodNameRV,
		)
		rpcClient.RegisterMethod(methodNameRV, handlerRV)

		methodNameAE := "raft.AppendEntries"
		handlerAE := rpc.MakeProtoRPCHandler(
			&raft_v1.AppendEntriesRequest{},
			raft.HandleAppendEntriesRequest,
			methodNameAE,
		)
		rpcClient.RegisterMethod(methodNameAE, handlerAE)

		raftNodes[i] = raft
		rpcClients[i] = rpcClient
		connected[i] = true
	}

	return &Harness{
		raftNodes:  raftNodes,
		rpcClients: rpcClients,
		connected:  connected,
		nodeIds:    nodeIds,
		n:          n,
		t:          t,
	}
}

func (h *Harness) Shutdown() {
	for i := 0; i < h.n; i++ {
		h.rpcClients[i].Disconnect()
		h.connected[i] = false
	}
}

func (h *Harness) GetAllNodeInfos() []map[string]interface{} {
	nodeInfos := make([]map[string]interface{}, h.n)
	for i, node := range h.raftNodes {
		nodeInfo := node.GetNodeInfo()
		nodeInfos[i] = nodeInfo
	}
	return nodeInfos
}

func (h Harness) CheckSingleLeader() (string, uint64) {
	tutl.WaitForCondition(h.t, time.Second*10, time.Millisecond*100, func() bool {
		nodeInfos := h.GetAllNodeInfos()
		var latestLeaderId string
		latestTerm := uint64(0)
		for _, info := range nodeInfos {
			leaderId := info["leaderId"].(string)
			currentTerm := info["currentTerm"].(uint64)
			if leaderId == "" {
				return false
			}

			if currentTerm > latestTerm {
				latestTerm = currentTerm
				latestLeaderId = leaderId
			} else if currentTerm == latestTerm {
				if (info["id"] != latestLeaderId || info["state"] != "Leader") &&
					(info["id"] == latestLeaderId || info["state"] != "Follower") {
					return false
				}
			}
			// リーダーが独りだけ存在することを確認
		}
		fmt.Println(nodeInfos)
		return true
	}, "invalid node status")
	nodeInfos := h.GetAllNodeInfos()
	var latestLeaderId string
	latestTerm := uint64(0)
	for _, info := range nodeInfos {
		leaderId := info["leaderId"].(string)
		currentTerm := info["currentTerm"].(uint64)
		if currentTerm > latestTerm {
			latestTerm = currentTerm
			latestLeaderId = leaderId
		}
	}
	return latestLeaderId, latestTerm
}

func (h Harness) CheckNoLeader() {
	for _, node := range h.raftNodes {
		nodeInfo := node.GetNodeInfo()
		require.NotEqual(h.t, nodeInfo["state"], "Leader")
	}

}

func (h *Harness) DisconnectNode(nodeId string) {
	for i, id := range h.nodeIds {
		if id == nodeId {
			h.rpcClients[i].Disconnect()
			h.connected[i] = false
			return
		}
	}
	h.t.Fatalf("Node with ID %s not found", nodeId)
}

// func (h *Harness) ReconnectNode(nodeId string) {
// 	for i, id := range h.nodeIds {
// 		if id == nodeId {
// 			if !h.connected[i] {
// 				h.rpcClients[i].Reconnect()
// 			}
// 			return
// 		}
// 	}
// 	h.t.Fatalf("Node with ID %s not found", nodeId)
// }
