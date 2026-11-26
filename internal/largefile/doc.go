package largefile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	maxSlice          = 64 << 20
	undoMax           = 10000
	mergeDeleteMax    = 4
	estimateLineBytes = 64
)

var (
	newline       = []byte{'\n'}
	errClosed     = errors.New("document is closed")
	errSliceLarge = fmt.Errorf("range is larger than %d MB", maxSlice>>20)
)

type editKind int

const (
	editOther editKind = iota
	editInsert
	editDelete
)

type span struct{ lo, hi int64 }

// TECHDEBT: Each undo step keeps a whole tree version, O(chunks) memory per step; store piece diffs or cap undo by bytes if documents collect very many pieces.
type step struct {
	t      *tree
	before span
	after  span
}

type Doc struct {
	path   string
	notify atomic.Pointer[func()]

	mu       sync.Mutex
	closed   bool
	base     *source
	sources  []*source
	cur      *tree
	add      []byte
	ids      uint64
	savedID  uint64
	undo     []step
	redo     []step
	lastKind editKind
}

func Open(path string) (*Doc, error) {
	d, err := openDoc(path)
	if err != nil {
		return nil, err
	}
	d.base.startIndex(d.notifyIndex)
	return d, nil
}

func openDoc(path string) (*Doc, error) {
	src, err := openSource(path)
	if err != nil {
		return nil, err
	}
	d := &Doc{path: path, base: src, sources: []*source{src}, ids: 1, savedID: 1}
	d.cur = newTree(d.ids, rechunk([]piece{{src: src, n: src.size}}))
	return d, nil
}

func (d *Doc) Close() error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	sources := d.sources
	d.mu.Unlock()
	var errs []error
	for _, s := range sources {
		errs = append(errs, s.close())
	}
	return errors.Join(errs...)
}

func (d *Doc) OnIndexProgress(fn func()) {
	d.notify.Store(&fn)
}

func (d *Doc) notifyIndex() {
	if fn := d.notify.Load(); fn != nil && *fn != nil {
		(*fn)()
	}
}

func (d *Doc) Indexing() (done, total int64, err error) {
	d.mu.Lock()
	idx := d.base.idx
	d.mu.Unlock()
	done, total, _, err = idx.progress()
	return done, total, err
}

func (d *Doc) Len() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cur.len()
}

func (d *Doc) Version() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cur.id
}

func (d *Doc) Modified() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cur.id != d.savedID
}

func (d *Doc) LineCount() (int64, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.settled()
	if t.exact() {
		return t.lines[len(t.chunks)] + 1, true
	}
	lines, size := d.base.idx.density()
	if lines == 0 || size == 0 {
		return t.len()/estimateLineBytes + 1, false
	}
	return t.len()*lines/size + 1, false
}

func (d *Doc) LineStart(line int64) (int64, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.settled()
	if line <= 0 {
		return 0, true
	}
	rem := line
	ci, pos := 0, int64(0)
	if t.exact() {
		if line > t.lines[len(t.chunks)] {
			return t.len(), false
		}
		ci = searchLines(t, line)
		rem -= t.lines[ci]
		pos = t.starts[ci]
	}
	for _, c := range t.chunks[ci:] {
		if c.uncounted == 0 && c.nl < rem {
			rem -= c.nl
			pos += c.n
			continue
		}
		for _, p := range c.pieces {
			n, full := d.pieceLines(p)
			if n >= rem {
				at, ok := d.nthNewline(p, rem)
				return pos + at + 1, ok
			}
			if !full {
				return 0, false
			}
			rem -= n
			pos += p.n
		}
	}
	return t.len(), false
}

