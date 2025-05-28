package log

import (
	"io"
	"os"
	"testing"

	filesys "github.com/okitz/mqtt-log-pipeline/internal/filesys"
	tutl "github.com/okitz/mqtt-log-pipeline/internal/testutil"
)

func TestIndex(t *testing.T) {
	createTestFS(t)
	defer unmount()

	tgt := "file1.index"
	f, err := filesys.OpenFile(fs, tgt, os.O_WRONLY|os.O_CREATE)
	tutl.Require_NotNil(t, f)
	tutl.Require_NoError(t, err)

	c := Config{}
	c.Segment.MaxIndexBytes = 1024
	idx, err := newIndex(f, c)
	tutl.Require_NoError(t, err)
	_, _, err = idx.Read(-1)
	tutl.Require_Error(t, err)
	tutl.Require_Equal(t, f.Name(), idx.Name())

	entries := []struct {
		Off uint32
		Pos uint64
	}{
		{Off: 0, Pos: 0},
		{Off: 1, Pos: 10},
	}

	for _, want := range entries {
		err = idx.Write(want.Off, want.Pos)
		tutl.Require_NoError(t, err)

		_, pos, err := idx.Read(int64(want.Off))
		tutl.Require_NoError(t, err)
		tutl.Require_Equal(t, want.Pos, pos)
	}

	// index and scanner should error when reading past existing entries
	_, _, err = idx.Read(int64(len(entries)))
	tutl.Require_Equal(t, io.EOF, err)
	_ = idx.Sync()

	// index should build its state from the existing file
	f, _ = filesys.OpenFile(fs, tgt, os.O_WRONLY|os.O_CREATE)
	idx, err = newIndex(f, c)
	tutl.Require_NoError(t, err)
	off, pos, err := idx.Read(-1)
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint32(1), off)
	tutl.Require_Equal(t, entries[1].Pos, pos)
}
