package gitui

import (
	"context"
	"errors"
	"fmt"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type patchItem struct {
	header      bool
	title       string
	detail      string
	code        byte
	src         *diffSource
	patch       *editor.FilePatch
	commit      *git.Commit
	note        string
	binary      bool
	adds        int
	dels        int
	counted     bool
	size        int64
	fingerprint string
	row         *changeRow
	full        bool
}

type patchOpen struct {
	reveal  string
	preview bool
	focus   bool
	prepare func(t *patchTab)
}

type patchTab struct {
	g       *Git
	id      string
	title   string
	load    func(ctx context.Context, r repo) ([]patchItem, error)
	prepare func()
	items   []patchItem
	loaded  bool
	loading bool
	err     error
	gen     int
	view    *multidiff.View
}

func (g *Git) openPatchTab(id, title string, load func(ctx context.Context, r repo) ([]patchItem, error)) {
	g.openPatchTabWith(id, title, load, patchOpen{focus: true})
}

func (g *Git) openPatchTabWith(id, title string, load func(ctx context.Context, r repo) ([]patchItem, error), o patchOpen) {
	if !g.ready() {
		return
	}
	t, ok := g.patchTabs[id]
	if !ok {
		t = &patchTab{g: g, id: id, title: title, view: multidiff.New()}
		g.patchTabs[id] = t
		g.patchOrder = append(g.patchOrder, id)
		if len(g.patchOrder) > maxKeptTabs {
			g.patchTabs[g.patchOrder[0]].view.Close()
			delete(g.patchTabs, g.patchOrder[0])
			g.patchOrder = g.patchOrder[1:]
		}
	}
	t.load = load
	if o.prepare != nil {
		o.prepare(t)
	}
	if o.reveal != "" {
		t.view.RevealPath(o.reveal)
	}
	t.reload()
	g.openView(id, title, t.render, o.preview, o.focus)
}

func (g *Git) reloadGroupTabs() {
	for _, t := range g.patchTabs {
		if t.prepare != nil && t.loaded {
			t.reload()
		}
	}
}

func (t *patchTab) reload() {
	g := t.g
	if !g.ready() {
		return
	}
	if t.prepare != nil {
		t.prepare()
	}
	t.gen++
	gen, r, load := t.gen, g.repo, t.load
	t.loading, t.err = true, nil
	applog.Go(func() {
		items, err := load(g.ctx, r)
		g.post(func() {
			if gen != t.gen {
				return
			}
			t.loading, t.err, t.loaded = false, err, true
			if err == nil {
				t.items = items
				t.view.SetEntries(t.entries(r))
			}
		})
	})
}

func (t *patchTab) options() multidiff.Options {
	g := t.g
	if g.viewed == nil {
		g.viewed = multidiff.NewMemory()
	}
	return multidiff.Options{
		Label: t.title,
		Set:   t.id,
		Ctx:   g.ctx,
		Post:  g.post,
		Configure: func(d *editor.DiffState, rel string) {
			g.h.ConfigureDiff(d, g.paths.abs(rel))
			g.diffOpts.apply(d)
		},
		Language: func(rel, text string) *editor.Language { return g.h.Language(g.paths.abs(rel), text) },
		Viewed:   g.viewed,
		OpenFile: func(rel string) { g.h.OpenAt(g.paths.abs(rel), -1, -1) },
		Busy:     t.loading,
		Toolbar: func(c *ui.Context) {
			g.diffOptionsButton(c)
			if iconButton(c, "Reload Changes", icons.Refresh, t.loading) {
				t.reload()
			}
		},
	}
}

func (t *patchTab) render(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		switch {
		case t.err != nil:
			errorState(c, "Cannot load the changes", t.err, t.reload)
		case !t.loaded:
			loadingState(c, "Loading changes")
		case len(t.items) == 0:
			emptyState(c, "No changes", "")
		default:
			t.view.Render(c, t.options())
		}
	})
}

