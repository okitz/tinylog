package log

import (
	"io"
	"testing"

	log_v1 "github.com/okitz/tinylog/api/log"
	tutl "github.com/okitz/tinylog/internal/testutil"
)

func TestSegment(t *testing.T) {
	createTestFS(t)
	defer unmount()
	tutl.Require_NotNil(t, fs)

	value := []byte("hello world")
	want := &log_v1.Record{Value: value}

	c := Config{}
	c.Segment.MaxStoreBytes = 1024
	c.Segment.MaxIndexBytes = entWidth * 3
	dir := "tmp"
	fs.Mkdir(dir, 0000)
	s, err := newSegment(fs, dir, 16, c)
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(16), s.nextOffset)
	tutl.Require_False(t, s.IsMaxed())

	for i := uint64(0); i < 3; i++ {
		off, err := s.Append(value)
		tutl.Require_NoError(t, err)
		tutl.Require_Equal(t, 16+i, off)
		got, err := s.Read(off)
		tutl.Require_NoError(t, err)
		tutl.Require_ByteEqual(t, want.Value, got.Value)
	}

	_, err = s.Append(value)
	tutl.Require_Equal(t, io.EOF, err)

	// maxed index
	tutl.Require_True(t, s.IsMaxed())
	c.Segment.MaxStoreBytes = uint64(len(want.Value) * 3)
	c.Segment.MaxIndexBytes = 1024

	s.Sync()
	s, err = newSegment(fs, dir, 16, c)
	tutl.Require_NoError(t, err)
	// maxed store
	tutl.Require_True(t, s.IsMaxed())

	err = s.Remove()
	tutl.Require_NoError(t, err)
	s, err = newSegment(fs, dir, 16, c)
	tutl.Require_NoError(t, err)
	tutl.Require_False(t, s.IsMaxed())
}
