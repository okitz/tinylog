package testutil

import (
	"testing"
	"time"
)

func WaitForCondition(t *testing.T, timeout time.Duration, interval time.Duration, condition func() bool, failMsg string) {
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
