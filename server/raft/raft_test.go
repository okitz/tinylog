package raft

import (
	"fmt"
	"testing"
	"time"

	log_v1 "github.com/okitz/mqtt-log-pipeline/api/log"
	raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
	logpkg "github.com/okitz/mqtt-log-pipeline/server/log"
)

type stubLog struct{}

func (s *stubLog) HighestOffset() (uint64, error) { return 0, nil }
func (s *stubLog) Read(offset uint64) (*log_v1.Record, error) {
	return &log_v1.Record{}, nil
}

func setupFakeCluster(n int64) ([]*Raft, *FakeRPCTransporter) {
	tp := NewFakeRPCTransporter()
	nodes := make([]*Raft, n)
	peers := make([]string, n)
	for i := int64(0); i < n; i++ {
		peers[i] = fmt.Sprintf("node-%02d", i+1)
	}
	for i, nodeId := range peers {
		clt := NewFakeRPCClient(nodeId, tp)
		r := NewRaft(nodeId, (*logpkg.Log)(nil), peers, clt)
		r.becomeFollower(0)
		nodes[i] = r
		tp.RegisterNode(r)
	}
	return nodes, tp
}

func TestHandleRequestVoteRequest(t *testing.T) {
	nodes, _ := setupFakeCluster(1)
	raft := nodes[0]
	raft.currentTerm = 5

	// Term が小さいリクエスト → 拒否、Term は変わらず
	req := &raft_v1.RequestVoteRequest{Term: 4, CandidateId: ""}
	rep, err := raft.HandleRequestVoteRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Term != 5 || rep.VoteGranted {
		t.Errorf("expected Term=5,VoteGranted=false; got Term=%d,VoteGranted=%v", rep.Term, rep.VoteGranted)
	}

	// 同じTerm＆未投票 → 承認
	req = &raft_v1.RequestVoteRequest{Term: 5, CandidateId: "node-02"}
	rep, _ = raft.HandleRequestVoteRequest(req)
	if rep.Term != 5 || !rep.VoteGranted {
		t.Errorf("expected Term=5,VoteGranted=true; got Term=%d,VoteGranted=%v", rep.Term, rep.VoteGranted)
	}
	if raft.votedFor != "node-02" {
		t.Errorf("expected votedFor=node-02; got %s", raft.votedFor)
	}

	// 同じTermでも別の候補者に2度目投票は拒否
	req = &raft_v1.RequestVoteRequest{Term: 5, CandidateId: "node-03"}
	rep, _ = raft.HandleRequestVoteRequest(req)
	if rep.VoteGranted {
		t.Error("expected second vote in same term to be denied")
	}

	// より大きいTerm → フォロワーモード＆投票リセット後に承認
	raft.state = Candidate
	req = &raft_v1.RequestVoteRequest{Term: 6, CandidateId: "node-03"}
	rep, _ = raft.HandleRequestVoteRequest(req)
	if raft.state != Follower {
		t.Errorf("expected becomeFollower on higher term; got state=%v", raft.state)
	}
	if !rep.VoteGranted {
		t.Error("expected vote granted for new term")
	}
}

