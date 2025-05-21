package log

import (
	"io"
	"testing"

	api "github.com/okitz/mqtt-log-pipeline/api"
	"github.com/stretchr/testify/require"
)

func TestLog(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T, log *Log,
	){
		"append and read a record succeeds": testAppendRead,
		"offset out of range error":         testOutOfRangeErr,
		"init with existing segments":       testInitExisting,
		// "reader":                            testReader,
		// "truncate":                          testTruncate,
	} {
		t.Run(scenario, func(t *testing.T) {
			dir := "tmp"
			// defer os.RemoveAll(dir)

			c := Config{}
			c.Segment.MaxStoreBytes = 32
			createTestFS(t)

			log, err := NewLog(fs, dir, c)
			require.NoError(t, err)

			fn(t, log)
		})
	}
}

func testAppendRead(t *testing.T, log *Log) {
	append := &api.Record{
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
	append := &api.Record{
		Value: []byte("hello world"),
	}
	for i := 0; i < 3; i++ {
		_, err := o.Append(append)
		require.NoError(t, err)
	}
	require.NoError(t, o.Close())

	// off, err := o.LowestOffset()
	// require.NoError(t, err)
	// require.Equal(t, uint64(0), off)
	// off, err = o.HighestOffset()
	// require.NoError(t, err)
	// require.Equal(t, uint64(2), off)

	// n, err := NewLog(fs, o.Dir.Name(), o.Config)
	// require.NoError(t, err)

	// // off, err = n.LowestOffset()
	// // require.NoError(t, err)
	// // require.Equal(t, uint64(0), off)
	// // off, err = n.HighestOffset()
	// // require.NoError(t, err)
	// // require.Equal(t, uint64(2), off)
}

func testReader(t *testing.T, log *Log) {
	append := &api.Record{
		Value: []byte("hello world"),
	}
	off, err := log.Append(append)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	reader := log.Reader()
	b, err := io.ReadAll(reader)
	require.NoError(t, err)

	read := &api.Record{}
	err = read.UnmarshalVT(b[lenWidth:])
	require.NoError(t, err)
	require.Equal(t, append.Value, read.Value)
}

func testTruncate(t *testing.T, log *Log) {
	append := &api.Record{
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
