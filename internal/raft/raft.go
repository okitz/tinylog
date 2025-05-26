package raft

import (
	"context"
	"encoding/json"
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
	peers             []string
	leaderId          string
	lastElectionReset time.Time
	rpcClt            RPCClient
	mu                sync.Mutex
}

type RPCClient interface {
	CallRPC(ctx context.Context, targetId string, method string, reqParams json.RawMessage) (json.RawMessage, error)
	BroadcastRPC(ctx context.Context, method string, reqParams json.RawMessage) (<-chan RPCResopnse, error)
}

type RPCResopnse interface {
	Error() error
	Raw() json.RawMessage
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
	return &Raft{
		raftState:         *raftState,
		Id:                id,
		peers:             peers,
		state:             Follower,
		lastElectionReset: time.Now(),
		rpcClt:            rpcClt,
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
	fmt.Println(r.Id, "HandleRequestVoteRequest: term=", req.Term, "candidate=", req.CandidateId)
	if r.state == Dead || r.Id == req.CandidateId {
		return nil, nil
	}
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
	fmt.Println(r.Id, "HandleAppendEntriesRequest: term=", req.Term, "leader=", req.LeaderId)
	if r.state == Dead || r.Id == req.LeaderId {
		return nil, nil
	}
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
			fmt.Println(r.Id, "Election timeout, starting election")
			r.mu.Unlock()
			r.startElection()
			return
		}

		r.mu.Unlock()
	}
}

func (r *Raft) startElection() error {
	fmt.Println(r.Id, "startElection: term=", r.currentTerm+1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	repCh, err := r.rpcClt.BroadcastRPC(ctx, "RequestVote", reqJSON)
	if err != nil {
		return err
	}

	for {
		select {
		case rawRep, ok := <-repCh:
			if !ok {
				fmt.Println(r.Id, "No more responses for RequestVote")
				return nil
			}
			if rawRep.Error() != nil {
				fmt.Println(r.Id, "Error in RequestVote response:", rawRep.Error())
				return rawRep.Error()
			}
			repJSON := rawRep.Raw()
			if len(repJSON) == 0 {
				continue
			}
			if r.state != Candidate {
				return nil
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
				// deferでctxが閉じる
				return nil // 全員から応答あり
			}
		case <-ctx.Done():
			fmt.Println(r.Id, "startElection timeout")
			return ctx.Err()
		}
	}

}

func (r *Raft) becomeLeader() {
	fmt.Println(r.Id, "becomeLeader: id=", r.Id)
	r.state = Leader
	r.leaderId = r.Id
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		// リーダーとしてハートビートを送信
		for {
			r.sendLeaderHeartbeats()
			<-ticker.C

			r.mu.Lock()
			if r.state != Leader {
				r.mu.Unlock()
				return
			}
			r.mu.Unlock()
		}
	}()
}

func (r *Raft) sendLeaderHeartbeats() {
	fmt.Println(r.Id, "sendLeaderHeartbeats: term=", r.currentTerm)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
		panic(err)
	}
	repCh, err := r.rpcClt.BroadcastRPC(ctx, "AppendEntries", reqJSON)
	if err != nil {
		fmt.Println(r.Id, "Error broadcasting AppendEntries:", err)
		panic(err)
	}

	for {
		select {
		case rawRep, ok := <-repCh:
			if !ok {
				fmt.Println(r.Id, "No more responses for AppendEntries")
				return
			}
			if rawRep.Error() != nil {
				fmt.Println(r.Id, "Error in AppendEntries response:", rawRep.Error())
				panic(rawRep.Error())
			}
			repJSON := rawRep.Raw()
			if len(repJSON) == 0 {
				continue
			}
			rep := &raft_v1.AppendEntriesReply{}
			if err := rep.UnmarshalJSON(repJSON); err != nil {
				panic(err)
			}
			fmt.Println(r.Id, "Received AppendEntries reply: success=", rep.Success)
			r.mu.Lock()
			if rep.Term > r.currentTerm {
				r.becomeFollower(rep.Term)
				r.mu.Unlock()
				return
			}
			r.mu.Unlock()
		case <-ctx.Done():
			fmt.Println(r.Id, "sendLeaderHeartbeats timeout")
			if err := ctx.Err(); err != nil {
				panic(err)
			}

		}
	}

}
