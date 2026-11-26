package largefile

import (
	"strings"
	"unicode/utf8"
)

const (
	shortLineProbe = 1 << 10
	lineProbe      = 4 << 10
	maxLineScan    = 1 << 20
	crlfLen        = 2
)

type lineInfo struct {
	start int64
	end   int64
	next  int64
	raw   string
	cut   bool
}

func (l lineInfo) hasNext() bool { return l.next >= 0 }

func peek(d *Doc, start, end int64) string {
	s, err := d.Slice(start, end)
	if err != nil {
		return ""
	}
	return s
}

// TECHDEBT: Only \n ends a line, so a lone \r (classic Mac endings) shows as a replacement mark; split on \r too if such files matter.
func readLine(d *Doc, start int64) (lineInfo, error) {
	w, err := d.Slice(start, start+shortLineProbe)
	if err == nil && !strings.Contains(w, "\n") && start+int64(len(w)) < d.Len() {
		w, err = d.Slice(start, start+rawLineCap+crlfLen)
	}
	if err != nil {
		return lineInfo{start: start, end: start, next: -1}, err
	}
	li := lineInfo{start: start, next: -1}
	switch i := strings.IndexByte(w, '\n'); {
	case i >= 0:
		li.next = start + int64(i) + 1
		li.end = start + int64(trimCR(w[:i]))
		li.raw = w[:li.end-start]
	case start+int64(len(w)) >= d.Len():
		li.end = start + int64(len(w))
		li.raw = w
	default:
		li.raw, li.cut = w[:rawLineCap], true
		li.end = lineEndOf(d, start)
		if nl := findNewline(d, start); nl >= 0 && nl < d.Len() {
			li.next = nl + 1
		}
	}
	return li, nil
}

func trimCR(s string) int {
	return len(strings.TrimSuffix(s, "\r"))
}

func findNewline(d *Doc, off int64) int64 {
	size := d.Len()
	w := peek(d, off, off+lineProbe)
	if i := strings.IndexByte(w, '\n'); i >= 0 {
		return off + int64(i)
	}
	if off+int64(len(w)) >= size {
		return size
	}
	if l, ok := d.LineOf(off); ok {
		if s, ok := d.LineStart(l + 1); ok && s > off {
			return s - 1
		}
		if n, exact := d.LineCount(); exact && l+1 >= n {
			return size
		}
	}
	for pos := off + int64(len(w)); pos < size && pos-off < maxLineScan; pos += lineProbe {
		if i := strings.IndexByte(peek(d, pos, pos+lineProbe), '\n'); i >= 0 {
			return pos + int64(i)
		}
	}
	return -1
}

func nextLineStart(d *Doc, off int64) (int64, bool) {
	nl := findNewline(d, off)
	if nl < 0 || nl >= d.Len() {
		return 0, false
	}
	return nl + 1, true
}

// TECHDEBT: Before the line index reaches it, a line longer than maxLineScan has no known end; the caret stops at the scan limit until indexing finishes.
func lineEndOf(d *Doc, off int64) int64 {
	nl := findNewline(d, off)
	if nl < 0 {
		return min(d.Len(), off+maxLineScan)
	}
	if nl > off && nl < d.Len() {
		if peek(d, nl-1, nl) == "\r" {
			return nl - 1
		}
	}
	return nl
}

// TECHDEBT: Before the line index reaches it, a line longer than maxLineScan starts at the scan limit; the real start shows once indexing finishes.
func lineStartOf(d *Doc, off int64) int64 {
	off = clamp(off, 0, d.Len())
	lo := max(0, off-lineProbe)
	if i := strings.LastIndexByte(peek(d, lo, off), '\n'); i >= 0 {
		return lo + int64(i) + 1
	}
	if lo == 0 {
		return 0
	}
	if l, ok := d.LineOf(off); ok {
		if s, ok := d.LineStart(l); ok {
			return s
		}
	}
	for pos := lo; pos > 0 && off-pos < maxLineScan; {
		lo := max(0, pos-lineProbe)
		if i := strings.LastIndexByte(peek(d, lo, pos), '\n'); i >= 0 {
			return lo + int64(i) + 1
		}
		pos = lo
	}
	return max(0, off-maxLineScan)
}

func prevBoundary(d *Doc, off int64) int64 {
	if off <= 0 {
		return 0
	}
	w := peek(d, max(0, off-utf8.UTFMax), off)
	if len(w) == 0 {
		return off - 1
	}
	if strings.HasSuffix(w, "\r\n") {
		return off - crlfLen
	}
	_, size := utf8.DecodeLastRuneInString(w)
	return off - int64(max(1, size))
}

func nextBoundary(d *Doc, off int64) int64 {
	w := peek(d, off, off+utf8.UTFMax)
	if len(w) == 0 {
		return off
	}
	if strings.HasPrefix(w, "\r\n") {
		return off + crlfLen
	}
	_, size := utf8.DecodeRuneInString(w)
	return off + int64(max(1, size))
}
