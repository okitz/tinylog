package raft

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
	logpkg "github.com/okitz/mqtt-log-pipeline/internal/log"
	logger "github.com/okitz/mqtt-log-pipeline/internal/logger"
)

var dlog *logger.Logger

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
	hasCommited bool // true if no commit has been made yet
	commitIndex uint64
	lastApplied uint64
	nextIndex   []uint64
}

type Raft struct {
	raftState
	state              RState
	Id                 string
	peers              []string // 他のノードのIDリスト
	leaderId           string
	lastElectionReset  time.Time
	rpcClt             RPCClient
	mu                 sync.Mutex
	commitChan         chan<- raft_v1.CommitEntry // コミットされたエントリを通知するチャネル
	newCommitReadyChan chan struct{}              // 新しいコミットが準備できたことを通知するチャネル
}

func newRaftState(log *logpkg.Log, peers []string) *raftState {
	return &raftState{
		log:         log,
		currentTerm: 0,
		votedFor:    NoVote,
		hasCommited: false, // 初期状態ではコミットはされていない
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make([]uint64, len(peers)),
	}
}

func NewRaft(id string, log *logpkg.Log, peers []string, rpcClt RPCClient, commitChan chan<- raft_v1.CommitEntry) *Raft {
	dlog = logger.New(id, logger.DebugLevel)
	raftState := newRaftState(log, peers)
	dlog.Info("New Raft instance: id=%s, peers=%v", id, peers)
	r := &Raft{
		raftState:          *raftState,
		Id:                 id,
		peers:              peers,
		state:              Follower,
		rpcClt:             rpcClt,
		commitChan:         commitChan,
		newCommitReadyChan: make(chan struct{}, 16),
	}

	go func() {
		time.Sleep(time.Duration(100 * time.Millisecond)) // 遅延を追加
		r.mu.Lock()
		r.lastElectionReset = time.Now()
		r.mu.Unlock()
		r.runElectionTimer()
	}()
	go r.commitChanSender()

	return r
}

func (r *Raft) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == Dead {
		return
	}
	dlog.Info("Stopped, state=%s", r.state)
	r.state = Dead
	close(r.newCommitReadyChan)
}

func (r *Raft) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == Dead {
		dlog.Info("Resumed as Follower")
		r.newCommitReadyChan = make(chan struct{}, 16)
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
	if r.state == Dead {
		return
	}
	r.state = Follower
	r.currentTerm = term
	r.votedFor = NoVote
	r.lastElectionReset = time.Now()
	dlog.Info("become Follower, new term=%d", term)

	go r.runElectionTimer()
}

func (r *Raft) lastLogIndexAndTerm() (bool, uint64, uint64) {
	nextLogIndex := r.log.NextIndex()
	if nextLogIndex == 0 {
		return false, 0, 0
	}
	lastLogIndex := nextLogIndex - 1
	lastLogEntry, err := r.readLogEntryAt(lastLogIndex)
	if err != nil {
		return true, 0, 0
	}
	return true, lastLogIndex, lastLogEntry.GetTerm()
}

// 候補者に投票 or 拒否する
func (r *Raft) HandleRequestVoteRequest(req *raft_v1.RequestVoteRequest) (*raft_v1.RequestVoteReply, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == Dead {
		return nil, fmt.Errorf("cannot handle RequestVoteRequest in dead state")
	}
	// if r.Id == req.CandidateId {
	// 	return nil, fmt.Errorf("cannot handle RequestVoteRequest in dead state")
	// }
	isLogStored, lastLogIndex, lastLogTerm := r.lastLogIndexAndTerm()
	dlog.Debug("HandleRequestVoteRequest: term=%d, candidate=%s", req.Term, req.CandidateId)

	if req.Term > r.currentTerm {
		r.becomeFollower(req.Term)
	}

	// lastLogIndex, lastLogTerm := r.getLastLog()
	res := &raft_v1.RequestVoteReply{
		Term:        r.currentTerm,
		VoteGranted: false,
	}
	if req.Term == r.currentTerm &&
		(r.votedFor == NoVote || r.votedFor == req.CandidateId) &&
		(!isLogStored ||
			req.LastLogTerm > lastLogTerm ||
			(req.LastLogTerm == lastLogTerm && req.LastLogIndex >= lastLogIndex)) {
		r.votedFor = req.CandidateId
		r.lastElectionReset = time.Now()
		res.VoteGranted = true
		dlog.Info("Vote granted to candidate=%s", req.CandidateId)
	} else {
		dlog.Info("Vote denied for candidate=%s", req.CandidateId)
	}

	return res, nil
}

