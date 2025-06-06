package raft

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	raft_v1 "github.com/okitz/tinylog/api/raft"
	logpkg "github.com/okitz/tinylog/internal/log"
	tutl "github.com/okitz/tinylog/internal/testutil"
	"tinygo.org/x/tinyfs"
	"tinygo.org/x/tinyfs/littlefs"
)

type Harness struct {
	raftNodes   []*Raft
	rpcClients  []*FakeRPCClient
	connected   []bool
	nodeIds     []string
	commitChans []chan raft_v1.CommitEntry
	commits     [][]raft_v1.CommitEntry
	mu          sync.Mutex
	n           int
	t           *testing.T
}

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

func NewHarness(t *testing.T, n int) *Harness {
	raftNodes := make([]*Raft, n)
	rpcClients := make([]*FakeRPCClient, n)
	connected := make([]bool, n)
	nodeIds := make([]string, n)
	commitChans := make([]chan raft_v1.CommitEntry, n)
	commits := make([][]raft_v1.CommitEntry, n)
	tp := NewFakeRPCTransporter()

	for i := 0; i < n; i++ {
		nodeIds[i] = fmt.Sprintf("node-%02d", i)
	}

	for i, nodeId := range nodeIds {
		log := setupLog(t)

		rpcClient := NewFakeRPCClient(nodeId, tp)
		var peers []string
		for j, peerId := range nodeIds {
			if nodeId == peerId {
				peers = append(append([]string{}, nodeIds[:j]...), nodeIds[j+1:]...)
				break
			}
		}

		commitChans[i] = make(chan raft_v1.CommitEntry)

		raft := NewRaft(nodeId, log, peers, rpcClient, commitChans[i])
		tp.RegisterNode(raft)
		raftNodes[i] = raft
		rpcClients[i] = rpcClient
		connected[i] = true
	}

	h := &Harness{
		raftNodes:   raftNodes,
		rpcClients:  rpcClients,
		connected:   connected,
		nodeIds:     nodeIds,
		commitChans: commitChans,
		commits:     commits,
		mu:          sync.Mutex{},
		n:           n,
		t:           t,
	}
	for i := 0; i < n; i++ {
		go h.collectCommits(i)
	}
	fmt.Println("new harness with nodes:", nodeIds)
	return h
}

func (h *Harness) Shutdown() {
	for i := 0; i < h.n; i++ {
		h.rpcClients[i].Disconnect()
		h.raftNodes[i].Stop()
		h.connected[i] = false
		close(h.commitChans[i])
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

func (h *Harness) CheckSingleLeader() (string, uint64) {
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

func (h *Harness) CheckNoLeader() {
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

func (h *Harness) SubmitCommand(nodeId string, command string) (bool, error) {
	for i, id := range h.nodeIds {
		if id == nodeId {
			return h.raftNodes[i].Submit(command)

		}
	}
	h.t.Fatalf("Node with ID %s not found", nodeId)
	return false, fmt.Errorf("node with ID %s not found", nodeId)
}

func (h *Harness) CheckCommitted(cmd string) (nc int, index uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find the length of the commits slice for connected servers.
	commitsLen := -1
	for i := 0; i < h.n; i++ {
		if h.connected[i] {
			if commitsLen >= 0 {
				// If this was set already, expect the new length to be the same.
				if len(h.commits[i]) != commitsLen {
					h.t.Fatalf("commits[%d] = %v, commitsLen = %d", i, h.commits[i], commitsLen)
				}
			} else {
				commitsLen = len(h.commits[i])
			}
		}
	}
	// Check consistency of commits from the start and to the command we're asked
	// about. This loop will return once a command=cmd is found.
	for c := 0; c < commitsLen; c++ {
		cmdAtC := ""
		for i := 0; i < h.n; i++ {
			if h.connected[i] {
				cmdOfN := h.commits[i][c].GetCommand()
				if cmdAtC == "" {
					cmdAtC = cmdOfN
				} else {
					if cmdOfN != cmdAtC {
						h.t.Errorf("got %s, want %s at h.commits[%d][%d]", cmdOfN, cmdAtC, i, c)
					}
				}
			}
		}
		if cmdAtC == cmd {
			// Check consistency of Index.
			index = math.MaxUint64
			nc := 0
			for i := 0; i < h.n; i++ {
				if h.connected[i] {
					if index < math.MaxUint64 && h.commits[i][c].GetIndex() != index {
						h.t.Errorf("got Index=%d, want %d at h.commits[%d][%d]", h.commits[i][c].Index, index, i, c)
					} else {
						index = h.commits[i][c].GetIndex()
					}
					nc++
				}
			}
			return nc, index
		}
	}

	// If there's no early return, we haven't found the command we were looking
	// for.
	h.t.Errorf("cmd=%s not found in commits", cmd)
	return -1, math.MaxUint64
}

// CheckCommittedN verifies that cmd was committed by exactly n connected
// servers.
func (h *Harness) CheckCommittedN(cmd string, n int) {
	nc, _ := h.CheckCommitted(cmd)
	if nc != n {
		h.t.Errorf("CheckCommittedN got nc=%d, want %d", nc, n)
	}
}

func (h *Harness) collectCommits(i int) {
	for c := range h.commitChans[i] {
		h.mu.Lock()
		h.commits[i] = append(h.commits[i], c)
		h.mu.Unlock()
	}
}
