package log

import (
	"io"
	"testing"

	log_v1 "github.com/okitz/mqtt-log-pipeline/api/log"
	"github.com/stretchr/testify/require"
)

func TestSegment(t *testing.T) {
	createTestFS(t)
	defer unmount()
	require.NotNil(t, fs)

	want := &log_v1.Record{Value: []byte("hello world")}

	c := Config{}
	c.Segment.MaxStoreBytes = 1024
	c.Segment.MaxIndexBytes = entWidth * 3
	dir := "tmp"
	fs.Mkdir(dir, 0000)
	s, err := newSegment(fs, dir, 16, c)
	require.NoError(t, err)
	require.Equal(t, uint64(16), s.nextOffset, s.nextOffset)
	require.False(t, s.IsMaxed())

	for i := uint64(0); i < 3; i++ {
		off, err := s.Append(want)
		require.NoError(t, err)
		require.Equal(t, 16+i, off)
		got, err := s.Read(off)
		require.NoError(t, err)
		require.Equal(t, want.Value, got.Value)
	}

	_, err = s.Append(want)
	require.Equal(t, io.EOF, err)

	// maxed index
	require.True(t, s.IsMaxed())
	c.Segment.MaxStoreBytes = uint64(len(want.Value) * 3)
	c.Segment.MaxIndexBytes = 1024

	s.Sync()
	s, err = newSegment(fs, dir, 16, c)
	require.NoError(t, err)
	// maxed store
	require.True(t, s.IsMaxed())

	err = s.Remove()
	require.NoError(t, err)
	s, err = newSegment(fs, dir, 16, c)
	require.NoError(t, err)
	require.False(t, s.IsMaxed())
}
