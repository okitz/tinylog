package testutil

import (
	"bytes"
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

func Require_NotEqual(t *testing.T, a, b interface{}) {
	if a == b {
		t.Fatalf("expected not equal: %v, %v", a, b)
	}
}
func Require_Equal(t *testing.T, a, b interface{}) {
	if a != b {
		t.Fatalf("expected equal: %v, %v", a, b)
	}
}
func Require_ByteEqual(t *testing.T, a, b []byte) {
	if !bytes.Equal(a, b) {
		t.Fatalf("expected byte slices not equal: %v, %v", a, b)
	}
}
func Require_Nil[T comparable](t *testing.T, a T) {
	var zero T
	if a != zero {
		t.Fatalf("expected nil, but got: %#v", a)
	}
}
func Require_NotNil[T comparable](t *testing.T, a T) {
	var zero T
	if a == zero {
		t.Fatalf("expected nil, but got: %#v", a)
	}
}
func Require_Error(t *testing.T, err error) {
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
func Require_NoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
func Require_True(t *testing.T, condition bool) {
	if !condition {
		t.Fatal("expected true, got false")
	}
}
func Require_False(t *testing.T, condition bool) {
	if condition {
		t.Fatal("expected false, got true")
	}
}
