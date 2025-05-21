package log

import (
	"encoding/binary"
	"io"
	"sync"

	"tinygo.org/x/tinyfs/littlefs"
)

var (
	enc = binary.BigEndian
)

const (
	lenWidth = 8
)

type store struct {
	file        *littlefs.File
	mu          sync.Mutex
	data        []byte
	initialSize uint64
	size        uint64
}

func newStore(f *littlefs.File, c Config) (*store, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	initialSize := uint64(fi.Size())
	data := make([]byte, c.Segment.MaxStoreBytes)
	if initialSize > 0 {
		if _, err := f.Read(data); err != nil {
			return nil, err
		}
	}

	return &store{
		file:        f,
		data:        data,
		initialSize: initialSize,
		size:        initialSize,
	}, nil
}

func (s *store) Append(p []byte) (n uint64, pos uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pos = s.size

	valueHead := s.size + lenWidth
	valueTail := valueHead + uint64(len(p))

	enc.PutUint64(s.data[s.size:valueHead], uint64(len(p)))
	copy(s.data[valueHead:valueTail], p)

	w := uint64(len(p)) + lenWidth
	s.size += w
	return w, pos, nil
}

func (s *store) Read(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	valueHead := pos + lenWidth
	if valueHead > s.size {
		return nil, io.ErrUnexpectedEOF
	}
	size := enc.Uint64(s.data[pos:valueHead])
	copyBuf := make([]byte, size)
	copy(copyBuf, s.data[valueHead:valueHead+size])
	return copyBuf, nil
}

func (s *store) ReadAt(p []byte, off int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uoff := uint64(off)
	uendOff := uoff + uint64(len(p))
	if uendOff > s.size {
		copy(p, s.data[uoff:s.size])
		return int(s.size - uoff), io.EOF
	}

	copy(p, s.data[uoff:uendOff])
	return len(p), nil
}

func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.size > s.initialSize {
		appendData := s.data[s.initialSize:s.size]
		if _, err := s.file.Write(appendData); err != nil {
			return err
		}

	}
	// if err := s.file.Sync(); err != nil {
	// 	return err
	// }
	return s.file.Close()
}

func (s *store) Name() string {
	return s.file.Name()
}