func (r *Raft) checkAppendEntiresRequestAcceptable(
	req *raft_v1.AppendEntriesRequest,
) bool {
	// 一つもログが保存されていない場合
	if !req.FollowerHasEntries {
		return true
	}
	// PrevLogIndex, PrevLogTerm が現在のログと整合するか
	if req.PrevLogIndex >= r.log.NextIndex() {
		return false
	}
	prevEntry, err := r.readLogEntryAt(req.PrevLogIndex)
	if err != nil {
		dlog.Error(err, "Error reading log entry at PrevLogIndex")
		return false
	}
	return prevEntry.GetTerm() == req.PrevLogTerm
}

func (r *Raft) HandleAppendEntriesRequest(req *raft_v1.AppendEntriesRequest) (*raft_v1.AppendEntriesReply, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == Dead {
		return nil, fmt.Errorf("cannot handle AppendEntriesRequest in dead state")
	}
	dlog.Debug("handling AppendEntriesRequest: term=%d, leader=%s", req.Term, req.LeaderId)
	if req.Term > r.currentTerm {
		r.becomeFollower(req.Term)
	}

	rep := &raft_v1.AppendEntriesReply{
		Term:    r.currentTerm,
		Success: false,
	}
	if req.Term == r.currentTerm {
		dlog.Debug("AppendEntriesRequest: same term, processing")
		if r.state != Follower {
			r.becomeFollower(req.Term)
		}
		r.lastElectionReset = time.Now()
		r.leaderId = req.LeaderId
		if r.checkAppendEntiresRequestAcceptable(req) {
			rep.Success = true

			// どこからログを新規追加すべきか決定
			var logInsertIndex uint64
			if req.FollowerHasEntries {
				logInsertIndex = req.PrevLogIndex + 1
			} else {
				logInsertIndex = 0
			}
			newEntriesIndex := 0
			for {
				if logInsertIndex >= r.log.NextIndex() || newEntriesIndex >= len(req.Entries) {
					break
				}
				insertEntry, err := r.readLogEntryAt(logInsertIndex)
				if err != nil {
					dlog.Error(err, "Error reading log entry at PrevLogIndex")
					return nil, err
				}
				if insertEntry.GetTerm() != req.Entries[newEntriesIndex].Term {
					break
				}
				logInsertIndex++
				newEntriesIndex++
			}

			if len(req.Entries) == 0 {
				// リーダーからのハートビート
				dlog.Debug("AppendEntriesRequest: empty entries, it's a heartbeat")
			} else {
				// ログを追加
				insertingCommands := make([]string, 0, len(req.Entries)-newEntriesIndex)
				for _, entry := range req.Entries[newEntriesIndex:] {
					insertingCommands = append(insertingCommands, entry.Command)
				}
				dlog.Debug("inserting entries %v from index %d", insertingCommands, logInsertIndex)
				for i := newEntriesIndex; i < len(req.Entries); i++ {
					data, err := req.Entries[i].MarshalVT()
					if err != nil {
						dlog.Error(err, "Error writing log entry")
						return nil, err
					}
					if _, err = r.log.Append(data); err != nil {
						dlog.Error(err, "Error appending log entry")
						return nil, err
					}
				}
			}

			// CommitIndexを設定
			// req.LeaderCommit = r.commitIndex = 0 の場合、noCommitを参照
			if req.LeaderHasComitted && (!r.hasCommited || req.LeaderCommit > r.commitIndex) {
				r.commitIndex = min(req.LeaderCommit, r.log.NextIndex()-1)
				dlog.Debug("setting commitIndex=%d", r.commitIndex)
				r.newCommitReadyChan <- struct{}{}
			}
		}
	} else {
		dlog.Debug("AppendEntriesRequest: different term, rejecting")
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
			dlog.Debug("election timeout (last reset at %d), starting election", r.lastElectionReset.UnixMilli())
			r.mu.Unlock()
			r.startElection()
			return
		}

		r.mu.Unlock()
	}
}

