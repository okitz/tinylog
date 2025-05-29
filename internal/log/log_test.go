package log

import (
	"fmt"
	"io"
	"strings"
	"testing"

	log_v1 "github.com/okitz/mqtt-log-pipeline/api/log"
	tutl "github.com/okitz/mqtt-log-pipeline/internal/testutil"
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
			tutl.Require_NoError(t, err)

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
	value := []byte("hello world")
	off, err := log.Append(value)
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(0), off)

	read, err := log.Read(off)
	tutl.Require_NoError(t, err)
	tutl.Require_ByteEqual(t, value, read.Value)
}

func testOutOfRangeErr(t *testing.T, log *Log) {
	read, err := log.Read(1)
	tutl.Require_Nil(t, read)
	tutl.Require_Error(t, err)
}

func testInitExisting(t *testing.T, o *Log) {
	value := []byte("hello world")
	for i := 0; i < 3; i++ {
		_, err := o.Append(value)
		tutl.Require_NoError(t, err)
	}
	tutl.Require_NoError(t, o.Close())

	off, err := o.LowestOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(0), off)
	off, err = o.HighestOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(2), off)

	n, err := NewLog(fs, o.Dir.Name(), o.Config)
	tutl.Require_NoError(t, err)
	off, err = n.LowestOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(0), off)
	off, err = n.HighestOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(2), off)
	// lscmd([]string{"ls", "tmp"})

}

func testReader(t *testing.T, log *Log) {
	value := []byte("hello world")
	off, err := log.Append(value)
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(0), off)

	reader := log.Reader()
	b, err := io.ReadAll(reader)
	tutl.Require_NoError(t, err)

	read := &log_v1.Record{}
	err = read.UnmarshalVT(b[lenWidth:])
	tutl.Require_NoError(t, err)
	tutl.Require_ByteEqual(t, value, read.Value)
}

func testTruncate(t *testing.T, log *Log) {
	value := []byte("hello world")
	for i := 0; i < 3; i++ {
		_, err := log.Append(value)
		tutl.Require_NoError(t, err)
	}

	err := log.Truncate(1)
	tutl.Require_NoError(t, err)

	_, err = log.Read(0)
	tutl.Require_Error(t, err)
}

func testRemove(t *testing.T, log *Log) {
	value := []byte("hello world")
	for i := 0; i < 3; i++ {
		_, err := log.Append(value)
		tutl.Require_NoError(t, err)
	}
	err := log.Remove()
	tutl.Require_NoError(t, err)

	err = log.Close()
	tutl.Require_Error(t, err)
}
