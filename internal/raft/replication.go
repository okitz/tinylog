package raft

import (
	"fmt"

	raft_v1 "github.com/okitz/mqtt-log-pipeline/api/raft"
)

func (r *Raft) Submit(command string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == Leader {
		le := raft_v1.LogEntry{
			Command: command,
			Term:    r.currentTerm,
		}
		data, err := le.MarshalVT()
		if err != nil {
			fmt.Println("Error marshalling log entry:", err)
			return false, err
		}
		_, err = r.raftState.log.Append(data)
		if err != nil {
			fmt.Println("Error appending log entry:", err)
			return false, err
		}
		return true, nil
	}
	return false, fmt.Errorf("not leader")
}
