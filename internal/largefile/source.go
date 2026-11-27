package largefile

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/ZerGo0/editor/internal/applog"
)

const (
	blockSize     = 64 << 10
	cacheBlocks   = 256
	directReadMin = 4 * blockSize
)

var errChanged = errors.New("file changed on disk")

type source struct {
	f      *os.File
	name   string
	size   int64
	idx    *lineIndex
	cancel context.CancelFunc

	mu     sync.Mutex
	blocks map[int64]*list.Element
	lru    list.List
}

type cachedBlock struct {
	index int64
	data  []byte
}

func openSource(path string) (*source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("open %s: not a regular file", path)
	}
	s := &source{f: f, name: path, size: info.Size(), blocks: map[int64]*list.Element{}}
	s.idx = newLineIndex(s)
	return s, nil
}

func (s *source) startIndex(notify func()) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	applog.Go(func() { s.idx.run(ctx, notify) })
}

func (s *source) close() error {
	if s.cancel != nil {
		s.cancel()
		<-s.idx.finished
	}
	return s.f.Close()
}

func (s *source) readAt(p []byte, off int64) error {
	if off < 0 || off+int64(len(p)) > s.size {
		return fmt.Errorf("read %s at %d: out of range", s.name, off)
	}
	if len(p) >= directReadMin {
		if _, err := s.f.ReadAt(p, off); err != nil {
			return s.readErr(off, err)
		}
		return nil
	}
	for len(p) > 0 {
		n, err := s.copyBlock(p, off)
		if err != nil {
			return err
		}
		p = p[n:]
		off += int64(n)
	}
	return nil
}

func (s *source) copyBlock(p []byte, off int64) (int, error) {
	var n int
	err := s.withBlock(off/blockSize, func(b []byte) error {
		k := off % blockSize
		if k >= int64(len(b)) {
			return fmt.Errorf("read %s at %d: %w", s.name, off, errChanged)
		}
		n = copy(p, b[k:])
		return nil
	})
	return n, err
}

func (s *source) withBlock(i int64, fn func([]byte) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := s.block(i)
	if err != nil {
		return err
	}
	return fn(b)
}

func (s *source) block(i int64) ([]byte, error) {
	if e, ok := s.blocks[i]; ok {
		s.lru.MoveToFront(e)
		return e.Value.(*cachedBlock).data, nil
	}
	start := i * blockSize
	n := min(blockSize, s.size-start)
	var data []byte
	if s.lru.Len() >= cacheBlocks {
		old := s.lru.Back()
		s.lru.Remove(old)
		b := old.Value.(*cachedBlock)
		delete(s.blocks, b.index)
		data = b.data[:0]
	}
	if int64(cap(data)) < n {
		data = make([]byte, n)
	}
	data = data[:n]
	if _, err := s.f.ReadAt(data, start); err != nil {
		return nil, s.readErr(start, err)
	}
	s.blocks[i] = s.lru.PushFront(&cachedBlock{index: i, data: data})
	return data, nil
}

func (s *source) readErr(off int64, err error) error {
	if errors.Is(err, io.EOF) {
		err = errChanged
	}
	return fmt.Errorf("read %s at %d: %w", s.name, off, err)
}
