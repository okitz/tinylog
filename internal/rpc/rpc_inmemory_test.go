package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/okitz/tinylog/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestRPCClient_BroadcastAndHandle(t *testing.T) {
	broker := testutil.NewBroker()

	clientA := NewRPCClient(testutil.NewMockMQTTClient(broker, "A"), "A")
	clientB := NewRPCClient(testutil.NewMockMQTTClient(broker, "B"), "B")

	clientB.RegisterMethod("echo", func(params json.RawMessage) (json.RawMessage, error) {
		return params, nil
	})
	if err := clientB.Start(); err != nil {
		t.Fatalf("clientB.Start failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	params := json.RawMessage(`{"hello":"world"}`)
	repCh, err := clientA.BroadcastRPC(ctx, "echo", params)
	if err != nil {
		t.Fatalf("BroadcastRPC failed: %v", err)
	}

	select {
	case res := <-repCh:
		require.NoError(t, res.Error(), "expected no error in response")
		resJSON := res.Raw()
		if string(resJSON) != string(params) {
			t.Errorf("expected %s, got %s", params, resJSON)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for response")
	}
}

func TestRPCClient_CallRPC(t *testing.T) {
	broker := testutil.NewBroker()

	clientA := NewRPCClient(testutil.NewMockMQTTClient(broker, "A"), "A")
	clientB := NewRPCClient(testutil.NewMockMQTTClient(broker, "B"), "B")

	clientB.RegisterMethod("sum", func(params json.RawMessage) (json.RawMessage, error) {
		var data struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err := json.Unmarshal(params, &data); err != nil {
			return nil, err
		}
		result := struct {
			Result int `json:"result"`
		}{Result: data.A + data.B}
		return json.Marshal(result)
	})
	if err := clientB.Start(); err != nil {
		t.Fatalf("clientB.Start failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	params := json.RawMessage(`{"a":2,"b":3}`)
	resp, err := clientA.CallRPC(ctx, "B", "sum", params)
	if err != nil {
		t.Fatalf("CallRPC failed: %v", err)
	}

	expected := `{"result":5}`
	if string(resp) != expected {
		t.Errorf("expected %s, got %s", expected, resp)
	}
}

func TestRPCClient_CallRPC_UnregisteredMethod(t *testing.T) {
	broker := testutil.NewBroker()
	clientA := NewRPCClient(testutil.NewMockMQTTClient(broker, "A"), "A")
	clientB := NewRPCClient(testutil.NewMockMQTTClient(broker, "B"), "B")

	if err := clientB.Start(); err != nil {
		t.Fatalf("clientB.Start failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := clientA.CallRPC(ctx, "B", "noSuchMethod", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown method, got nil")
	}
}
