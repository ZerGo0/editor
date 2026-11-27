package hexdump

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/ZerGo0/editor/internal/applog"
)

const maxCachedPages = 16

var (
	ErrLoading   = errors.New("hexdump: page is loading")
	ErrPageRange = errors.New("hexdump: page is outside the file")
)

type Source struct {
	r      io.ReaderAt
	size   int64
	loaded func()

	mu    sync.Mutex
	pages map[int64]*entry
	tick  uint64
}

type entry struct {
	data  []byte
	err   error
	ready bool
	used  uint64
}

func NewSource(r io.ReaderAt, size int64, loaded func()) *Source {
	return &Source{r: r, size: size, loaded: loaded, pages: map[int64]*entry{}}
}

func (s *Source) Size() int64 { return s.size }

func (s *Source) Page(i int64) ([]byte, error) {
	if i < 0 || i >= PageCount(s.size) {
		return nil, ErrPageRange
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tick++
	if e, ok := s.pages[i]; ok {
		e.used = s.tick
		if !e.ready {
			return nil, ErrLoading
		}
		return e.data, e.err
	}
	s.evict()
	e := &entry{used: s.tick}
	s.pages[i] = e
	applog.Go(func() { s.read(i, e) })
	return nil, ErrLoading
}

func (s *Source) evict() {
	for len(s.pages) >= maxCachedPages {
		oldest, found := int64(0), false
		for i, e := range s.pages {
			if e.ready && (!found || e.used < s.pages[oldest].used) {
				oldest, found = i, true
			}
		}
		if !found {
			return
		}
		delete(s.pages, oldest)
	}
}

func (s *Source) read(i int64, e *entry) {
	start, end := PageBounds(i, s.size)
	data, err := ReadRange(s.r, start, end)
	s.mu.Lock()
	e.data, e.err, e.ready = data, err, true
	s.mu.Unlock()
	if s.loaded != nil {
		s.loaded()
	}
}

func ReadRange(r io.ReaderAt, start, end int64) ([]byte, error) {
	buf := make([]byte, end-start)
	n, err := r.ReadAt(buf, start)
	if n == len(buf) {
		return buf, nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return nil, fmt.Errorf("read bytes %d to %d: %w", start, end, err)
}
