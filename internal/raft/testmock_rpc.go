package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
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
	req json.RawMessage,
) (<-chan RPCResopnse, error) {
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
		return nil, nil
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