func (t *patchTab) entries(r repo) []multidiff.Entry {
	g := t.g
	entries := make([]multidiff.Entry, 0, len(t.items))
	seen := map[string]int{}
	for i := range t.items {
		it := t.items[i]
		var e multidiff.Entry
		switch {
		case it.header:
			e.Section = func(c *ui.Context) {
				list.Row(c, list.State{}, list.Item{Label: it.title, Strong: true, Description: it.detail, Icon: icons.Commit})
			}
		case it.commit != nil:
			e.Section = g.commitSection(*it.commit, it.full)
		default:
			e = t.fileEntry(r, it)
		}
		e.Key = uniqueKey(seen, fmt.Sprintf("%d:%s", i, it.title))
		if it.src != nil {
			e.Key = uniqueKey(seen, it.src.id())
		}
		entries = append(entries, e)
	}
	return entries
}

func uniqueKey(seen map[string]int, key string) string {
	seen[key]++
	if n := seen[key]; n > 1 {
		return fmt.Sprintf("%s#%d", key, n)
	}
	return key
}

func (t *patchTab) fileEntry(r repo, it patchItem) multidiff.Entry {
	g := t.g
	e := multidiff.Entry{
		Path: it.title, OldPath: it.detail, Status: gitStatus(it.code), Adds: multidiff.CountUnknown, Dels: multidiff.CountUnknown,
		Size: it.size, Fingerprint: it.fingerprint, Binary: it.binary, Note: it.note, Patch: it.patch,
	}
	if it.counted {
		e.Adds, e.Dels = it.adds, it.dels
	}
	if it.src == nil {
		return e
	}
	src := *it.src
	abs := g.paths.abs(src.path)
	ops := &hunkOps{g: g, src: src, reload: t.reload}
	ops.live = func() bool { return src.usesWorkTree() && g.h.Buffer(abs) != nil }
	if src.usesWorkTree() {
		e.Live = func() *editor.Buffer { return g.h.Buffer(abs) }
	}
	e.Load = func(ctx context.Context, live bool) (string, string, error) {
		oldText, newText, err := loadDiffTexts(ctx, r, src, abs, !live)
		if err != nil {
			return "", "", noteError(err)
		}
		return oldText, newText, nil
	}
	e.Setup = func(d *editor.DiffState, _ *diffmode.Engine) {
		ops.d = d
		d.RenderHunkActions = ops.render
	}
	e.Frame = func(d *editor.DiffState, eng *diffmode.Engine) { d.HunkActions = ops.enabled() && !eng.Filtered() }
	e.Actions = func(c *ui.Context) { t.fileActions(c, it, src) }
	return e
}

func noteError(err error) error {
	if errors.Is(err, errBinary) || errors.Is(err, errTooLarge) {
		return multidiff.Note(err.Error())
	}
	return err
}

func (t *patchTab) fileActions(c *ui.Context, it patchItem, src diffSource) {
	g := t.g
	busy := g.busy != ""
	switch src.kind {
	case diffWorkTree, diffUntracked:
		if iconButton(c, "Stage Changes", icons.Plus, busy) {
			g.stage(src.path)
		}
		if it.row != nil && iconButton(c, "Discard Changes", icons.Undo, busy) {
			g.rollback([]changeRow{*it.row})
		}
	case diffStaged:
		if iconButton(c, "Unstage Changes", icons.Minus, busy) {
			g.unstage(src.path)
		}
	}
	if iconButton(c, "Open Single File Diff", icons.Preview, false) {
		g.openSingleDiff(src, false, true)
	}
}

func fileDiffItems(fds []git.FileDiff) []patchItem {
	items := make([]patchItem, 0, len(fds))
	for _, fd := range fds {
		it := patchItem{title: fd.Path(), code: 'M', binary: fd.Binary}
		it.adds, it.dels, it.counted = diffCounts(fd), diffDels(fd), true
		switch {
		case fd.IsNew():
			it.code = 'A'
		case fd.IsDeleted():
			it.code = 'D'
		case fd.IsRename():
			it.code, it.detail = 'R', fd.OldPath
		}
		if fd.Binary {
			items = append(items, it)
			continue
		}
		fps, err := editor.ParsePatch(string(git.BuildPatch(fd, func(int, int) bool { return true }, false)))
		switch {
		case err != nil:
			it.note = "Cannot show this change: " + err.Error()
		case len(fps) == 0:
			it.note = "This change has no text lines."
		default:
			it.patch = &fps[0]
		}
		items = append(items, it)
	}
	return items
}
