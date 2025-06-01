package log

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	log_v1 "github.com/okitz/mqtt-log-pipeline/api/log"
	"github.com/okitz/mqtt-log-pipeline/internal/filesys"
	"tinygo.org/x/tinyfs/littlefs"
)

type Log struct {
	mu sync.RWMutex

	Fs     *littlefs.LFS
	Dir    *littlefs.File
	Config Config

	activeSegment *segment
	segments      []*segment
}

func NewLog(fs *littlefs.LFS, dirStr string, c Config) (*Log, error) {
	if c.Segment.MaxStoreBytes == 0 {
		c.Segment.MaxStoreBytes = 1024
	}
	if c.Segment.MaxIndexBytes == 0 {
		c.Segment.MaxIndexBytes = 1024
	}
	fs.Mkdir(dirStr, 0000)
	dir, err := filesys.OpenFile(fs, dirStr, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return nil, err
	}
	l := &Log{
		Fs:     fs,
		Dir:    dir,
		Config: c,
	}

	return l, l.setup()
}

func (l *Log) setup() error {
	files, err := l.Dir.Readdir(0)
	if err != nil {
		return err
	}
	var baseOffsets []uint64
	for _, file := range files {
		offStr := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)
		off, _ := strconv.ParseUint(offStr, 10, 0)
		baseOffsets = append(baseOffsets, off)
	}
	sort.Slice(baseOffsets, func(i, j int) bool {
		return baseOffsets[i] < baseOffsets[j]
	})
	// store, indexのbaseOffsetが重複して入っているので
	// 2つずつ飛ばして新しいsegmentを作る
	for i := 0; i < len(baseOffsets); i += 2 {
		if err = l.newSegment(baseOffsets[i]); err != nil {
			return err
		}
	}
	if l.segments == nil {
		if err = l.newSegment(
			l.Config.Segment.InitialOffset,
		); err != nil {
			return err
		}
	}
	return nil
}

// TODO: segmentごとのロック
func (l *Log) Append(value []byte) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	nextOffset, err := l.nextOffset()
	if err != nil {
		return 0, err
	}
	if flag, err := l.activeSegment.ToBeMaxed(value); err != nil {
		return 0, err
	} else if flag {
		err = l.newSegment(nextOffset)
		if err != nil {
			return 0, err
		}
	}

	off, err := l.activeSegment.Append(value)
	if err != nil {
		return 0, err
	}
	return l.offsetToIndex(off)
}

func (l *Log) Read(index uint64) (*log_v1.Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	off, err := l.indexToOffset(index)
	if err != nil {
		return nil, err
	}
	record, err := l.readAtOffset(off)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (l *Log) ReadFrom(index uint64) ([]*log_v1.Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	off, err := l.indexToOffset(index)
	if err != nil {
		return nil, err
	}
	var records []*log_v1.Record
	nextOffset, err := l.nextOffset()
	if err != nil {
		return nil, err
	}
	for i := off; i < nextOffset; i++ {
		record, err := l.readAtOffset(i)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (l *Log) NextIndex() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	nextOffset, err := l.nextOffset()
	if err != nil {
		panic(err)
	}
	nextIndex, err := l.offsetToIndex(nextOffset)
	if err != nil {
		panic(err)
	}
	return nextIndex
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, segment := range l.segments {
		if err := segment.Close(); err != nil {
			return err
		}
	}
	if err := l.Dir.Close(); err != nil {
		return err
	}
	return nil
}

func (l *Log) Remove() error {
	if err := l.activeSegment.Sync(); err != nil {
		return err
	}
	for _, segment := range l.segments {
		if err := segment.Remove(); err != nil {
			return err
		}
	}
	if err := l.Dir.Close(); err != nil {
		return err
	}
	return nil
}

func (l *Log) Reset() error {
	if err := l.Remove(); err != nil {
		return err
	}
	return l.setup()
}

func (l *Log) offsetToIndex(off uint64) (uint64, error) {
	lo, err := l.lowestOffset()
	if err != nil {
		return 0, err
	}
	if off < lo {
		return 0, fmt.Errorf("offset out of range: %d < base(%d)", off, lo)
	}
	return off - lo, nil
}

func (l *Log) indexToOffset(index uint64) (uint64, error) {
	lo, err := l.lowestOffset()
	if err != nil {
		return 0, err
	}
	nextOffset, err := l.nextOffset()
	if err != nil {
		return 0, err
	}
	if index >= nextOffset-lo {
		return 0, fmt.Errorf("index out of range: %d", index)
	}
	return lo + index, nil
}

func (l *Log) readAtOffset(off uint64) (*log_v1.Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var s *segment
	// TODO: 二分探索
	for _, segment := range l.segments {
		if segment.baseOffset <= off && off < segment.nextOffset {
			s = segment
			break
		}
	}
	if s == nil || s.nextOffset <= off {
		return nil, fmt.Errorf("offset out of range: %d", off)
	}
	return s.Read(off)
}

func (l *Log) newSegment(off uint64) error {
	s, err := newSegment(l.Fs, l.Dir.Name(), off, l.Config)
	if err != nil {
		return err
	}
	l.segments = append(l.segments, s)
	l.activeSegment = s
	return nil
}

func (l *Log) lowestOffset() (uint64, error) {
	return l.segments[0].baseOffset, nil
}

// ロックなしで呼び出せるPrivateメソッド
func (l *Log) nextOffset() (uint64, error) {
	off := l.segments[len(l.segments)-1].nextOffset
	return off, nil
}

// func (l *Log) Truncate(lowest uint64) error {
// 	l.mu.Lock()
// 	defer l.mu.Unlock()
// 	var segments []*segment
// 	for _, s := range l.segments {
// 		if s.nextOffset <= lowest+1 {
// 			if err := s.Remove(); err != nil {
// 				return err
// 			}
// 			continue
// 		}
// 		segments = append(segments, s)
// 	}
// 	l.segments = segments
// 	return nil
// }

// func (l *Log) Reader() io.Reader {
// 	l.mu.RLock()
// 	defer l.mu.RUnlock()
// 	readers := make([]io.Reader, len(l.segments))
// 	for i, segment := range l.segments {
// 		readers[i] = &originReader{segment.store, 0}
// 	}
// 	return io.MultiReader(readers...)
// }

// type originReader struct {
// 	*store
// 	off int64
// }

// func (o *originReader) Read(p []byte) (int, error) {
// 	n, err := o.ReadAt(p, o.off)
// 	o.off += int64(n)
// 	return n, err
// }
