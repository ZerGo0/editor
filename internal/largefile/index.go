package largefile

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"
)

const indexChunk = 64 * blockSize

type lineIndex struct {
	src      *source
	finished chan struct{}

	mu   sync.Mutex
	cum  []int64
	done bool
	err  error
}

func newLineIndex(src *source) *lineIndex {
	return &lineIndex{src: src, finished: make(chan struct{}), cum: []int64{0}}
}

func (x *lineIndex) run(ctx context.Context, notify func()) {
	defer close(x.finished)
	size := x.src.size
	buf := make([]byte, min(indexChunk, size))
	for off := int64(0); off < size; {
		if ctx.Err() != nil {
			return
		}
		n := min(indexChunk, size-off)
		if _, err := x.src.f.ReadAt(buf[:n], off); err != nil {
			x.fail(x.src.readErr(off, err))
			notify()
			return
		}
		counts := make([]int64, 0, (n+blockSize-1)/blockSize)
		for b := int64(0); b < n; b += blockSize {
			counts = append(counts, int64(bytes.Count(buf[b:min(b+blockSize, n)], newline)))
		}
		x.mu.Lock()
		for _, c := range counts {
			x.cum = append(x.cum, x.cum[len(x.cum)-1]+c)
		}
		x.mu.Unlock()
		off += n
		notify()
	}
	x.mu.Lock()
	x.done = true
	x.mu.Unlock()
	notify()
}

func (x *lineIndex) fail(err error) {
	x.mu.Lock()
	x.err = fmt.Errorf("index lines: %w", err)
	x.mu.Unlock()
}

func (x *lineIndex) progress() (done, total int64, finished bool, err error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.frontierLocked(), x.src.size, x.done, x.err
}

func (x *lineIndex) frontierLocked() int64 {
	return min(x.src.size, int64(len(x.cum)-1)*blockSize)
}

func (x *lineIndex) isDone() bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.done
}

func (x *lineIndex) density() (lines, bytes int64) {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.cum[len(x.cum)-1], x.frontierLocked()
}

func (x *lineIndex) count(a, b int64) (int64, bool) {
	if a >= b {
		return 0, true
	}
	nb, ok := x.before(b)
	if !ok {
		return 0, false
	}
	na, ok := x.before(a)
	if !ok {
		return 0, false
	}
	return nb - na, true
}

func (x *lineIndex) before(off int64) (int64, bool) {
	blk := off / blockSize
	x.mu.Lock()
	if off > x.frontierLocked() || blk >= int64(len(x.cum)) {
		x.mu.Unlock()
		return 0, false
	}
	base := x.cum[blk]
	x.mu.Unlock()
	rest := off - blk*blockSize
	if rest == 0 {
		return base, true
	}
	var n int
	err := x.src.withBlock(blk, func(b []byte) error {
		if rest > int64(len(b)) {
			return errChanged
		}
		n = bytes.Count(b[:rest], newline)
		return nil
	})
	return base + int64(n), err == nil
}

func (x *lineIndex) nth(m int64) (int64, bool) {
	x.mu.Lock()
	last := len(x.cum) - 1
	if m <= 0 || m > x.cum[last] {
		x.mu.Unlock()
		return 0, false
	}
	blk := sort.Search(last, func(i int) bool { return x.cum[i+1] >= m })
	rem := m - x.cum[blk]
	x.mu.Unlock()
	at := -1
	err := x.src.withBlock(int64(blk), func(b []byte) error {
		at = nthByte(b, newline[0], rem)
		return nil
	})
	if err != nil || at < 0 {
		return 0, false
	}
	return int64(blk)*blockSize + int64(at), true
}

func nthByte(b []byte, c byte, n int64) int {
	pos := 0
	for {
		i := bytes.IndexByte(b[pos:], c)
		if i < 0 {
			return -1
		}
		n--
		if n == 0 {
			return pos + i
		}
		pos += i + 1
	}
}
