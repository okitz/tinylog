package log

import (
	"fmt"
	"testing"

	tutl "github.com/okitz/tinylog/internal/testutil"
)

func TestLog(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T, log *Log,
	){
		"append and read a record succeeds":              testAppendRead,
		"read from offset 0 to end":                      testReadFrom,
		"offset out of range error":                      testOutOfRangeErr,
		"init with existing segments (access by offset)": testInitExistingOffset,
		"init with existing segments (access by index)":  testInitExistinIndex,
		// "reader":                            testReader,
		// "truncate":                          testTruncate,
		"remove": testRemove,
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

// func lscmd(argv []string) {
// 	path := "/"
// 	if len(argv) > 1 {
// 		path = strings.TrimSpace(argv[1])
// 	}
// 	dir, err := fs.Open(path)
// 	if err != nil {
// 		fmt.Printf("Could not open directory %s: %v\n", path, err)
// 		return
// 	}
// 	defer dir.Close()
// 	infos, err := dir.Readdir(0)
// 	_ = infos
// 	if err != nil {
// 		fmt.Printf("Could not read directory %s: %v\n", path, err)
// 		return
// 	}
// 	for _, info := range infos {
// 		s := "-rwxrwxrwx"
// 		if info.IsDir() {
// 			s = "drwxrwxrwx"
// 		}
// 		fmt.Printf("%s %5d %s\n", s, info.Size(), info.Name())
// 	}
// }

func testAppendRead(t *testing.T, log *Log) {
	value := []byte("hello world")
	idx, err := log.Append(value)
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(0), idx)

	read, err := log.Read(idx)
	tutl.Require_NoError(t, err)
	tutl.Require_ByteEqual(t, value, read.Value)
}

func testReadFrom(t *testing.T, log *Log) {
	values := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		value := []byte(fmt.Sprintf("hello world %d", i))
		values[i] = value
		idx, err := log.Append(value)
		tutl.Require_NoError(t, err)
		tutl.Require_Equal(t, uint64(i), idx)
	}
	readRecords, err := log.ReadFrom(0)
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, len(values), len(readRecords))
	for i, read := range readRecords {
		tutl.Require_ByteEqual(t, values[i], read.Value)
	}
}

func testOutOfRangeErr(t *testing.T, log *Log) {
	read, err := log.Read(1)
	tutl.Require_Nil(t, read)
	tutl.Require_Error(t, err)
}

func testInitExistingOffset(t *testing.T, o *Log) {
	value := []byte("hello world")
	for i := 0; i < 3; i++ {
		_, err := o.Append(value)
		tutl.Require_NoError(t, err)
	}
	tutl.Require_NoError(t, o.Close())

	off, err := o.lowestOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(0), off)
	off, err = o.nextOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(2+1), off)

	n, err := NewLog(fs, o.Dir.Name(), o.Config)
	tutl.Require_NoError(t, err)
	off, err = n.lowestOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(0), off)
	off, err = n.nextOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(2+1), off)
	// lscmd([]string{"ls", "tmp"})

}

func testInitExistinIndex(t *testing.T, o *Log) {
	value := []byte("hello world")
	for i := 0; i < 3; i++ {
		_, err := o.Append(value)
		tutl.Require_NoError(t, err)
	}
	tutl.Require_NoError(t, o.Close())

	idx, err := o.nextOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(2+1), idx)

	n, err := NewLog(fs, o.Dir.Name(), o.Config)
	tutl.Require_NoError(t, err)
	idx, err = n.nextOffset()
	tutl.Require_NoError(t, err)
	tutl.Require_Equal(t, uint64(2+1), idx)
}

// func testReader(t *testing.T, log *Log) {
// 	value := []byte("hello world")
// 	off, err := log.Append(value)
// 	tutl.Require_NoError(t, err)
// 	tutl.Require_Equal(t, uint64(0), off)

// 	reader := log.Reader()
// 	b, err := io.ReadAll(reader)
// 	tutl.Require_NoError(t, err)

// 	read := &log_v1.Record{}
// 	err = read.UnmarshalVT(b[lenWidth:])
// 	tutl.Require_NoError(t, err)
// 	tutl.Require_ByteEqual(t, value, read.Value)
// }

// func testTruncate(t *testing.T, log *Log) {
// 	value := []byte("hello world")
// 	for i := 0; i < 3; i++ {
// 		_, err := log.Append(value)
// 		tutl.Require_NoError(t, err)
// 	}

// 	err := log.Truncate(1)
// 	tutl.Require_NoError(t, err)

// 	_, err = log.Read(0)
// 	tutl.Require_Error(t, err)
// }

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
