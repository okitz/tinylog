package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	raft_v1 "github.com/okitz/tinylog/api/raft"
)

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
	reqParams json.RawMessage,
) (<-chan RPCResopnse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.trp.BroadcastTrp(c.id, ctx, method, reqParams)
}

func (c *FakeRPCClient) CallRPC(
	ctx context.Context,
	targetId string,
	method string,
	reqParams json.RawMessage,
) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.trp.CallTrp(c.id, targetId, method, reqParams)
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
	nodes        map[string]*Raft
	disconnected map[string]*atomic.Bool
}

func NewFakeRPCTransporter() *FakeRPCTransporter {
	return &FakeRPCTransporter{
		nodes:        make(map[string]*Raft),
		disconnected: make(map[string]*atomic.Bool),
	}
}

func (f *FakeRPCTransporter) RegisterNode(r *Raft) {
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
) (<-chan RPCResopnse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.disconnected[from].Load() {
		return nil, fmt.Errorf("node %s is disconnected", from)
	}

	ch := make(chan RPCResopnse, 10) // 応答を受け取るためのチャネル
	var wg sync.WaitGroup
	for id, node := range f.nodes {
		if id == from {
			continue
		}
		if f.disconnected[id].Load() {
			continue
		}
		wg.Add(1)
		go func(n *Raft) {
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

func (f *FakeRPCTransporter) CallTrp(
	from string,
	targetId string,
	method string,
	req json.RawMessage,
) (json.RawMessage, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.disconnected[from].Load() {
		return nil, fmt.Errorf("node %s is disconnected", from)
	}

	if f.disconnected[targetId] == nil || f.disconnected[targetId].Load() {
		return nil, fmt.Errorf("target node %s is disconnected", targetId)
	}

	node, ok := f.nodes[targetId]
	if !ok {
		return nil, fmt.Errorf("target node %s not found", targetId)
	}

	switch method {
	case "raft.RequestVote":
		var args raft_v1.RequestVoteRequest
		_ = args.UnmarshalJSON(req)
		rep, _ := node.HandleRequestVoteRequest(&args)
		if rep != nil {
			return rep.MarshalJSON()
		}

	case "raft.AppendEntries":
		var args raft_v1.AppendEntriesRequest
		_ = args.UnmarshalJSON(req)
		rep, _ := node.HandleAppendEntriesRequest(&args)
		if rep != nil {
			return rep.MarshalJSON()
		}
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
	return nil, nil
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
