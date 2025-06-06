package raft

import (
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
	tutl "github.com/okitz/mqtt-log-pipeline/internal/testutil"
)

// startElection()を直接呼び出したノードがリーダーになることを確認
func TestClusterElection(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()
	raft0 := h.raftNodes[0]
	// ノード0をLeaderにする
	err := raft0.startElection()
	if err != nil {
		t.Fatalf("startElection failed: %v", err)
	}

	// Leaderになっていることを確認
	if raft0.state != Leader {
		t.Errorf("expected node1 to become Leader; got %v", raft0.state)
	}
	time.Sleep(100 * time.Millisecond) // 少し待つ
	if raft0.state != Leader {
		t.Errorf("expected node1 to be Leader; got %v", raft0.state)
	}
}

// HandleRequestVoteRequest を直接呼び出すテスト
func TestHandleRequestVoteRequest(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()
	raft0 := h.raftNodes[0]
	raft0.currentTerm = 5

	// Term が小さいリクエスト → 拒否、Term は変わらず
	req := &raft_v1.RequestVoteRequest{Term: 4, CandidateId: ""}
	rep, err := raft0.HandleRequestVoteRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Term != 5 || rep.VoteGranted {
		t.Errorf("expected Term=5,VoteGranted=false; got Term=%d,VoteGranted=%v", rep.Term, rep.VoteGranted)
	}

	// 同じTerm＆未投票 → 承認
	req = &raft_v1.RequestVoteRequest{Term: 5, CandidateId: "node-01"}
	rep, _ = raft0.HandleRequestVoteRequest(req)
	if rep.Term != 5 || !rep.VoteGranted {
		t.Errorf("expected Term=5,VoteGranted=true; got Term=%d,VoteGranted=%v", rep.Term, rep.VoteGranted)
	}
	if raft0.votedFor != "node-01" {
		t.Errorf("expected votedFor=node-01; got %s", raft0.votedFor)
	}

	// 同じTermでも別の候補者に2度目投票は拒否
	req = &raft_v1.RequestVoteRequest{Term: 5, CandidateId: "node-02"}
	rep, _ = raft0.HandleRequestVoteRequest(req)
	if rep.VoteGranted {
		t.Error("expected second vote in same term to be denied")
	}

	// より大きいTerm → フォロワーモード＆投票リセット後に承認
	raft0.state = Candidate
	req = &raft_v1.RequestVoteRequest{Term: 6, CandidateId: "node-02"}
	rep, _ = raft0.HandleRequestVoteRequest(req)
	if raft0.state != Follower {
		t.Errorf("expected becomeFollower on higher term; got state=%v", raft0.state)
	}
	if !rep.VoteGranted {
		t.Error("expected vote granted for new term")
	}
}

// HandleAppendEntriesRequest を直接呼び出すテスト
func TestHandleAppendEntriesRequest(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()
	raft0 := h.raftNodes[0]
	raft0.currentTerm = 5

	// Term が小さいエントリ → 拒否
	req := &raft_v1.AppendEntriesRequest{Term: 4, LeaderId: "node-01"}
	rep, err := raft0.HandleAppendEntriesRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Success {
		t.Error("expected Success=false for lower term")
	}

	// 同じTerm → フォロワーに遷移して承認
	req = &raft_v1.AppendEntriesRequest{Term: 5, LeaderId: "node-01"}
	rep, _ = raft0.HandleAppendEntriesRequest(req)
	if rep.Term != 5 || !rep.Success {
		t.Errorf("expected Term=5, Success=true; got Term=%d, Success=%v", rep.Term, rep.Success)
	}
	if raft0.state != Follower {
		t.Errorf("expected state=Follower; got %v", raft0.state)
	}
	if raft0.leaderId != "node-01" {
		t.Errorf("expected leaderId=node-01; got %s", raft0.leaderId)
	}

	// より大きいTerm → フォロワー遷移＆承認
	raft0.state = Candidate
	req = &raft_v1.AppendEntriesRequest{Term: 6, LeaderId: "node-02"}
	rep, _ = raft0.HandleAppendEntriesRequest(req)
	if raft0.state != Follower {
		t.Errorf("expected becomeFollower on higher term; got state=%v", raft0.state)
	}
	if rep.Success != true {
		t.Error("expected Success=true for higher term")
	}
	if raft0.leaderId != "node-02" {
		t.Errorf("expected leaderId updated to node-02; got %s", raft0.leaderId)
	}
}

