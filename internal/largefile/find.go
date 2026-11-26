package largefile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

const (
	findChunk  = 1 << 20
	maxPattern = 4096
	asciiCase  = 'a' - 'A'
)

func (d *Doc) Find(ctx context.Context, from int64, pattern string, matchCase, backward bool) (start, end int64, found bool, err error) {
	if pattern == "" {
		return 0, 0, false, nil
	}
	if len(pattern) > maxPattern {
		return 0, 0, false, fmt.Errorf("find: pattern is longer than %d bytes", maxPattern)
	}
	d.mu.Lock()
	snap := d.snapshotLocked()
	closed := d.closed
	d.mu.Unlock()
	if closed {
		return 0, 0, false, errClosed
	}
	pat := []byte(pattern)
	fold := !matchCase
	if fold {
		lowerASCII(pat)
	}
	var at int64
	if backward {
		at, found, err = snap.findBackward(ctx, from, pat, fold)
	} else {
		at, found, err = snap.findForward(ctx, from, pat, fold)
	}
	if err != nil || !found {
		return 0, 0, false, err
	}
	return at, at + int64(len(pat)), true, nil
}

func (s snapshot) findForward(ctx context.Context, from int64, pat []byte, fold bool) (int64, bool, error) {
	size, m := s.t.len(), int64(len(pat))
	buf := make([]byte, findChunk+m-1)
	for lo := max(0, from); lo+m <= size; lo += findChunk {
		w, err := s.window(ctx, buf, lo, min(size, lo+findChunk+m-1), fold)
		if err != nil {
			return 0, false, err
		}
		if i := bytes.Index(w, pat); i >= 0 {
			return lo + int64(i), true, nil
		}
	}
	return 0, false, nil
}

func (s snapshot) findBackward(ctx context.Context, from int64, pat []byte, fold bool) (int64, bool, error) {
	size, m := s.t.len(), int64(len(pat))
	buf := make([]byte, findChunk+m-1)
	for top := min(from, size-m+1); top > 0; {
		lo := max(0, top-findChunk)
		w, err := s.window(ctx, buf, lo, top+m-1, fold)
		if err != nil {
			return 0, false, err
		}
		if i := bytes.LastIndex(w, pat); i >= 0 {
			return lo + int64(i), true, nil
		}
		top = lo
	}
	return 0, false, nil
}

func (s snapshot) window(ctx context.Context, buf []byte, lo, hi int64, fold bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	w := buf[:hi-lo]
	n, err := s.readAt(w, lo)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("find: %w", err)
	}
	w = w[:n]
	if fold {
		lowerASCII(w)
	}
	return w, nil
}

// TECHDEBT: Ignoring case folds ASCII letters only; fold Unicode with a rune-aware matcher if users need it.
func lowerASCII(b []byte) {
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + asciiCase
		}
	}
}
