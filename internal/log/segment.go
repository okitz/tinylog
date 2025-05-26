package log

import (
	"fmt"
	"os"
	"path/filepath"

	log_v1 "github.com/okitz/mqtt-log-pipeline/api/log"
	"github.com/okitz/mqtt-log-pipeline/internal/filesys"
	"tinygo.org/x/tinyfs/littlefs"
)

type segment struct {
	store                  *store
	index                  *index
	baseOffset, nextOffset uint64
	config                 Config
	fs                     *littlefs.LFS
	dirname                string
	closed                 bool
}

func newSegment(fs *littlefs.LFS, dirname string, baseOffset uint64, c Config) (*segment, error) {
	s := &segment{
		baseOffset: baseOffset,
		config:     c,
		fs:         fs,
		dirname:    dirname,
		closed:     false,
	}
	filename := fmt.Sprintf("%d%s", baseOffset, ".store")
	fmt.Println("store filename", filename)
	storeFile, err := filesys.OpenFile(fs,
		filepath.Join(dirname, filename),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
	)
	if err != nil {
		return nil, err
	}
	if s.store, err = newStore(storeFile, c); err != nil {
		return nil, err
	}
	filename = fmt.Sprintf("%d%s", baseOffset, ".index")
	fmt.Println("index filename", filename)
	indexFile, err := filesys.OpenFile(fs,
		filepath.Join(dirname, filename),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
	)

	if err != nil {
		return nil, err
	}
	if s.index, err = newIndex(indexFile, c); err != nil {
		return nil, err
	}
	if off, _, err := s.index.Read(-1); err != nil {
		s.nextOffset = baseOffset
	} else {
		s.nextOffset = baseOffset + uint64(off) + 1
	}
	return s, nil
}

func (s *segment) Append(record *log_v1.Record) (offset uint64, err error) {
	cur := s.nextOffset
	record.Offset = cur
	p, err := record.MarshalVT()
	if err != nil {
		return 0, err
	}
	_, pos, err := s.store.Append(p)
	if err != nil {
		return 0, err
	}
	if err = s.index.Write(
		// relateive offset
		uint32(s.nextOffset-uint64(s.baseOffset)),
		pos,
	); err != nil {
		return 0, err
	}
	s.nextOffset++
	return cur, nil
}

func (s *segment) Read(off uint64) (*log_v1.Record, error) {
	_, pos, err := s.index.Read(int64(off - s.baseOffset))
	if err != nil {
		return nil, err
	}
	p, err := s.store.Read(pos)
	if err != nil {
		return nil, err
	}
	record := &log_v1.Record{}
	err = record.UnmarshalVT(p)
	return record, err
}

func (s *segment) IsMaxed() bool {
	return s.store.size >= s.config.Segment.MaxStoreBytes ||
		s.index.size >= s.config.Segment.MaxIndexBytes ||
		s.index.IsMaxed()
}

func (s *segment) ToBeMaxed(record *log_v1.Record) (bool, error) {
	p, err := record.MarshalVT()
	if err != nil {
		return false, err
	}
	return s.store.size+lenWidth+uint64(len(p)) >= s.config.Segment.MaxStoreBytes ||
		s.index.IsMaxed(), nil
}

func (s *segment) Remove() error {
	if s.closed {
		return fmt.Errorf("segment %d already closed", s.baseOffset)
	}
	if err := s.Close(); err != nil {
		return err
	}
	s.closed = true
	indexPath := filepath.Join(s.dirname, fmt.Sprintf("%d%s", s.baseOffset, ".index"))
	storePath := filepath.Join(s.dirname, fmt.Sprintf("%d%s", s.baseOffset, ".store"))
	if err := s.fs.Remove(indexPath); err != nil {
		return err
	}
	if err := s.fs.Remove(storePath); err != nil {
		return err
	}
	return nil
}

func (s *segment) Sync() error {
	if err := s.store.Sync(); err != nil {
		return err
	}
	if err := s.index.Sync(); err != nil {
		return err
	}
	return nil
}

func (s *segment) Close() error {
	if s.closed {
		return fmt.Errorf("segment %d already closed", s.baseOffset)
	}
	if err := s.index.Close(); err != nil {
		return err
	}
	if err := s.store.Close(); err != nil {
		return err
	}
	return nil
}