func searchLines(t *tree, line int64) int {
	lo, hi := 0, len(t.chunks)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if t.lines[mid+1] >= line {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func (d *Doc) LineOf(off int64) (int64, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.settled()
	off = clamp(off, 0, t.len())
	if len(t.chunks) == 0 {
		return 0, true
	}
	var lines, pos int64
	ci := 0
	if t.exact() {
		ci = t.chunkAt(off)
		lines, pos = t.lines[ci], t.starts[ci]
	}
	for _, c := range t.chunks[ci:] {
		if pos+c.n <= off && c.uncounted == 0 {
			lines += c.nl
			pos += c.n
			continue
		}
		for _, p := range c.pieces {
			if pos+p.n <= off {
				n, full := d.pieceLines(p)
				if !full {
					return 0, false
				}
				lines += n
				pos += p.n
				continue
			}
			n, ok := d.prefixLines(p, off-pos)
			return lines + n, ok
		}
	}
	return lines, true
}

func (d *Doc) settled() *tree {
	if !d.cur.exact() && d.base.idx.isDone() {
		d.cur = d.cur.settle(d.pieceLines)
	}
	return d.cur
}

func (d *Doc) pieceLines(p piece) (int64, bool) {
	if p.counted {
		return p.nl, true
	}
	if n, ok := p.src.idx.count(p.off, p.end()); ok {
		return n, true
	}
	done, _, _, _ := p.src.idx.progress()
	if done <= p.off {
		return 0, false
	}
	n, _ := p.src.idx.count(p.off, done)
	return n, false
}

func (d *Doc) prefixLines(p piece, k int64) (int64, bool) {
	if p.src == nil {
		return int64(bytes.Count(d.add[p.off:p.off+k], newline)), true
	}
	return p.src.idx.count(p.off, p.off+k)
}

func (d *Doc) nthNewline(p piece, k int64) (int64, bool) {
	if p.src == nil {
		at := nthByte(d.add[p.off:p.end()], '\n', k)
		return int64(at), at >= 0
	}
	before, ok := p.src.idx.before(p.off)
	if !ok {
		return 0, false
	}
	at, ok := p.src.idx.nth(before + k)
	return at - p.off, ok
}

func (d *Doc) cut(p piece, k int64) (piece, piece) {
	l := piece{src: p.src, off: p.off, n: k}
	r := piece{src: p.src, off: p.off + k, n: p.n - k}
	if p.src == nil {
		if k <= p.n-k {
			l.nl = int64(bytes.Count(d.add[l.off:l.end()], newline))
			r.nl = p.nl - l.nl
		} else {
			r.nl = int64(bytes.Count(d.add[r.off:r.end()], newline))
			l.nl = p.nl - r.nl
		}
		l.counted, r.counted = true, true
		return l, r
	}
	ln, lok := p.src.idx.count(l.off, l.end())
	rn, rok := p.src.idx.count(r.off, r.end())
	switch {
	case lok && rok:
		l.nl, l.counted, r.nl, r.counted = ln, true, rn, true
	case lok && p.counted:
		l.nl, l.counted, r.nl, r.counted = ln, true, p.nl-ln, true
	case rok && p.counted:
		l.nl, l.counted, r.nl, r.counted = p.nl-rn, true, rn, true
	case lok:
		l.nl, l.counted = ln, true
	case rok:
		r.nl, r.counted = rn, true
	}
	return l, r
}

func (d *Doc) ReadAt(p []byte, off int64) (int, error) {
	d.mu.Lock()
	snap := d.snapshotLocked()
	closed := d.closed
	d.mu.Unlock()
	if closed {
		return 0, errClosed
	}
	return snap.readAt(p, off)
}

func (d *Doc) Slice(start, end int64) (string, error) {
	d.mu.Lock()
	snap := d.snapshotLocked()
	closed := d.closed
	d.mu.Unlock()
	if closed {
		return "", errClosed
	}
	start = clamp(start, 0, snap.t.len())
	end = clamp(end, start, snap.t.len())
	if end-start > maxSlice {
		return "", errSliceLarge
	}
	buf := make([]byte, end-start)
	n, err := snap.readAt(buf, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return string(buf[:n]), err
	}
	return string(buf[:n]), nil
}

func (d *Doc) Insert(off int64, s string) { d.Replace(off, off, s) }

func (d *Doc) Delete(start, end int64) { d.Replace(start, end, "") }

func (d *Doc) Replace(start, end int64, s string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	t := d.cur
	start = clamp(start, 0, t.len())
	end = clamp(end, start, t.len())
	if start == end && s == "" {
		return
	}
	var ins []piece
	if s != "" {
		ins = []piece{{off: int64(len(d.add)), n: int64(len(s)), nl: int64(strings.Count(s, "\n")), counted: true}}
		d.add = append(d.add, s...)
	}
	d.record(start, end, s)
	d.ids++
	d.cur = t.replace(d.ids, start, end, ins, d.cut)
}

func (d *Doc) record(start, end int64, s string) {
	kind := editOther
	switch {
	case start == end && !strings.Contains(s, "\n"):
		kind = editInsert
	case s == "" && end-start <= mergeDeleteMax:
		kind = editDelete
	}
	after := span{start, start + int64(len(s))}
	if n := len(d.undo); n > 0 && kind != editOther && kind == d.lastKind {
		top := &d.undo[n-1]
		switch {
		case kind == editInsert && start == top.after.hi:
			top.after.hi = after.hi
			return
		case kind == editDelete && end == top.after.lo:
			top.before.lo = start
			top.after = after
			return
		case kind == editDelete && start == top.after.lo:
			top.before.hi += end - start
			return
		}
	}
	d.lastKind = kind
	d.undo = append(d.undo, step{t: d.cur, before: span{start, end}, after: after})
	if len(d.undo) > undoMax {
		d.undo = slices.Delete(d.undo, 0, len(d.undo)-undoMax)
	}
	d.redo = nil
}

func (d *Doc) breakGroup() {
	d.mu.Lock()
	d.lastKind = editOther
	d.mu.Unlock()
}

func (d *Doc) Undo() (start, end int64, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.undo)
	if n == 0 || d.closed {
		return 0, 0, false
	}
	s := d.undo[n-1]
	d.undo = d.undo[:n-1]
	d.redo = append(d.redo, step{t: d.cur, before: s.before, after: s.after})
	d.cur = s.t
	d.lastKind = editOther
	return s.before.lo, s.before.hi, true
}

func (d *Doc) Redo() (start, end int64, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.redo)
	if n == 0 || d.closed {
		return 0, 0, false
	}
	s := d.redo[n-1]
	d.redo = d.redo[:n-1]
	d.undo = append(d.undo, step{t: d.cur, before: s.before, after: s.after})
	d.cur = s.t
	d.lastKind = editOther
	return s.after.lo, s.after.hi, true
}

type snapshot struct {
	t   *tree
	add []byte
}

func (d *Doc) snapshotLocked() snapshot {
	return snapshot{t: d.cur, add: d.add[:len(d.add):len(d.add)]}
}

func (s snapshot) readAt(p []byte, off int64) (int, error) {
	size := s.t.len()
	if off < 0 {
		return 0, fmt.Errorf("read at %d: negative offset", off)
	}
	if off >= size {
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	n := 0
	ci := s.t.chunkAt(off)
	pos := s.t.starts[ci]
	for _, c := range s.t.chunks[ci:] {
		for _, pc := range c.pieces {
			at := off + int64(n)
			if pos+pc.n <= at {
				pos += pc.n
				continue
			}
			k := at - pos
			m := min(pc.n-k, int64(len(p)-n))
			dst := p[n : n+int(m)]
			if pc.src == nil {
				copy(dst, s.add[pc.off+k:])
			} else if err := pc.src.readAt(dst, pc.off+k); err != nil {
				return n, err
			}
			n += int(m)
			pos += pc.n
			if n == len(p) {
				return n, nil
			}
		}
	}
	return n, io.EOF
}

func clamp(v, lo, hi int64) int64 {
	return max(lo, min(v, hi))
}