func (r *Raft) startElection() error {
	r.mu.Lock()
	if r.state == Dead {
		return fmt.Errorf("cannot start election in dead state")
	}
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	r.mu.Lock()
	r.state = Candidate
	r.currentTerm++
	r.votedFor = r.Id
	r.lastElectionReset = time.Now()
	voteCount := 1
	isLogStored, lastLogIndex, lastLogTerm := r.lastLogIndexAndTerm()
	dlog.Info(r.Id, "starting Election: term=", r.currentTerm)

	req := &raft_v1.RequestVoteRequest{
		Term:         r.currentTerm,
		CandidateId:  r.Id,
		IsLogStored:  isLogStored,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
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
	for rawRep := range repCh {
		r.mu.Lock()
		if r.state != Candidate {
			r.mu.Unlock()
			return nil
		}
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			dlog.Debug("waiting for vote timeout")
			go r.runElectionTimer()
			return ctx.Err()
		default:
			repJSON := rawRep.Raw()
			repErr := rawRep.Error()
			if len(repJSON) == 0 {
				continue
			}
			if repErr != nil {
				dlog.Error(repErr, "Error in RequestVote response")
				return repErr
			}
			rep := &raft_v1.RequestVoteReply{}
			if err := rep.UnmarshalJSON(repJSON); err != nil {
				return err
			}
			dlog.Debug("Received vote reply: granted=%v", rep.VoteGranted)
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
		}

	}
	dlog.Debug("No more responses for RequestVote")
	go r.runElectionTimer()
	return nil
}

