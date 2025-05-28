package raft

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
	logpkg "github.com/okitz/mqtt-log-pipeline/internal/log"
)

type RState int

const (
	Leader RState = iota
	Candidate
	Follower
	Dead
)

const NoVote = ""

func (s RState) String() string {
	switch s {
	case Leader:
		return "Leader"
	case Candidate:
		return "Candidate"
	case Follower:
		return "Follower"
	case Dead:
		return "Dead"
	default:
		panic("Unknown server state")

	}
}

type raftState struct {
	currentTerm uint64
	votedFor    string // nil indicates no vote has been cast
	log         *logpkg.Log
	commitIndex uint64
	lastApplied uint64
	nextIndex   []uint64
	matchIndex  []uint64
}

type Raft struct {
	raftState
	state             RState
	Id                string
	peers             []string // 他のノードのIDリスト
	leaderId          string
	lastElectionReset time.Time
	rpcClt            RPCClient
	mu                sync.Mutex
}

func newRaftState(log *logpkg.Log, peers []string) *raftState {
	return &raftState{
		log:         log,
		currentTerm: 0,
		votedFor:    NoVote,
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make([]uint64, len(peers)),
		matchIndex:  make([]uint64, len(peers)),
	}
}

func NewRaft(id string, log *logpkg.Log, peers []string, rpcClt RPCClient) *Raft {
	raftState := newRaftState(log, peers)
	fmt.Println("NewRaft: id=", id)
	r := &Raft{
		raftState: *raftState,
		Id:        id,
		peers:     peers,
		state:     Follower,
		rpcClt:    rpcClt,
	}

	go func() {
		time.Sleep(time.Duration(100 * time.Millisecond)) // 遅延を追加
		r.mu.Lock()
		r.lastElectionReset = time.Now()
		r.mu.Unlock()
		r.runElectionTimer()
	}()

	return r
}

func (r *Raft) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Println(r.Id, "Stop: state=", r.state)
	r.state = Dead
}

func (r *Raft) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == Dead {
		fmt.Println(r.Id, "Resume: state was Dead, resuming as Follower")
		r.state = Follower
		r.becomeFollower(r.currentTerm)
	}
}

func (r *Raft) GetNodeInfo() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()

	return map[string]any{
		"id":                r.Id,
		"state":             r.state.String(),
		"currentTerm":       r.currentTerm,
		"votedFor":          r.votedFor,
		"leaderId":          r.leaderId,
		"lastElectionReset": r.lastElectionReset.UnixMilli(),
	}
}

func (r *Raft) becomeFollower(term uint64) {
	fmt.Println(r.Id, "becomeFollower: new term=", term)
	if r.state == Dead {
		return
	}
	r.state = Follower
	r.currentTerm = term
	r.votedFor = NoVote
	r.lastElectionReset = time.Now()

	go r.runElectionTimer()
}

// func (r *Raft) getLastLog() (uint64, uint64) {
// 	lastRecordOff, err := r.log.HighestOffset()
// 	if err != nil || lastRecordOff == 0 {
// 		return 0, 0
// 	}
// 	lastRecord, err := r.log.Read(lastRecordOff)
// 	if err != nil {
// 		return 0, 0
// 	}
// 	return lastRecord.Value.Offset, lastRecord.Value.Term
// }

// 候補者に投票 or 拒否する
func (r *Raft) HandleRequestVoteRequest(req *raft_v1.RequestVoteRequest) (*raft_v1.RequestVoteReply, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == Dead || r.Id == req.CandidateId {
		return nil, nil
	}
	fmt.Println(r.Id, "HandleRequestVoteRequest: term=", req.Term, "candidate=", req.CandidateId)

	if req.Term > r.currentTerm {
		r.becomeFollower(req.Term)
	}

	// lastLogIndex, lastLogTerm := r.getLastLog()
	res := &raft_v1.RequestVoteReply{
		Term:        r.currentTerm,
		VoteGranted: false,
	}
	if req.Term == r.currentTerm &&
		(r.votedFor == NoVote || r.votedFor == req.CandidateId) {
		// &&(req.LastLogTerm > lastLogTerm ||
		// 	(req.LastLogTerm == lastLogTerm && req.LastLogIndex >= lastLogIndex)) {
		r.votedFor = req.CandidateId
		r.lastElectionReset = time.Now()
		res.VoteGranted = true
		fmt.Println(r.Id, "Vote granted to candidate=", req.CandidateId)
	} else {
		fmt.Println(r.Id, "Vote denied for candidate=", req.CandidateId)
	}

	return res, nil
}

