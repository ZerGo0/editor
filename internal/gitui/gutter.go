package gitui

import (
	"errors"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

const maxBases = 64

type baseEntry struct {
	oid     string
	buf     *editor.Buffer
	loading bool
	missing bool
}

type untrackedCache struct {
	status *git.Status
	set    map[string]bool
}

func (g *Git) isUntracked(rel string) bool {
	st := g.snap.status
	if st == nil {
		return false
	}
	if g.untracked.status != st {
		set := map[string]bool{}
		for _, f := range st.Untracked() {
			set[f.Path] = true
		}
		g.untracked = untrackedCache{status: st, set: set}
	}
	return g.untracked.set[rel]
}

func (g *Git) Base(path string) *editor.Buffer {
	if !g.ready() || g.snap.status == nil || g.snap.status.OID == "" {
		return nil
	}
	rel, ok := g.paths.rel(path)
	if !ok || g.isUntracked(rel) {
		return nil
	}
	oid := g.snap.status.OID
	if e, ok := g.bases[rel]; ok && e.oid == oid {
		return e.buf
	}
	if len(g.bases) >= maxBases {
		for k := range g.bases {
			delete(g.bases, k)
			break
		}
	}
	e := &baseEntry{oid: oid, loading: true}
	g.bases[rel] = e
	r := g.repo
	applog.Go(func() {
		data, err := r.Show(g.ctx, oid, rel)
		if err == nil {
			err = checkText(data)
		}
		g.post(func() {
			if g.bases[rel] != e {
				return
			}
			e.loading = false
			if err != nil {
				e.missing = errors.Is(err, git.ErrNotFound) || errors.Is(err, errBinary) || errors.Is(err, errTooLarge)
				if !e.missing {
					delete(g.bases, rel)
				}
				return
			}
			text := string(data)
			buf := editor.NewBuffer(text)
			buf.SetLanguage(g.h.Language(path, text))
			e.buf = buf
		})
	})
	return nil
}
