package test

import (
	"fmt"
	"testing"
	"time"

	raft_v1 "github.com/okitz/tinylog/api/raft"
	logpkg "github.com/okitz/tinylog/internal/log"
	mqtt "github.com/okitz/tinylog/internal/mqtt"
	"github.com/okitz/tinylog/internal/raft"
	"github.com/okitz/tinylog/internal/rpc"
	tutl "github.com/okitz/tinylog/internal/testutil"
	"tinygo.org/x/tinyfs"
	"tinygo.org/x/tinyfs/littlefs"
)

func createTestFS(t *testing.T) *littlefs.LFS {
	// create/format/mount the filesystem
	bd := tinyfs.NewMemoryDevice(64, 256, 2048)
	fs := littlefs.New(bd).Configure(&littlefs.Config{
		CacheSize:     128,
		LookaheadSize: 128,
		BlockCycles:   500,
	})
	if err := fs.Format(); err != nil {
		t.Error("Could not format", err)
	}
	if err := fs.Mount(); err != nil {
		t.Error("Could not mount", err)
	}
	// fs.Unmount()
	return fs
}

func setupLog(t *testing.T) *logpkg.Log {
	fs := createTestFS(t)
	cfg := logpkg.Config{}
	cfg.Segment.MaxStoreBytes = 1024
	cfg.Segment.MaxIndexBytes = 1024
	dir := "tmp"
	log, err := logpkg.NewLog(fs, dir, cfg)
	tutl.Require_NoError(t, err)
	return log
}

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
		nodeIds[i] = fmt.Sprintf("node-%02d", i)
	}

	for i, nodeId := range nodeIds {
		log := setupLog(t)
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

		commitChan := make(chan raft_v1.CommitEntry, 100)
		raft := raft.NewRaft(nodeId, log, peers, rpcClient, commitChan)

		methodNameRV := "raft.RequestVote"
		rpc.RegisterProtoHandler(
			rpcClient,
			&raft_v1.RequestVoteRequest{},
			raft.HandleRequestVoteRequest,
			methodNameRV,
		)

		methodNameAE := "raft.AppendEntries"
		rpc.RegisterProtoHandler(
			rpcClient,
			&raft_v1.AppendEntriesRequest{},
			raft.HandleAppendEntriesRequest,
			methodNameAE,
		)

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
		h.raftNodes[i].Stop()
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
		if len(nodeInfos) == 0 {
			return false
		}

		currentTerm := uint64(0)
		var currentLeaderId string
		for _, info := range nodeInfos {
			t := info["currentTerm"].(uint64)
			if t > currentTerm {
				currentTerm = t
				currentLeaderId = info["leaderId"].(string)
			}
		}
		if currentTerm == 0 || currentLeaderId == "" {
			return false
		}

		leaderCount := 0
		for i, info := range nodeInfos {
			// タームが遅れているノードは無視する
			if info["state"] == "Dead" || !h.connected[i] {
				continue
			}
			if info["leaderId"] != currentLeaderId {
				return false
			}
			if info["state"] == "Leader" {
				leaderCount++
				if info["id"] != currentLeaderId {
					return false
				}
			}
		}
		if leaderCount != 1 {
			return false
		}

		fmt.Println(nodeInfos)
		return true
	}, "invalid node status")
	nodeInfos := h.GetAllNodeInfos()
	currentTerm := uint64(0)
	var currentLeaderId string
	for _, info := range nodeInfos {
		t := info["currentTerm"].(uint64)
		if t > currentTerm {
			currentTerm = t
			currentLeaderId = info["leaderId"].(string)
		}
	}
	return currentLeaderId, currentTerm
}

func (h Harness) CheckNoLeader() {
	for _, node := range h.raftNodes {
		nodeInfo := node.GetNodeInfo()
		tutl.Require_NotEqual(h.t, nodeInfo["state"], "Leader")
	}

}

func (h *Harness) StopNode(nodeId string) {
	for i, id := range h.nodeIds {
		if id == nodeId {
			h.raftNodes[i].Stop()
			h.connected[i] = false
			return
		}
	}
	h.t.Fatalf("Node with ID %s not found", nodeId)
}

func (h *Harness) ResumeNode(nodeId string) {
	for i, id := range h.nodeIds {
		if id == nodeId {
			h.raftNodes[i].Resume()
			h.connected[i] = true
			return
		}
	}
	h.t.Fatalf("Node with ID %s not found", nodeId)
}