func (r *Raft) HandleAppendEntriesRequest(req *raft_v1.AppendEntriesRequest) (*raft_v1.AppendEntriesReply, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == Dead || r.Id == req.LeaderId {
		return nil, nil
	}
	fmt.Println(r.Id, "HandleAppendEntriesRequest: term=", req.Term, "leader=", req.LeaderId)
	if req.Term > r.currentTerm {
		r.becomeFollower(req.Term)
	}

	rep := &raft_v1.AppendEntriesReply{
		Term:    r.currentTerm,
		Success: false,
	}
	if req.Term == r.currentTerm {
		fmt.Println(r.Id, "AppendEntriesRequest: same term, processing")
		if r.state == Candidate {
			r.becomeFollower(req.Term)
		}
		r.lastElectionReset = time.Now()
		r.leaderId = req.LeaderId
		rep.Success = true
	} else {
		fmt.Println(r.Id, "AppendEntriesRequest: different term, rejecting")
	}

	return rep, nil
}

func (r *Raft) runElectionTimer() {
	timeout := time.Duration(150+rand.Intn(150)) * time.Millisecond
	termStarted := r.currentTerm
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C
		r.mu.Lock()

		if r.state == Leader || r.state == Dead || termStarted != r.currentTerm {
			r.mu.Unlock()
			return
		}
		if time.Since(r.lastElectionReset) >= timeout {
			fmt.Printf("%s Election timeout; last reset at %d, now at %d; starting election\n", r.Id, r.lastElectionReset.UnixMilli(), time.Now().UnixMilli())
			r.mu.Unlock()
			r.startElection()
			return
		}

		r.mu.Unlock()
	}
}

func (r *Raft) startElection() error {
	if r.state == Dead {
		return fmt.Errorf("cannot start election in dead state")
	}
	fmt.Println(r.Id, "startElection: term=", r.currentTerm+1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	r.mu.Lock()
	r.state = Candidate
	r.currentTerm++
	r.votedFor = r.Id
	r.lastElectionReset = time.Now()
	voteCount := 1
	replyCount := 0

	req := &raft_v1.RequestVoteRequest{
		Term:         r.currentTerm,
		CandidateId:  r.Id,
		LastLogIndex: 0, // TODO: 最後のログのインデックスを取得
		LastLogTerm:  0, // TODO: 最後のログのタームを取得
	}
	r.mu.Unlock()

	reqJSON, err := req.MarshalJSON()
	if err != nil {
		return err
	}
	var repCh <-chan RPCResopnse
	if repCh, err = r.rpcClt.BroadcastRPC(ctx, "raft.RequestVote", reqJSON); err != nil {
		return err
	}

	for {
		r.mu.Lock()
		if r.state != Candidate {
			r.mu.Unlock()
			return nil
		}
		r.mu.Unlock()

		select {
		case rawRep, ok := <-repCh:
			if !ok {
				fmt.Println(r.Id, "No more responses for RequestVote")
				go r.runElectionTimer()
				return nil
			}
			repJSON := rawRep.Raw()
			repErr := rawRep.Error()
			if len(repJSON) == 0 {
				continue
			}
			if repErr != nil {
				fmt.Println(r.Id, "Error in RequestVote response:", repErr)
				return repErr
			}
			if bytes.Equal(bytes.TrimSpace(repJSON), []byte("null")) {
				// 返信元が Dead になっているが通信はできた場合、
				// 返信が空になる (テスト中にのみ発生？)
				continue
			}
			rep := &raft_v1.RequestVoteReply{}
			if err := rep.UnmarshalJSON(repJSON); err != nil {
				return err
			}

			fmt.Println(r.Id, "Received vote reply: granted=", rep.VoteGranted)
			if rep.Term > r.currentTerm {
				r.becomeFollower(rep.Term)
				return nil
			}
			if rep.VoteGranted {
				voteCount++

				// 選挙に勝利
				if voteCount*2 > len(r.peers)+1 {
					r.becomeLeader()
					return nil
				}
			}

			replyCount++
			if replyCount == len(r.peers) {
				// 全員から応答あり
				// deferでctxが閉じる
				return nil
			}
		case <-ctx.Done():
			fmt.Println(r.Id, "startElection timeout")
			go r.runElectionTimer()
			return ctx.Err()
			// default:
			// fmt.Println(r.Id, "startElection: still waiting for votes")
		}

	}

}

func (r *Raft) becomeLeader() error {
	if r.state == Dead {
		return fmt.Errorf("cannot become leader in dead state")
	}
	fmt.Println(r.Id, "becomeLeader: id=", r.Id)
	r.state = Leader
	r.leaderId = r.Id

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		// リーダーとしてハートビートを送信
		for {
			if err := r.sendLeaderHeartbeats(); err != nil {
				fmt.Println(r.Id, "Error sending leader heartbeats:", err)
			}
			<-ticker.C

			r.mu.Lock()
			if r.state != Leader {
				r.mu.Unlock()
				return
			}
			r.mu.Unlock()
		}
	}()

	// if err := <-errCh; err != nil {
	// 	return fmt.Errorf("failed to become leader: %w", err)
	// }
	return nil
}

