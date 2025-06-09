// rpc/jsonrpc.go
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	mqtt "github.com/okitz/tinylog/internal/mqtt"
	raft "github.com/okitz/tinylog/internal/raft"
)

const QOS = 2

type Request struct {
	RequestId string          `json:"requestId"`
	SenderId  string          `json:"sender_id"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	RequestId string          `json:"requestId"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     ResponseError   `json:"error,omitempty"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type RPCResopnse struct {
	raw json.RawMessage
	err error
}

func NewRPCResponse(raw json.RawMessage, err error) RPCResopnse {
	return RPCResopnse{
		raw: raw,
		err: err,
	}
}

func (r RPCResopnse) Error() error {
	return r.err
}
func (r RPCResopnse) Raw() json.RawMessage {
	return r.raw
}

type MQTTClient interface {
	Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token
	SubscribeMultiple(topics map[string]byte, callback mqtt.MessageHandler) mqtt.Token
	Unsubscribe(topics ...string) mqtt.Token
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
}

// ハンドラの入出力はprotobufで定義する
type Handler func(params json.RawMessage) (json.RawMessage, error)

type RPCClient struct {
	id         string // client ID
	mqtt       mqtt.Client
	registry   *Registry // method registry
	pending    map[string](chan<- raft.RPCResopnse)
	pendingMux sync.Mutex
}

func NewRPCClient(m mqtt.Client, clientId string) *RPCClient {
	c := &RPCClient{
		id:       clientId,
		mqtt:     m,
		registry: NewRegistry(),
		pending:  make(map[string]chan<- raft.RPCResopnse),
	}
	return c
}

func (c *RPCClient) ID() string {
	return c.id
}

func (c *RPCClient) RegisterMethod(name string, handler Handler) {
	c.registry.Register(name, handler)
}

func (c *RPCClient) Start() error {
	c.pendingMux.Lock()
	defer c.pendingMux.Unlock()

	// Clear pending requests
	for reqId, ch := range c.pending {
		close(ch)
		delete(c.pending, reqId)
	}
	if err := c.subscribeRequests(); err != nil {
		return fmt.Errorf("failed to subscribe to requests: %w", err)
	}

	return nil
}

func (c *RPCClient) Disconnect() {
	c.mqtt.Disconnect()
}

func (c *RPCClient) subscribeRequests() error {
	handler := func(_ mqtt.Client, msg mqtt.Message) {
		c.handleRequest(msg.Payload())
	}

	topics := map[string]byte{
		"rpc/broadcast": QOS,
		"rpc/" + c.id:   QOS,
	}
	token := c.mqtt.SubscribeMultiple(topics, handler)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe: %w", token.Error())
	}
	return nil
}

func (c *RPCClient) handleRequest(payload []byte) {
	var req Request
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}
	// ここで自身に向けたリクエストを弾く
	if req.SenderId == c.id {
		return
	}

	handler := c.registry.Get(req.Method)
	res := Response{
		RequestId: req.RequestId,
		Error: ResponseError{
			Code:    0,
			Message: "",
		},
	}
	if handler == nil {
		res.Error = ResponseError{
			Code:    404,
			Message: fmt.Sprintf("method %s not found", req.Method),
		}
	} else if req.Params == nil {
		res.Error = ResponseError{
			Code:    400,
			Message: "params cannot be empty",
		}
	} else {
		result, resErr := handler(req.Params)
		if result == nil {
			return
		} else if resErr != nil {
			res.Error = ResponseError{
				Code:    500,
				Message: fmt.Sprintf("error handling request: %s", resErr.Error()),
			}
		} else {
			var err error
			res.Result, err = result.MarshalJSON()
			if err != nil {
				res.Error = ResponseError{
					Code:    500,
					Message: fmt.Sprintf("error marshalling result: %s", err.Error()),
				}
			}
		}
	}
	out, _ := json.Marshal(res)

	topic := "rpc/response/" + req.RequestId
	token := c.mqtt.Publish(topic, QOS, false, out)
	if token.Wait() && token.Error() != nil {
		fmt.Printf("Failed to publish response: %s\n", token.Error())
		return
	}
}

func (c *RPCClient) CallRPC(ctx context.Context, targetId string, method string, reqParams json.RawMessage) (json.RawMessage, error) {
	req := Request{
		RequestId: uuid.New().String(),
		SenderId:  c.id,
		Method:    method,
		Params:    reqParams,
	}
	out, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	repCh := make(chan raft.RPCResopnse, 10)
	c.pendingMux.Lock()
	c.pending[req.RequestId] = repCh
	c.pendingMux.Unlock()
	if err := c.subscribeResponses(ctx, req.RequestId); err != nil {
		return nil, err
	}

	topic := "rpc/" + targetId
	token := c.mqtt.Publish(topic, QOS, false, out)
	if token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to publish request: %w", token.Error())
	}
	select {
	case res := <-repCh:
		if res.Error() != nil {
			return nil, fmt.Errorf("error in response: %w", res.Error())
		}
		if res.Raw() == nil {
			return nil, fmt.Errorf("no result in response for request id %s", req.RequestId)
		}
		return res.Raw(), nil
	case <-ctx.Done():
		c.pendingMux.Lock()
		close(c.pending[req.RequestId])
		delete(c.pending, req.RequestId)
		c.pendingMux.Unlock()
		return nil, ctx.Err()
	}
}

func (c *RPCClient) BroadcastRPC(ctx context.Context, method string, reqParams json.RawMessage) (<-chan raft.RPCResopnse, error) {
	req := Request{
		RequestId: uuid.New().String(),
		SenderId:  c.id,
		Method:    method,
		Params:    reqParams,
	}
	out, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	repCh := make(chan raft.RPCResopnse, 10)
	c.pendingMux.Lock()
	c.pending[req.RequestId] = repCh
	c.pendingMux.Unlock()

	if err := c.subscribeResponses(ctx, req.RequestId); err != nil {
		return nil, err
	}
	topic := "rpc/broadcast"
	token := c.mqtt.Publish(topic, QOS, false, out)

	cleanup := func() {
		c.pendingMux.Lock()
		if ch, ok := c.pending[req.RequestId]; ok {
			close(ch)
			delete(c.pending, req.RequestId)
		}
		c.pendingMux.Unlock()
	}
	if token.Wait() && token.Error() != nil {
		cleanup()
		return nil, fmt.Errorf("failed to publish broadcast request: %w", token.Error())
	}
	go func() {
		<-ctx.Done()
		cleanup()
	}()
	return repCh, nil
}

func (c *RPCClient) subscribeResponses(ctx context.Context, reqId string) error {
	handler := func(_ mqtt.Client, msg mqtt.Message) {
		c.handleResponse(msg.Payload(), reqId)
	}

	topic := "rpc/response/" + reqId
	token := c.mqtt.Subscribe(topic, QOS, handler)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to responses: %w", token.Error())
	}

	go func() {
		<-ctx.Done()
		unsubToken := c.mqtt.Unsubscribe(topic)
		unsubToken.Wait()
	}()
	return nil
}

func (c *RPCClient) handleResponse(payload []byte, reqId string) {
	var res Response
	if err := json.Unmarshal(payload, &res); err != nil {
		return
	}
	if res.RequestId != reqId {
		return
	}

	c.pendingMux.Lock()
	resCh, ok := c.pending[res.RequestId]
	c.pendingMux.Unlock()
	if !ok {
		fmt.Printf("No pending request for response id %s\n", res.RequestId)
		return
	}

	rep := RPCResopnse{
		raw: res.Result,
		err: nil,
	}
	if res.Error.Code != 0 {
		fmt.Printf("Error in response %s: %s\n", res.RequestId, res.Error.Message)
		rep.err = fmt.Errorf("error in response: %s", res.Error.Message)
		resCh <- rep
		return
	}
	if res.Result == nil {
		fmt.Printf("No result in response for request id %s\n", res.RequestId)
		rep.err = fmt.Errorf("no result in response for request id %s", res.RequestId)
		return
	}
	select {
	case resCh <- rep:
		fmt.Printf("Response for request %s sent successfully\n", reqId)
	default:
		fmt.Printf("[WARN] repCh full: dropping reply from topic %s", reqId)

	}

}
