package log

import (
	"fmt"
	"io"
	"strings"
	"testing"

	log_v1 "github.com/okitz/mqtt-log-pipeline/api/log"
	"github.com/stretchr/testify/require"
)

func TestLog(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T, log *Log,
	){
		"append and read a record succeeds": testAppendRead,
		"offset out of range error":         testOutOfRangeErr,
		"init with existing segments":       testInitExisting,
		"reader":                            testReader,
		"truncate":                          testTruncate,
		"remove":                            testRemove,
	} {
		t.Run(scenario, func(t *testing.T) {
			dir := "tmp"

			c := Config{}
			c.Segment.MaxStoreBytes = 32
			createTestFS(t)

			log, err := NewLog(fs, dir, c)
			require.NoError(t, err)

			fn(t, log)
		})
	}
}

func lscmd(argv []string) {
	path := "/"
	if len(argv) > 1 {
		path = strings.TrimSpace(argv[1])
	}
	dir, err := fs.Open(path)
	if err != nil {
		fmt.Printf("Could not open directory %s: %v\n", path, err)
		return
	}
	defer dir.Close()
	infos, err := dir.Readdir(0)
	_ = infos
	if err != nil {
		fmt.Printf("Could not read directory %s: %v\n", path, err)
		return
	}
	for _, info := range infos {
		s := "-rwxrwxrwx"
		if info.IsDir() {
			s = "drwxrwxrwx"
		}
		fmt.Printf("%s %5d %s\n", s, info.Size(), info.Name())
	}
}

func testAppendRead(t *testing.T, log *Log) {
	append := &log_v1.Record{
		Value: []byte("hello world"),
	}
	off, err := log.Append(append)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	read, err := log.Read(off)
	require.NoError(t, err)
	require.Equal(t, append.Value, read.Value)
}

func testOutOfRangeErr(t *testing.T, log *Log) {
	read, err := log.Read(1)
	require.Nil(t, read)
	require.Error(t, err)
}

func testInitExisting(t *testing.T, o *Log) {
	append := &log_v1.Record{
		Value: []byte("hello world"),
	}
	for i := 0; i < 3; i++ {
		_, err := o.Append(append)
		require.NoError(t, err)
	}
	require.NoError(t, o.Close())

	off, err := o.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)
	off, err = o.HighestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), off)

	n, err := NewLog(fs, o.Dir.Name(), o.Config)
	require.NoError(t, err)
	off, err = n.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)
	off, err = n.HighestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), off)
	// lscmd([]string{"ls", "tmp"})

}

func testReader(t *testing.T, log *Log) {
	append := &log_v1.Record{
		Value: []byte("hello world"),
	}
	off, err := log.Append(append)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	reader := log.Reader()
	b, err := io.ReadAll(reader)
	require.NoError(t, err)

	read := &log_v1.Record{}
	err = read.UnmarshalVT(b[lenWidth:])
	require.NoError(t, err)
	require.Equal(t, append.Value, read.Value)
}

func testTruncate(t *testing.T, log *Log) {
	append := &log_v1.Record{
		Value: []byte("hello world"),
	}
	for i := 0; i < 3; i++ {
		_, err := log.Append(append)
		require.NoError(t, err)
	}

	err := log.Truncate(1)
	require.NoError(t, err)

	_, err = log.Read(0)
	require.Error(t, err)
}

func testRemove(t *testing.T, log *Log) {
	append := &log_v1.Record{
		Value: []byte("hello world"),
	}
	for i := 0; i < 3; i++ {
		_, err := log.Append(append)
		require.NoError(t, err)
	}
	err := log.Remove()
	require.NoError(t, err)

	err = log.Close()
	require.Error(t, err)
}