//// テストハーネスを使ったテスト ////

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
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

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

// コマンドのコミットテスト
func TestRaftCommitOneCommand(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, 3)
	defer h.Shutdown()

	// リーダーを待機
	leaderId, _ := h.CheckSingleLeader()

	// リーダーにコマンドを送信
	isLeader, err := h.SubmitCommand(leaderId, "cmd-test")
	if err != nil {
		t.Errorf("failed to submit command: %v", err)
	}
	if !isLeader {
		t.Error("expected node to be leader")
	}

	time.Sleep(150 * time.Millisecond)
	// 全ノードでコミットされていることを確認
	h.CheckCommittedN("cmd-test", 3)
}

// 非リーダーへのサブミットテスト
func TestRaftSubmitNonLeaderFails(t *testing.T) {
	h := NewHarness(t, 3)
	defer h.Shutdown()

	leaderId, _ := h.CheckSingleLeader()
	nonLeaderId := 0
	if h.nodeIds[nonLeaderId] == leaderId {
		nonLeaderId = 1
	}

	// 非リーダーにコマンドを送信
	isLeader, err := h.raftNodes[nonLeaderId].Submit("cmd-test")
	if err == nil {
		t.Error("expected error when submitting to non-leader")
	}
	if isLeader {
		t.Error("expected node to not be leader")
	}
}

// // 複数コマンドのコミットテスト
func TestRaftCommitMultipleCommands(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, 3)
	defer h.Shutdown()

	// リーダーを待機
	leaderId, _ := h.CheckSingleLeader()

	values := []string{"cmd-test-1", "cmd-test-2", "cmd-test-3"}
	for _, v := range values {
		isLeader, err := h.SubmitCommand(leaderId, v)
		if err != nil {
			t.Errorf("failed to submit command %s: %v", v, err)
		}
		if !isLeader {
			t.Error("expected node to be leader")
		}
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(150 * time.Millisecond)
	// 全ノードで全てのコマンドがコミットされていることを確認
	for _, node := range h.raftNodes {
		if node.commitIndex != uint64(len(values)-1) || !node.hasCommited {
			t.Errorf("expected commitIndex=%d, got %d", len(values)-1, node.commitIndex)
		}
	}
}

// // ノード切断と再接続時のコミットテスト
func TestRaftCommitWithDisconnectionAndRecover(t *testing.T) {
	defer leaktest.CheckTimeout(t, 100*time.Millisecond)()

	h := NewHarness(t, 3)
	defer h.Shutdown()

	leaderId, _ := h.CheckSingleLeader()

	// 最初のコマンドを送信
	isLeader, err := h.SubmitCommand(leaderId, "cmd-test-1")
	if err != nil {
		t.Errorf("failed to submit first command: %v", err)
	}
	if !isLeader {
		t.Error("expected node to be leader")
	}

	time.Sleep(250 * time.Millisecond)
	// 全ノードでコミットされていることを確認
	for _, node := range h.raftNodes {
		if node.commitIndex != 0 || !node.hasCommited {
			t.Errorf("expected node.hasCommited=true and node.commitIndex=0, got %v and %d", node.hasCommited, node.commitIndex)
		}
	}

	// 1つのノードを切断
	dPeerId := 0
	if h.nodeIds[dPeerId] == leaderId {
		dPeerId = 1
	}
	h.DisconnectNode(h.nodeIds[dPeerId])
	time.Sleep(250 * time.Millisecond)

	// 新しいコマンドを送信
	isLeader, err = h.SubmitCommand(leaderId, "cmd-test-2")
	if err != nil {
		t.Errorf("failed to submit second command: %v", err)
	}
	if !isLeader {
		t.Error("expected node to be leader")
	}

	time.Sleep(250 * time.Millisecond)
	// 接続中のノードでのみコミットされていることを確認
	connectedNodes := 0
	for _, node := range h.raftNodes {
		if node.commitIndex == 1 {
			connectedNodes++
		}
	}
	if connectedNodes != 2 {
		t.Errorf("expected 2 nodes to have commitIndex=1, got %d", connectedNodes)
	}

	// 切断したノードを再接続
	h.ReconnectNode(h.nodeIds[dPeerId])

	// // 全ノードでコミットされていることを確認
	tutl.WaitForCondition(t, time.Second*10, time.Millisecond*100, func() bool {
		for _, node := range h.raftNodes {
			if node.commitIndex != 1 {
				return false
			}
		}
		return true
	}, "not all nodes have committed the second command")
}
