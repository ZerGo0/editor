package largefile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	saveChunk   = 1 << 20
	spaceMargin = 256 << 20
)

var (
	ErrNoSpace = errors.New("not enough free disk space")
	freeSpace  = diskFree
)

func (d *Doc) Save(ctx context.Context, progress func(done, total int64)) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return errClosed
	}
	snap := d.snapshotLocked()
	path := d.path
	d.mu.Unlock()
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	if err := checkSpace(filepath.Dir(target), snap.t.len()); err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	err = fsutil.WriteAtomic(target, info.Mode().Perm(), func(w io.Writer) error {
		return snap.writeTo(ctx, w, progress)
	})
	if err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	return d.rebase(target, snap)
}

func checkSpace(dir string, size int64) error {
	free, err := freeSpace(dir)
	if err != nil {
		return fmt.Errorf("check free space: %w", err)
	}
	need := uint64(size) + spaceMargin
	if free < need {
		return fmt.Errorf("%w: need %d MB, %d MB free", ErrNoSpace, need>>20, free>>20)
	}
	return nil
}

func (s snapshot) writeTo(ctx context.Context, w io.Writer, progress func(done, total int64)) error {
	total := s.t.len()
	var done int64
	buf := make([]byte, min(saveChunk, total))
	for _, c := range s.t.chunks {
		for _, p := range c.pieces {
			for k := int64(0); k < p.n; {
				if err := ctx.Err(); err != nil {
					return err
				}
				m := min(saveChunk, p.n-k)
				var data []byte
				if p.src == nil {
					data = s.add[p.off+k : p.off+k+m]
				} else {
					data = buf[:m]
					if err := p.src.readAt(data, p.off+k); err != nil {
						return err
					}
				}
				if _, err := w.Write(data); err != nil {
					return err
				}
				k += m
				done += m
				if progress != nil {
					progress(done, total)
				}
			}
		}
	}
	return nil
}

// TECHDEBT: Closing the old file fails a Find still reading it with an error; reference-count sources if searches must survive a save.
func (d *Doc) rebase(target string, snap snapshot) error {
	old, err := d.swapBase(target, snap)
	for _, s := range old {
		_ = s.close()
	}
	return err
}

func (d *Doc) swapBase(target string, snap snapshot) ([]*source, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, nil
	}
	if d.cur.id != snap.t.id {
		d.savedID = snap.t.id
		return nil, nil
	}
	src, err := openSource(target)
	if err != nil {
		return nil, fmt.Errorf("reopen after save: %w", err)
	}
	if src.size != snap.t.len() {
		return []*source{src}, fmt.Errorf("reopen after save %s: size %d, want %d: %w", target, src.size, snap.t.len(), errChanged)
	}
	t := d.settled()
	p := piece{src: src, n: src.size, nl: t.lines[len(t.chunks)], counted: t.exact()}
	old := d.sources
	d.base, d.sources = src, []*source{src}
	d.cur = newTree(t.id, rechunk([]piece{p}))
	d.savedID = t.id
	d.undo, d.redo, d.add = nil, nil, nil
	d.lastKind = editOther
	src.startIndex(d.notifyIndex)
	return old, nil
}
