package raft

import (
	"context"
	"encoding/json"
)

type RPCClient interface {
	CallRPC(ctx context.Context, targetId string, method string, reqParams json.RawMessage) (json.RawMessage, error)
	BroadcastRPC(ctx context.Context, method string, reqParams json.RawMessage) (<-chan RPCResopnse, error)
}

type RPCResopnse interface {
	Error() error
	Raw() json.RawMessage
}
