package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
	logpkg "github.com/okitz/mqtt-log-pipeline/internal/log"
	"github.com/okitz/mqtt-log-pipeline/internal/raft"
)

type Harness struct {
	raftNodes  []*raft.Raft
	rpcClients []*FakeRPCClient
	connected  []bool
	nodeIds    []string
	n          int
}

func NewHarness(n int) *Harness {
	raftNodes := make([]*raft.Raft, n)
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

		raft := raft.NewRaft(nodeId, (*logpkg.Log)(nil), peers, rpcClient)
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

func (h *Harness) StopNode(nodeId string) {
	for i, id := range h.nodeIds {
		if id == nodeId {
			h.raftNodes[i].Stop()
			h.connected[i] = false
			return
		}
	}
}

func (h *Harness) ResumeNode(nodeId string) {
	for i, id := range h.nodeIds {
		if id == nodeId {
			h.raftNodes[i].Resume()
			h.connected[i] = true
			return
		}
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
}

func (h *Harness) ReconnectNode(nodeId string) {
	for i, id := range h.nodeIds {
		if id == nodeId {
			h.rpcClients[i].Reconnect()
			h.connected[i] = true
			return
		}
	}
}

type FakeRPCClient struct {
	id  string
	mu  sync.Mutex
	trp *FakeRPCTransporter
}

func NewFakeRPCClient(id string, trp *FakeRPCTransporter) *FakeRPCClient {
	return &FakeRPCClient{
		id:  id,
		trp: trp,
	}
}
func (c *FakeRPCClient) BroadcastRPC(
	ctx context.Context,
	method string,
	req json.RawMessage,
) (<-chan raft.RPCResopnse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.trp.BroadcastTrp(c.id, ctx, method, req)
}

func (c *FakeRPCClient) CallRPC(
	ctx context.Context,
	targetId string,
	method string,
	reqParams json.RawMessage,
) (json.RawMessage, error) {
	return nil, fmt.Errorf("CallRPC not implemented in FakeRPCClient")
}

func (c *FakeRPCClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.trp.DisconnectNode(c.id)
}

func (c *FakeRPCClient) Reconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.trp.ReconnectNode(c.id)
}

type FakeRPCTransporter struct {
	mu           sync.RWMutex
	nodes        map[string]*raft.Raft
	disconnected map[string]*atomic.Bool
}

func NewFakeRPCTransporter() *FakeRPCTransporter {
	return &FakeRPCTransporter{
		nodes:        make(map[string]*raft.Raft),
		disconnected: make(map[string]*atomic.Bool),
	}
}

func (f *FakeRPCTransporter) RegisterNode(r *raft.Raft) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nodes[r.Id] = r
	f.disconnected[r.Id] = &atomic.Bool{}
}

func (f *FakeRPCTransporter) DisconnectNode(id string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if b, ok := f.disconnected[id]; ok {
		b.Store(true)
	}
}

func (f *FakeRPCTransporter) ReconnectNode(id string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if b, ok := f.disconnected[id]; ok {
		b.Store(false)
	}
}

func (f *FakeRPCTransporter) BroadcastTrp(
	from string,
	ctx context.Context,
	method string,
	req json.RawMessage,
) (<-chan raft.RPCResopnse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.disconnected[from].Load() {
		return nil, nil
	}

	ch := make(chan raft.RPCResopnse, 10) // 応答を受け取るためのチャネル
	var wg sync.WaitGroup
	for id, node := range f.nodes {
		if id == from {
			continue
		}
		if f.disconnected[id].Load() {
			continue
		}
		wg.Add(1)
		go func(n *raft.Raft) {
			defer wg.Done()
			switch method {
			case "raft.RequestVote":
				var args raft_v1.RequestVoteRequest
				_ = args.UnmarshalJSON(req)
				rep, _ := n.HandleRequestVoteRequest(&args)
				if rep != nil {
					b, _ := rep.MarshalJSON()
					r := FakeRPCResopnse{b}
					ch <- r
				}

			case "raft.AppendEntries":
				var args raft_v1.AppendEntriesRequest
				_ = args.UnmarshalJSON(req)
				rep, _ := n.HandleAppendEntriesRequest(&args)
				if rep != nil {
					b, _ := rep.MarshalJSON()
					r := FakeRPCResopnse{b}
					ch <- r
				}

			}
		}(node)
	}

	// 全ゴルーチンが応答を送信し終えるまで待ってからチャンネルを閉じる
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch, nil
}

type FakeRPCResopnse struct {
	rawJson json.RawMessage
}

func (f FakeRPCResopnse) Raw() json.RawMessage {
	return f.rawJson
}
func (f FakeRPCResopnse) Error() error {
	// FakeRPCResopnse does not handle errors
	return nil
}
