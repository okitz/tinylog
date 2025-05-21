package log

import (
	"io"
	"sync"

	"tinygo.org/x/tinyfs/littlefs"
)

const (
	offWidth uint64 = 4
	posWidth uint64 = 8
	entWidth        = offWidth + posWidth
)

type index struct {
	file        *littlefs.File
	mu          sync.Mutex
	data        []byte
	initialSize uint64
	size        uint64
}

func newIndex(f *littlefs.File, c Config) (*index, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	initialSize := uint64(fi.Size())
	data := make([]byte, c.Segment.MaxIndexBytes)
	if initialSize > 0 {
		if _, err := f.Read(data); err != nil {
			return nil, err
		}
	}

	return &index{
		file:        f,
		data:        data,
		initialSize: initialSize,
		size:        initialSize,
	}, nil
}

// TODO: グレースフルでないシャットダウン
func (i *index) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.size > i.initialSize {
		appendData := i.data[i.initialSize:i.size]
		if _, err := i.file.Write(appendData); err != nil {
			return err
		}
	}
	// if err := i.file.Sync(); err != nil {
	// 	return err
	// }
	return i.file.Close()
}

func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	if i.size == 0 {
		return 0, 0, io.EOF
	}
	if in == -1 {
		out = uint32((i.size / entWidth) - 1)
	} else {
		out = uint32(in)
	}
	pos = uint64(out) * entWidth
	if i.size < pos+entWidth {
		return 0, 0, io.EOF
	}
	out = enc.Uint32(i.data[pos : pos+offWidth])
	pos = enc.Uint64(i.data[pos+offWidth : pos+entWidth])
	return out, pos, nil
}

func (i *index) Write(off uint32, pos uint64) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.IsMaxed() {
		return io.EOF
	}

	enc.PutUint32(i.data[i.size:i.size+offWidth], off)
	enc.PutUint64(i.data[i.size+offWidth:i.size+entWidth], pos)
	i.size += uint64(entWidth)
	return nil

}

func (i *index) IsMaxed() bool {
	return uint64(len(i.data)) < i.size+entWidth
}

func (i *index) Name() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.file.Name()
}
