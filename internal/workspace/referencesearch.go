package workspace

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/glob"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/search/references"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/workspace/symbolnav"
	"github.com/egoist/mygo/ui/editor"
)

func (w *Workspace) startSearch(t *layout.Tab, a symbolnav.Action, word string, q references.Query, locate bool) {
	pk := &w.peek
	if pk.cancel != nil {
		pk.cancel()
	}
	pk.gen++
	gen := pk.gen
	ctx, cancel := context.WithCancel(w.ctx)
	pk.cancel = cancel
	pk.tab, pk.line = t, t.Editor.CursorPos().Line
	near := symbolnav.Near{Path: t.Path, Pos: t.Editor.CursorPos()}
	switch a.Mode {
	case symbolnav.ModeFindAll:
		pk.view.Close()
		w.showFindResults(finder.FindResults{Title: symbolnav.SearchTitle(a.Kind, word), Busy: true})
	case symbolnav.ModePeek:
		pk.view.Begin(a.Kind, word, 0)
	default:
		pk.view.Begin(a.Kind, word, token.SpinnerDelay)
	}
	q.Word, q.Root, q.Current, q.Overlay, q.Exclude = word, w.Root, t.Path, w.overlay(), w.excludePatterns()
	useIndex := w.settings.Bool(settings.SearchUseIgnoreFiles)
	applog.Go(func() {
		var res references.Result
		var err error
		if locate {
			res, err = references.Locate(ctx, q)
		} else {
			q.Files = w.projectFiles(ctx, useIndex, q.Exclude)
			res, err = references.Find(ctx, q)
		}
		if err != nil {
			return
		}
		w.update(func() { w.finishSearch(gen, a, word, near, symbolnav.NewResult(a.Kind, word, w.Root, res)) })
	})
}

func (w *Workspace) finishSearch(gen int, a symbolnav.Action, word string, near symbolnav.Near, res symbolnav.Result) {
	pk := &w.peek
	if gen != pk.gen {
		return
	}
	switch {
	case a.Mode == symbolnav.ModeFindAll:
		w.showFindResults(res.Find())
		if res.Total == 0 {
			w.setNotice(a.Kind.Missing(word))
		}
	case res.Total == 0:
		pk.view.Close()
		w.setNotice(a.Kind.Missing(word))
	case a.Mode != symbolnav.ModePeek && res.Total == 1:
		pk.view.Close()
		w.openLocation(res.Location(res.Files[0], 0), finder.OpenOptions{Preview: true, Focus: true, Side: a.Mode == symbolnav.ModeSide})
	default:
		pk.view.Show(res, near)
	}
}

func (w *Workspace) excludePatterns() []string {
	return append(slices.Clone(w.settings.Strings(settings.Exclude)), w.settings.Strings(settings.SearchExclude)...)
}

func (w *Workspace) overlay() func(path string) (string, bool) {
	snaps := map[string]editor.Snapshot{}
	for _, p := range w.openPaths() {
		if buf := w.Buffer(p); buf != nil {
			snaps[p] = buf.Snapshot()
		}
	}
	return func(p string) (string, bool) {
		s, ok := snaps[p]
		if !ok {
			return "", false
		}
		return s.String(), true
	}
}

func (w *Workspace) projectFiles(ctx context.Context, useIndex bool, exclude []string) []string {
	rels, ready := w.search.files.Paths()
	if !useIndex || !ready {
		rels, _ = textsearch.WalkFiles(ctx, w.Root, func(rel string) bool { return glob.MatchAny(exclude, rel) })
	}
	out := make([]string, len(rels))
	for i, rel := range rels {
		out[i] = filepath.Join(w.Root, filepath.FromSlash(rel))
	}
	return out
}