func TestHandleAppendEntriesRequest(t *testing.T) {
	nodes, _ := setupFakeCluster(1)
	raft := nodes[0]
	raft.currentTerm = 5

	// Term が小さいエントリ → 拒否
	req := &raft_v1.AppendEntriesRequest{Term: 4, LeaderId: "node-02"}
	rep, err := raft.HandleAppendEntriesRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Success {
		t.Error("expected Success=false for lower term")
	}

	// 同じTerm → フォロワーに遷移して承認
	req = &raft_v1.AppendEntriesRequest{Term: 5, LeaderId: "node-02"}
	rep, _ = raft.HandleAppendEntriesRequest(req)
	if rep.Term != 5 || !rep.Success {
		t.Errorf("expected Term=5, Success=true; got Term=%d, Success=%v", rep.Term, rep.Success)
	}
	if raft.state != Follower {
		t.Errorf("expected state=Follower; got %v", raft.state)
	}
	if raft.leaderId != "node-02" {
		t.Errorf("expected leaderId=node-02; got %s", raft.leaderId)
	}

	// より大きいTerm → フォロワー遷移＆承認
	raft.state = Candidate
	req = &raft_v1.AppendEntriesRequest{Term: 6, LeaderId: "node-03"}
	rep, _ = raft.HandleAppendEntriesRequest(req)
	if raft.state != Follower {
		t.Errorf("expected becomeFollower on higher term; got state=%v", raft.state)
	}
	if rep.Success != true {
		t.Error("expected Success=true for higher term")
	}
	if raft.leaderId != "node-03" {
		t.Errorf("expected leaderId updated to node-03; got %s", raft.leaderId)
	}
}

func TestClusterElection(t *testing.T) {
	nodes, _ := setupFakeCluster(3)

	// ノード1で選挙開始
	err := nodes[0].startElection()
	if err != nil {
		t.Fatalf("startElection failed: %v", err)
	}

	// Leaderになっていることを確認
	if nodes[0].state != Leader {
		t.Errorf("expected node1 to become Leader; got %v", nodes[0].state)
	}

}

func TestLeaderStable(t *testing.T) {
	fmt.Println("TestLeaderStable #####################")

	nodes, _ := setupFakeCluster(3)

	// ノード1をLeaderにする
	err := nodes[0].startElection()
	if err != nil {
		t.Fatalf("startElection failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // 少し待つ
	if nodes[0].state != Leader {
		t.Errorf("expected node1 to become Leader; got %v", nodes[0].state)
	}
}

// リーダーがkillされた後、他のノードが新しいリーダーになることを確認するテスト
func TestNewLeaderAfterKill(t *testing.T) {
	fmt.Println("TestNewLeaderAfterKill #####################")
	nodes, tp := setupFakeCluster(5)

	// ノード1をLeaderにする
	err := nodes[0].startElection()
	if err != nil {
		t.Fatalf("startElection failed: %v", err)
	}
	if nodes[0].state != Leader {
		t.Errorf("expected node1 to become Leader; got %v", nodes[0].state)
	}

	// ノード1をkill
	tp.KillNode(nodes[0].Id)
	fmt.Println("Node 1 killed, waiting for new Leader")

	waitForCondition(t, 10*time.Second, 100*time.Millisecond, func() bool {
		lCnt := 0
		for i := 1; i < len(nodes); i++ {
			if nodes[i].state == Leader {
				lCnt++
				fmt.Printf("Node %s is now Leader\n", nodes[i].Id)
			}
		}
		return lCnt == 1
	}, fmt.Sprintf("expected another node to become Leader; got node2=%v, node3=%v, node4=%v, node5=%v",
		nodes[1].state, nodes[2].state, nodes[3].state, nodes[4].state))

	tp.RecoverNode(nodes[0].Id) // ノード1を復活させる
	fmt.Println("Node 1 recovered")

	waitForCondition(t, 10*time.Second, 100*time.Millisecond, func() bool {
		return nodes[0].state == Follower
	}, "expected node1 to become Follower")

}

func waitForCondition(t *testing.T, timeout time.Duration, interval time.Duration, condition func() bool, failMsg string) {
	deadline := time.Now().Add(timeout)
	for {
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout: %s\n", failMsg)
		}
		time.Sleep(interval)
	}
}

// Termが増加することを確認するテスト
func TestTermIncreaseOnElection(t *testing.T) {
	nodes, _ := setupFakeCluster(3)

	initialTerm := nodes[0].currentTerm
	err := nodes[0].startElection()
	if err != nil {
		t.Fatalf("startElection failed: %v", err)
	}

	if nodes[0].currentTerm <= initialTerm {
		t.Errorf("term did not increase after election; got %d, want > %d", nodes[0].currentTerm, initialTerm)
	}
}
