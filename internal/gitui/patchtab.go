package gitui

import (
	"context"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const patchSplit = 32

type patchItem struct {
	header bool
	title  string
	detail string
	code   byte
	src    *diffSource
	patch  *editor.FilePatch
	commit *git.Commit
	note   string
}

func (it patchItem) key(i int) any {
	switch {
	case it.src != nil:
		return it.src.id()
	case it.commit != nil:
		return "c:" + it.commit.OID + ":" + it.title
	}
	return i
}

type patchTab struct {
	g        *Git
	id       string
	title    string
	load     func(ctx context.Context, r repo) ([]patchItem, error)
	items    []patchItem
	loading  bool
	err      error
	gen      int
	list     ui.ListState
	selected int
	split    float32
	shown    int
	state    *editor.DiffState
	engine   *diffmode.Engine
	shownWS  git.WhitespaceMode
	layout   int
}

func (g *Git) openPatchTab(id, title string, load func(ctx context.Context, r repo) ([]patchItem, error)) {
	if !g.ready() {
		return
	}
	t, ok := g.patchTabs[id]
	if !ok {
		t = &patchTab{g: g, id: id, title: title, selected: -1, shown: -1, split: patchSplit}
		g.patchTabs[id] = t
		g.patchOrder = append(g.patchOrder, id)
		if len(g.patchOrder) > maxKeptTabs {
			g.patchTabs[g.patchOrder[0]].dropState()
			delete(g.patchTabs, g.patchOrder[0])
			g.patchOrder = g.patchOrder[1:]
		}
	}
	t.load = load
	t.reload()
	g.h.OpenView(id, title, t.view)
}

func (t *patchTab) reload() {
	g := t.g
	if !g.ready() {
		return
	}
	t.gen++
	gen, r := t.gen, g.repo
	t.loading, t.err = true, nil
	go func() {
		items, err := t.load(g.ctx, r)
		g.post(func() {
			if gen != t.gen {
				return
			}
			t.dropState()
			t.loading, t.err, t.items, t.shown = false, err, items, -1
			if t.selected >= len(items) || t.selected < 0 {
				t.selected = firstItem(items)
			}
		})
	}()
}

func (t *patchTab) dropState() {
	if t.engine != nil {
		t.engine.Close()
	}
	t.state, t.engine = nil, nil
}

func firstItem(items []patchItem) int {
	for i, it := range items {
		if !it.header {
			return i
		}
	}
	return -1
}

func fileDiffItems(fds []git.FileDiff) []patchItem {
	items := make([]patchItem, 0, len(fds))
	for _, fd := range fds {
		it := patchItem{title: fd.Path(), code: 'M'}
		switch {
		case fd.IsNew():
			it.code = 'A'
		case fd.IsDeleted():
			it.code = 'D'
		case fd.IsRename():
			it.code, it.detail = 'R', fd.OldPath
		}
		if fd.Binary {
			it.note = errBinary.Error()
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

func (t *patchTab) view(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		switch {
		case t.err != nil:
			errorState(c, "Cannot load the changes", t.err, t.reload)
		case t.loading && t.items == nil:
			loadingState(c, "Loading changes")
		case len(t.items) == 0:
			emptyState(c, "No changes", "")
		default:
			ui.SplitPercent(c, &t.split, func() { t.listView(c) }, func() { t.detailView(c) }).Fill()
		}
	})
}

func (t *patchTab) listView(c *ui.Context) {
	items := t.items
	t.list.Key = func(i int) any { return items[i].key(i) }
	t.list.Label = func(i int) string { return items[i].title }
	t.list.Header = func(i int) bool { return items[i].header }
	t.list.Selected = &t.selected
	list.View(c.Key("items"), &t.list, len(items), list.ViewOptions{Label: t.title}, func(i int) list.Item {
		it := items[i]
		switch {
		case it.header:
			return list.Item{Label: it.title, Strong: true, Description: it.detail, Icon: icons.Commit}
		case it.commit != nil:
			return list.Item{Label: it.title, Icon: icons.Commit, Description: shortHash(it.commit.OID)}
		}
		item := changedFileItem(it.title, it.code, func(preview, focus bool) {
			if it.src != nil {
				t.g.openDiffWith(*it.src, preview, focus)
			}
		})
		item.Description = it.detail
		item.OnClick = nil
		return item
	})
}

func (t *patchTab) detailView(c *ui.Context) {
	if t.selected < 0 || t.selected >= len(t.items) {
		emptyState(c, "Select a file", "Its diff shows here.")
		return
	}
	it := t.items[t.selected]
	switch {
	case it.header:
		emptyState(c, it.title, it.detail)
	case it.note != "":
		emptyState(c, it.note, "")
	case it.commit != nil:
		cm := *it.commit
		ui.Scroll(c).Fill().Children(func() {
			t.g.commitInfo(c, cm, nil, nil)
			ui.Row(c).Padding(token.Space0.Px(), token.PadView.Px()).Children(func() {
				if button(c, "Show in Log", false) {
					t.g.showCommit(cm.OID)
				}
			})
		})
	case it.src != nil:
		d := t.g.diffTabFor(*it.src)
		if d.state == nil && !d.loading && d.err == nil {
			d.load()
		}
		d.view(c)
	case it.patch != nil:
		t.patchView(c, it)
	}
}

func (t *patchTab) patchView(c *ui.Context, it patchItem) {
	mode := diffmode.Session.Whitespace
	if t.shown != t.selected || t.state == nil || t.shownWS != mode {
		t.dropState()
		fp := diffmode.FilterPatch(*it.patch, mode)
		d := &editor.DiffState{RenderFindBar: findwidget.Render}
		d.SetPatch(fp)
		abs := t.g.paths.abs(it.title)
		if d.New != nil {
			lang := t.g.h.Language(abs, d.New.String())
			d.New.SetLanguage(lang)
			if d.Old != nil {
				d.Old.SetLanguage(lang)
			}
		}
		t.engine = diffmode.NewEngine(t.g.ctx, d, t.g.post)
		t.engine.LineNumber = diffmode.PatchLineNumbers(fp)
		t.engine.Install(nil)
		t.state, t.shown, t.shownWS = d, t.selected, mode
	}
	t.engine.Refresh()
	t.state.Layout = editor.DiffLayout(t.layout)
	t.state.Annotations = t.engine.Annotations(nil)
	t.g.h.ConfigureDiff(t.state, t.g.paths.abs(it.title))
	t.g.diffOpts.apply(t.state)
	ui.Column(c).Fill().Children(func() {
		toolbarRow(c, "Diff Toolbar", func() {
			secondary(c, it.title).SingleLine().Shrink(1).MinWidth(0).TruncateStart()
			design.Spacer(c)
			action.Segmented(c, "Diff Layout", &t.layout, layoutNames, action.SegmentedOptions{})
			diffModeControls(c, t.engine)
			t.g.diffOptionsButton(c)
		})
		if len(t.state.Hunks()) == 0 && mode != git.WhitespaceNone {
			emptyState(c, "Only whitespace changed", "Choose \"Do not ignore\" to see the changes.")
			return
		}
		editor.DiffView(c.Key("patch"), t.state).Grow(1).FillWidth()
	})
}