func (r *Raft) sendLeaderHeartbeats() error {
	if r.state == Dead {
		return fmt.Errorf("cannot send heartbeats in dead state")
	}
	fmt.Printf("%s sendLeaderHeartbeats: term=%d, now at %d\n", r.Id, r.currentTerm, time.Now().UnixMilli())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := &raft_v1.AppendEntriesRequest{
		Term:         r.currentTerm,
		LeaderId:     r.Id,
		PrevLogIndex: 0,   // TODO: 前のログのインデックスを取得
		PrevLogTerm:  0,   // TODO: 前のログのタームを取得
		Entries:      nil, // TODO: エントリを追加
		LeaderCommit: r.commitIndex,
	}
	reqJSON, err := req.MarshalJSON()
	if err != nil {
		fmt.Println(r.Id, "Error marshalling AppendEntriesRequest:", err)
		return err
	}
	var repCh <-chan RPCResopnse
	if repCh, err = r.rpcClt.BroadcastRPC(ctx, "raft.AppendEntries", reqJSON); err != nil {
		return err
	}

	replyCount := 0
	for {
		r.mu.Lock()
		if r.state != Leader {
			r.mu.Unlock()
			return nil
		}
		r.mu.Unlock()

		select {
		case rawRep, ok := <-repCh:
			if !ok {
				fmt.Println(r.Id, "No more responses for AppendEntries")
				return nil
			}
			repJSON := rawRep.Raw()
			repErr := rawRep.Error()
			if len(repJSON) == 0 {
				continue
			}
			if repErr != nil {
				fmt.Println(r.Id, "Error in AppendEntries response:", repErr)
				return repErr
			}
			if bytes.Equal(bytes.TrimSpace(repJSON), []byte("null")) {
				// 返信元が Dead になっているが通信はできた場合、
				// 返信が空になる (テスト中にのみ発生？)
				continue
			}
			rep := &raft_v1.AppendEntriesReply{}
			if err := rep.UnmarshalJSON(repJSON); err != nil {
				return fmt.Errorf("failed to unmarshal AppendEntriesReply: %w", err)
			}
			fmt.Printf("%s AppendEntries reply: success=%v, term=%d, now at %d\n", r.Id, rep.Success, rep.Term, time.Now().UnixMilli())
			r.mu.Lock()
			if rep.Term > r.currentTerm {
				r.becomeFollower(rep.Term)
				r.mu.Unlock()
				return nil
			}
			r.mu.Unlock()

			replyCount++
			if replyCount == len(r.peers) {
				// 全員から応答あり
				// deferでctxが閉じる
				fmt.Println(r.Id, "All peers responded to AppendEntries")
				return nil
			}
		case <-ctx.Done():
			fmt.Println(r.Id, "sendLeaderHeartbeats timeout")
			return ctx.Err()
		}
	}

}
