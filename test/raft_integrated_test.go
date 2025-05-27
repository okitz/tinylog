package test

import (
	// "fmt"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
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

func TestMQTTElectionLeaderStop(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	origLeaderId, origTerm := h.CheckSingleLeader()

	h.StopNode(origLeaderId)
	time.Sleep(time.Millisecond * 100) // リーダーが切断されるのを待つ

	newLeaderId, newTerm := h.CheckSingleLeader()
	if newLeaderId == origLeaderId {
		t.Errorf("want new leader to be different from orig leader")
	}
	if newTerm <= origTerm {
		t.Errorf("want newTerm <= origTerm, got %d and %d", newTerm, origTerm)
	}
}

func TestElectionLeaderAndAnotherStop(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	origLeaderId, _ := h.CheckSingleLeader()

	h.StopNode(origLeaderId)
	otherId := h.nodeIds[0]
	if otherId == origLeaderId {
		otherId = h.nodeIds[1] // 別のノードを選ぶ
	}
	h.StopNode(otherId)

	time.Sleep(time.Millisecond * 500) // リーダーともう一つのノードが切断されるのを待つ
	h.CheckNoLeader()

	h.ResumeNode(otherId)
	h.CheckSingleLeader()
}

func TestStopAllThenRestore(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	time.Sleep(time.Millisecond * 100)
	for _, id := range h.nodeIds {
		h.StopNode(id)
	}
	time.Sleep(time.Millisecond * 450)
	h.CheckNoLeader()

	for _, id := range h.nodeIds {
		h.ResumeNode(id)
	}
	h.CheckSingleLeader()
}

func TestElectionLeaderStopThenResume(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()
	origLeaderId, _ := h.CheckSingleLeader()

	h.StopNode(origLeaderId)

	time.Sleep(time.Millisecond * 350)
	newLeaderId, newTerm := h.CheckSingleLeader()

	h.ResumeNode(origLeaderId)
	time.Sleep(time.Millisecond * 150)

	againLeaderId, againTerm := h.CheckSingleLeader()

	if newLeaderId != againLeaderId {
		t.Errorf("again leader id got %s; want %s", againLeaderId, newLeaderId)
	}
	if againTerm != newTerm {
		t.Errorf("again term got %d; want %d", againTerm, newTerm)
	}
}

func TestElectionLeaderStopThenResume5(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, 5)
	defer h.Shutdown()

	origLeaderId, _ := h.CheckSingleLeader()

	h.StopNode(origLeaderId)
	time.Sleep(time.Millisecond * 150)
	newLeaderId, newTerm := h.CheckSingleLeader()

	h.ResumeNode(origLeaderId)
	time.Sleep(time.Millisecond * 300)

	againLeaderId, againTerm := h.CheckSingleLeader()

	if newLeaderId != againLeaderId {
		t.Errorf("again leader id got %s; want %s", againLeaderId, newLeaderId)
	}
	if againTerm != newTerm {
		t.Errorf("again term got %d; want %d", againTerm, newTerm)
	}
}

// フォロワーがダウンしてもリーダーは変わらない
func TestElectionFollowerStopThenComesBack(t *testing.T) {
	// defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, 3)
	defer h.Shutdown()

	origLeaderId, origTerm := h.CheckSingleLeader()

	otherId := h.nodeIds[0]
	if otherId == origLeaderId {
		otherId = h.nodeIds[1] // 別のノードを選ぶ
	}
	h.StopNode(otherId)
	time.Sleep(650 * time.Millisecond)
	h.ResumeNode(otherId)
	time.Sleep(time.Millisecond * 150)

	_, newTerm := h.CheckSingleLeader()
	if newTerm != origTerm {
		t.Errorf("newTerm=%d, origTerm=%d", newTerm, origTerm)
	}
}

// フォロワーが切断されてタームが進んでいた場合、新たなリーダーが選出
func TestElectionFollowerDisconnectThenComesBack(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, 3)
	defer h.Shutdown()

	origLeaderId, origTerm := h.CheckSingleLeader()

	otherId := h.nodeIds[0]
	if otherId == origLeaderId {
		otherId = h.nodeIds[1] // 別のノードを選ぶ
	}
	h.DisconnectNode(otherId)
	time.Sleep(650 * time.Millisecond)
	h.ReconnectNode(otherId)
	time.Sleep(time.Millisecond * 150)

	_, newTerm := h.CheckSingleLeader()
	if newTerm <= origTerm {
		t.Errorf("newTerm=%d, origTerm=%d", newTerm, origTerm)
	}
}

func TestElectionStopLoop(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, 3)
	defer h.Shutdown()

	for cycle := 0; cycle < 5; cycle++ {
		leaderId, _ := h.CheckSingleLeader()

		h.StopNode(leaderId)
		otherId := h.nodeIds[0]
		if otherId == leaderId {
			otherId = h.nodeIds[1] // 別のノードを選ぶ
		}
		h.StopNode(otherId)
		time.Sleep(time.Millisecond * 310)
		h.CheckNoLeader()

		h.ResumeNode(otherId)
		h.ResumeNode(leaderId)

		time.Sleep(time.Millisecond * 150)
	}
}
