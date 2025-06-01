package raft

import (
	"context"
	"encoding/json"

	log_v1 "github.com/okitz/mqtt-log-pipeline/api/log"
)

type RPCClient interface {
	CallRPC(ctx context.Context, targetId string, method string, reqParams json.RawMessage) (json.RawMessage, error)
	BroadcastRPC(ctx context.Context, method string, reqParams json.RawMessage) (<-chan RPCResopnse, error)
}

type RPCResopnse interface {
	Error() error
	Raw() json.RawMessage
}

type Log interface {
	Append(data []byte) (uint64, error)              // ログにエントリを追加し、そのインデックスを返す
	Read(index uint64) (*log_v1.Record, error)       // 指定されたインデックスのログエントリを読み取る
	ReadFrom(index uint64) ([]*log_v1.Record, error) // 指定されたインデックス以降の最新までのログエントリを読み取る
	NextIndex() uint64                               // 次に追加されるログエントリのインデックスを返す
}