func (r *Raft) becomeLeader() error {
	if r.state == Dead {
		return fmt.Errorf("cannot become leader in dead state")
	}

	dlog.Info("become Leader, term=%d", r.currentTerm)
	r.state = Leader
	r.leaderId = r.Id
	for peerIdx := range r.peers {
		r.nextIndex[peerIdx] = r.log.NextIndex()
	}

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		// リーダーとしてハートビートを送信
		for {
			if err := r.sendLeaderHeartbeats(); err != nil {
				dlog.Error(err, "Error sending leader heartbeats")
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
	return nil
}

func (r *Raft) readLogEntryAt(index uint64) (*raft_v1.LogEntry, error) {
	rval, err := r.log.Read(index)
	if err != nil {
		dlog.Error(err, "Error reading log for AppendEntries")
		return nil, err
	}
	var entry raft_v1.LogEntry
	if err := entry.UnmarshalVT(rval.GetValue()); err != nil {
		dlog.Error(err, "Error unmarshalling log entry")
		return nil, err
	}
	return &entry, nil
}

func (r *Raft) readLogEntryFrom(index uint64) ([]*raft_v1.LogEntry, error) {
	records, err := r.log.ReadFrom(index)
	if err != nil {
		dlog.Error(err, "Error reading log entries from index %d", index)
		return nil, err
	}
	entries := make([]*raft_v1.LogEntry, len(records))
	for i, record := range records {
		var entry raft_v1.LogEntry
		if err := entry.UnmarshalVT(record.GetValue()); err != nil {
			dlog.Error(err, "Error unmarshalling log entry at index %d", index+uint64(i))
			return nil, err
		}
		entries[i] = &entry
	}
	return entries, nil
}

func (r *Raft) commitChanSender() error {
	for range r.newCommitReadyChan {
		// Find which entries we have to apply.
		r.mu.Lock()
		savedTerm := r.currentTerm
		savedLastApplied := r.lastApplied
		var nextApplyIndex uint64
		if r.hasCommited {
			nextApplyIndex = r.lastApplied + 1
		} else {
			nextApplyIndex = 0
		}

		var entries []*raft_v1.LogEntry
		if !r.hasCommited || r.commitIndex > r.lastApplied {
			readEntries, err := r.readLogEntryFrom(nextApplyIndex)
			if err != nil {
				dlog.Error(err, "Error reading log entries for commitChanSender")
				r.mu.Unlock()
				return err
			}
			entries = readEntries[:r.commitIndex-nextApplyIndex+1]
			r.lastApplied = r.commitIndex
			r.hasCommited = true
		}
		r.mu.Unlock()
		dlog.Info("commiting entries=%v, savedLastApplied=%d", entries, savedLastApplied)

		for i, entry := range entries {
			r.commitChan <- raft_v1.CommitEntry{
				Command: entry.Command,
				Index:   savedLastApplied + uint64(i) + 1,
				Term:    savedTerm,
			}
		}
	}
	return nil
}

func (r *Raft) updateCommitIndex() error {

	var nextCommitIndex uint64
	if r.hasCommited {
		nextCommitIndex = r.commitIndex + 1
	} else {
		nextCommitIndex = 0
	}
	if r.log.NextIndex() == 0 || nextCommitIndex >= r.log.NextIndex() {
		dlog.Debug("No log entries to commit, skipping updateCommitIndex")
		return nil
	}

	entries, err := r.readLogEntryFrom(nextCommitIndex)
	if err != nil {
		dlog.Error(err, "Error reading log entries for commit index update:")
		return err
	}
	HasUpdated := false
	for i, entry := range entries {
		index := nextCommitIndex + uint64(i)
		if entry.GetTerm() == r.currentTerm {
			matchCount := 1
			for peerIdx := range r.peers {
				if r.nextIndex[peerIdx] > index {
					matchCount++
				}
			}
			if matchCount*2 > len(r.peers)+1 {
				HasUpdated = true
				r.commitIndex = index
			}
		}
	}
	if HasUpdated {
		dlog.Debug("leader sets commitIndex=%d", r.commitIndex)
		r.newCommitReadyChan <- struct{}{}
	}
	return nil
}

func (r *Raft) sendLeaderHeartbeats() error {
	r.mu.Lock()
	if r.state != Leader {
		r.mu.Unlock()
		return nil
	}
	savedCurrentTerm := r.currentTerm
	r.mu.Unlock()
	dlog.Debug("sending Leader heartbeats: term=%d ", r.currentTerm)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	for peerIdx, peerId := range r.peers {
		go func(peerId string) {
			r.mu.Lock()
			// niの初期値は0
			ni := r.nextIndex[peerIdx]
			// フォロワーが一つ以上のログを持っているか確認
			followerHasEntries := ni > 0
			var prevLogIndex uint64
			var prevLogTerm uint64
			if followerHasEntries {
				prevLogIndex = ni - 1
				prevLogEntry, _ := r.readLogEntryAt(prevLogIndex)
				prevLogTerm = prevLogEntry.GetTerm()
			}
			entries := []*raft_v1.LogEntry{}
			if 0 < r.log.NextIndex() && ni < r.log.NextIndex() {
				var err error
				entries, err = r.readLogEntryFrom(ni)
				if err != nil {
					dlog.Error(err, "Error reading log entries for AppendEntries")
					r.mu.Unlock()
					return
				}
			}

			req := &raft_v1.AppendEntriesRequest{
				Term:               r.currentTerm,
				LeaderId:           r.Id,
				PrevLogIndex:       prevLogIndex,
				PrevLogTerm:        prevLogTerm,
				Entries:            entries,
				LeaderCommit:       r.commitIndex,
				FollowerHasEntries: followerHasEntries,
				LeaderHasComitted:  r.hasCommited,
			}

			reqJSON, err := req.MarshalJSON()
			if err != nil {
				dlog.Error(err, "Error marshalling AppendEntriesRequest")
				r.mu.Unlock()
				return
			}
			r.mu.Unlock()

			repJSON, err := r.rpcClt.CallRPC(ctx, peerId, "raft.AppendEntries", reqJSON)
			if err != nil {
				dlog.Warn("Error calling AppendEntries on peer %s: %s", peerId, err)
				return
			}
			if len(repJSON) == 0 {
				dlog.Debug("Empty response from AppendEntries on peer %s", peerId)
				return
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			rep := &raft_v1.AppendEntriesReply{}
			if err := rep.UnmarshalJSON(repJSON); err != nil {
				dlog.Error(err, "Error unmarshalling AppendEntriesReply from peer %s", peerId)
				return
			}
			if rep.GetTerm() > savedCurrentTerm {
				dlog.Debug("Peer %s has higher term %d than current term %d", peerId, rep.GetTerm(), r.currentTerm)
				r.becomeFollower(rep.GetTerm())
				return
			}
			if r.state == Leader && rep.GetTerm() == savedCurrentTerm {
				if rep.GetSuccess() {
					dlog.Debug("AppendEntries reply from %s: successed, term=%d", peerId, rep.GetTerm())
					r.nextIndex[peerIdx] = ni + uint64(len(entries))

					r.updateCommitIndex()
				} else {
					r.nextIndex[peerIdx] = ni - 1
					dlog.Debug("Peer %s rejected AppendEntries with term %d, nextIndex decremented to %d", peerId, rep.GetTerm(), r.nextIndex[peerIdx])
				}
			}
		}(peerId)
	}
	return nil
}
