package log

import (
	"os"
	"testing"

	filesys "github.com/okitz/mqtt-log-pipeline/internal/filesys"
	"github.com/stretchr/testify/require"
	"tinygo.org/x/tinyfs"
	"tinygo.org/x/tinyfs/littlefs"
)

var (
	write = []byte("hello world!")
	width = uint64(len(write)) + lenWidth
)

func TestStoreAppendRead(t *testing.T) {
	createTestFS(t)
	defer unmount()
	require.NotNil(t, fs)
	tgt := "file1.store"
	f, err := filesys.OpenFile(fs, tgt, os.O_WRONLY|os.O_CREATE)
	require.NotNil(t, f)
	require.NoError(t, err)

	c := Config{}
	c.Segment.MaxStoreBytes = 1024
	s, err := newStore(f, c)
	require.NotNil(t, s)
	require.NoError(t, err)
	testAppend(t, s)
	testRead(t, s)
	testReadAt(t, s)
	s.Sync()

	f, _ = filesys.OpenFile(fs, tgt, os.O_WRONLY|os.O_CREATE)
	s2, err := newStore(f, c)
	require.NoError(t, err)
	testRead(t, s2)
}

func testAppend(t *testing.T, s *store) {
	t.Helper()
	for i := uint64(1); i < 4; i++ {
		n, pos, err := s.Append(write)
		require.NoError(t, err)
		require.Equal(t, pos+n, width*i)
	}
}

func testRead(t *testing.T, s *store) {
	t.Helper()
	var pos uint64
	for i := uint64(1); i < 4; i++ {
		read, err := s.Read(pos)
		require.NoError(t, err)
		require.Equal(t, write, read)
		pos += width
	}
}

func testReadAt(t *testing.T, s *store) {
	t.Helper()
	for i, off := uint64(1), int64(0); i < 4; i++ {
		b := make([]byte, lenWidth)
		n, err := s.ReadAt(b, off)
		require.NoError(t, err)
		require.Equal(t, lenWidth, n)
		off += int64(n)

		size := enc.Uint64(b)
		b = make([]byte, size)
		n, err = s.ReadAt(b, off)
		require.NoError(t, err)
		require.Equal(t, write, b)
		require.Equal(t, int(size), n)
		off += int64(n)
	}
}

func TestStoreClose(t *testing.T) {
	createTestFS(t)
	defer unmount()
	require.NotNil(t, fs)
	tgt := "file1.txt"
	f, err := filesys.OpenFile(fs, tgt, os.O_WRONLY|os.O_CREATE)
	defer f.Close()
	require.NotNil(t, f)
	require.NoError(t, err)

	c := Config{}
	c.Segment.MaxStoreBytes = 1024
	s, err := newStore(f, c)
	require.NoError(t, err)
	_, _, err = s.Append(write)
	require.NoError(t, err)

	beforeSize, err := openFile(fs)
	require.NoError(t, err)

	err = s.Close()
	require.NoError(t, err)

	afterSize, err := openFile(fs)
	require.NoError(t, err)
	require.True(t, afterSize > beforeSize)
}

func openFile(fs *littlefs.LFS) (size int64, err error) {
	tgt := "file1.txt"
	f, err := filesys.OpenFile(fs, tgt, os.O_WRONLY|os.O_CREATE)
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size = fi.Size()
	return size, nil
}

var (
	fs      *littlefs.LFS
	bd      *tinyfs.MemBlockDevice
	unmount func()
)

func createTestFS(t *testing.T) {
	// create/format/mount the filesystem
	bd = tinyfs.NewMemoryDevice(64, 256, 2048)
	fs = littlefs.New(bd).Configure(&littlefs.Config{
		//	ReadSize:      16,
		//	ProgSize:      16,
		//	BlockSize:     512,
		//	BlockCount:    1024,
		CacheSize:     128,
		LookaheadSize: 128,
		BlockCycles:   500,
	})
	if err := fs.Format(); err != nil {
		t.Error("Could not format", err)
	}
	if err := fs.Mount(); err != nil {
		t.Error("Could not mount", err)
	}
	unmount = func() {
		if err := fs.Unmount(); err != nil {
			t.Error("Could not ummount", err)
		}
	}
}
