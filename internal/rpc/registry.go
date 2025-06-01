package rpc

import (
	"encoding/json"
	"fmt"

	protobuf_go_lite "github.com/aperturerobotics/protobuf-go-lite"
)

// Registry holds JSON-RPC method handlers
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register adds a handler for a method name
func (r *Registry) Register(name string, h Handler) {
	r.handlers[name] = h
}

// Get returns the handler for a method name
func (r *Registry) Get(name string) Handler {
	return r.handlers[name]
}

// Protobufで定義されたリクエストとレスポンスを処理するRPCハンドラを登録
// RaftのRPCメソッドを登録するための関数
func RegisterProtoHandler[Req, Rep protobuf_go_lite.JSONMessage](
	c *RPCClient,
	req Req,
	pHandler func(Req) (Rep, error),
	methodName string,
) {
	handler := makeProtoRPCHandler(
		req,
		pHandler,
		methodName,
	)
	c.RegisterMethod(methodName, handler)
}

// Protobufで定義されたresquestとresponseを処理するRPCハンドラを作成する
func makeProtoRPCHandler[Req, Rep protobuf_go_lite.JSONMessage](
	req Req,
	handle func(Req) (Rep, error),
	methodName string,
) func(json.RawMessage) (json.RawMessage, error) {
	return func(params json.RawMessage) (json.RawMessage, error) {
		if err := req.UnmarshalJSON(params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", methodName, err)
		}
		rep, err := handle(req)
		if err != nil {
			return nil, fmt.Errorf("failed to handle %s: %w", methodName, err)
		}
		// if any(rep) == nil {
		// 	return nil, fmt.Errorf("%s response is nil", methodName)
		// }
		return rep.MarshalJSON()
	}
}
