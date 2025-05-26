package test

import (
	// "fmt"
	"testing"
	"time"
	// "github.com/stretchr/testify/require"
	// raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
	// logpkg "github.com/okitz/mqtt-log-pipeline/internal/log"
	// tutl "github.com/okitz/mqtt-log-pipeline/internal/testutil"
	// "github.com/okitz/mqtt-log-pipeline/internal/mqtt"
	// "github.com/okitz/mqtt-log-pipeline/internal/rpc"
	// "github.com/okitz/mqtt-log-pipeline/internal/raft"
)

func TestMQTTElectionBasic(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()
	// クラスタ初期化直後はリーダーがいない
	h.CheckNoLeader()
	// しばらく待つとリーダーが選出される
	h.CheckSingleLeader()
}

func TestMQTTElectionLeaderDisconnect(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	origLeaderId, origTerm := h.CheckSingleLeader()

	h.DisconnectNode(origLeaderId)
	time.Sleep(time.Millisecond * 500) // リーダーが切断されるのを待つ

	newLeaderId, newTerm := h.CheckSingleLeader()
	if newLeaderId == origLeaderId {
		t.Errorf("want new leader to be different from orig leader")
	}
	if newTerm <= origTerm {
		t.Errorf("want newTerm <= origTerm, got %d and %d", newTerm, origTerm)
	}
}

func TestElectionLeaderAndAnotherDisconnect(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	origLeaderId, _ := h.CheckSingleLeader()

	h.DisconnectNode(origLeaderId)
	otherId := h.nodeIds[0]
	if otherId == origLeaderId {
		otherId = h.nodeIds[1] // 別のノードを選ぶ
	}
	h.DisconnectNode(otherId)

	time.Sleep(time.Millisecond * 500) // リーダーともう一つのノードが切断されるのを待つ
	h.CheckNoLeader()

	// h.ReconnectNode(otherId)
	h.CheckSingleLeader()
}
