package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/okitz/mqtt-log-pipeline/internal/mqtt"
	"github.com/stretchr/testify/require"
)

func setupMQTTClient(t *testing.T, clientID string) mqtt.Client {
	cfg := mqtt.Config{
		Broker:   "tcp://mosquitto:1883",
		ClientID: clientID,
		Timeout:  time.Second * 30,
	}
	client, err := mqtt.NewClient(cfg)
	if err != nil {
		t.Fatalf("MQTT接続エラー: %v", err)
	}
	return client
}

func TestRPCClient_BroadcastAndHandle_MOSQUITTO(t *testing.T) {
	clientA := NewRPCClient(setupMQTTClient(t, "A"), "A")
	clientB := NewRPCClient(setupMQTTClient(t, "B"), "B")
	defer clientA.Disconnect()
	defer clientB.Disconnect()

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

func TestRPCClient_CallRPC_MOSQUITTO(t *testing.T) {
	clientA := NewRPCClient(setupMQTTClient(t, "A"), "A")
	clientB := NewRPCClient(setupMQTTClient(t, "B"), "B")
	defer clientA.Disconnect()
	defer clientB.Disconnect()

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

func TestRPCClient_CallRPC_UnregisteredMethod_MOSQUITTO(t *testing.T) {
	clientA := NewRPCClient(setupMQTTClient(t, "A"), "A")
	clientB := NewRPCClient(setupMQTTClient(t, "B"), "B")
	defer clientA.Disconnect()
	defer clientB.Disconnect()

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
