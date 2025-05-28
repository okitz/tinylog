package raft

import (
	"fmt"
	"testing"
	"time"

	logpkg "github.com/okitz/mqtt-log-pipeline/internal/log"
	tutl "github.com/okitz/mqtt-log-pipeline/internal/testutil"
)

type Harness struct {
	raftNodes  []*Raft
	rpcClients []*FakeRPCClient
	connected  []bool
	nodeIds    []string
	n          int
	t          *testing.T
}

func NewHarness(t *testing.T, n int) *Harness {
	raftNodes := make([]*Raft, n)
	rpcClients := make([]*FakeRPCClient, n)
	connected := make([]bool, n)
	nodeIds := make([]string, n)
	tp := NewFakeRPCTransporter()

	for i := 0; i < n; i++ {
		nodeIds[i] = fmt.Sprintf("node-%02d", i)
	}

	for i, nodeId := range nodeIds {
		rpcClient := NewFakeRPCClient(nodeId, tp)
		var peers []string
		for j, peerId := range nodeIds {
			if nodeId == peerId {
				peers = append(append([]string{}, nodeIds[:j]...), nodeIds[j+1:]...)
				break
			}
		}

		raft := NewRaft(nodeId, (*logpkg.Log)(nil), peers, rpcClient)
		tp.RegisterNode(raft)
		raftNodes[i] = raft
		rpcClients[i] = rpcClient
		connected[i] = true
	}

	fmt.Println("new harness with nodes:", nodeIds)
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
		for _, info := range nodeInfos {
			if info["currentTerm"] != currentTerm {
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

func (h *Harness) ReconnectNode(nodeId string) {
	for i, id := range h.nodeIds {
		if id == nodeId {
			h.rpcClients[i].Reconnect()
			h.connected[i] = true
			return
		}
	}
	h.t.Fatalf("Node with ID %s not found", nodeId)
}
