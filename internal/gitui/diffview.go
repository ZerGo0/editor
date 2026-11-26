package gitui

import (
	"context"
	"errors"
	"fmt"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	maxKeptTabs  = 32
	readOnlyNote = "Read only"
)

var layoutNames = []string{"Side by Side", "Unified"}

type diffTab struct {
	g       *Git
	src     diffSource
	state   *editor.DiffState
	engine  *diffmode.Engine
	live    bool
	liveBuf *editor.Buffer
	layout  int
	loading bool
	err     error
	gen     int
}

func (g *Git) openDiff(kind diffKind, rel string) {
	g.openDiffSource(diffSource{kind: kind, path: rel})
}

func (g *Git) openDiffSource(src diffSource) {
	g.openDiffWith(src, false, true)
}

func (g *Git) openDiffWith(src diffSource, preview, focus bool) {
	if !g.ready() {
		return
	}
	t := g.diffTabFor(src)
	t.load()
	g.openView(src.id(), src.title(), t.view, preview, focus)
}

type viewOpener interface {
	OpenViewWith(id, title string, view func(c *ui.Context), preview, focus bool)
}

func (g *Git) openView(id, title string, view func(c *ui.Context), preview, focus bool) {
	if o, ok := g.h.(viewOpener); ok {
		o.OpenViewWith(id, title, view, preview, focus)
		return
	}
	g.h.OpenView(id, title, view)
}

func (g *Git) diffTabFor(src diffSource) *diffTab {
	id := src.id()
	if t, ok := g.tabs[id]; ok {
		return t
	}
	t := &diffTab{g: g, src: src}
	g.tabs[id] = t
	g.tabOrder = append(g.tabOrder, id)
	if len(g.tabOrder) > maxKeptTabs {
		if old := g.tabs[g.tabOrder[0]]; old.engine != nil {
			old.engine.Close()
		}
		delete(g.tabs, g.tabOrder[0])
		g.tabOrder = g.tabOrder[1:]
	}
	return t
}

func (t *diffTab) abs() string {
	return t.g.paths.abs(t.src.path)
}

func (t *diffTab) load() {
	g := t.g
	if !g.ready() {
		return
	}
	t.gen++
	gen := t.gen
	t.loading, t.err = true, nil
	var live *editor.Buffer
	if t.src.usesWorkTree() {
		live = g.h.Buffer(t.abs())
	}
	r, src, abs := g.repo, t.src, t.abs()
	go func() {
		oldText, newText, err := loadDiffTexts(g.ctx, r, src, abs, live == nil)
		g.post(func() {
			if gen != t.gen {
				return
			}
			t.loading, t.err = false, err
			if err == nil {
				t.apply(oldText, newText, live)
			}
		})
	}()
}

func (t *diffTab) apply(oldText, newText string, live *editor.Buffer) {
	if t.state == nil || t.liveBuf != live {
		t.dropState()
		d := &editor.DiffState{Layout: editor.DiffLayout(t.layout)}
		d.RenderHunkActions = t.hunkActions
		t.engine = diffmode.NewEngine(t.g.ctx, d, t.g.post)
		t.engine.Install(nil)
		t.state, t.live, t.liveBuf = d, live != nil, live
	}
	t.engine.SetSource(diffmode.Source{Old: oldText, New: newText, Live: live, Language: t.g.h.Language(t.abs(), newText)})
}

func (t *diffTab) dropState() {
	if t.engine != nil {
		t.engine.Close()
	}
	t.state, t.engine, t.live, t.liveBuf = nil, nil, false, nil
}

func (t *diffTab) hasHunkActions() bool {
	switch t.src.kind {
	case diffWorkTree, diffStaged, diffHead:
		return true
	}
	return false
}

func (t *diffTab) view(c *ui.Context) {
	if t.live && t.state != nil && t.g.h.Buffer(t.abs()) != t.liveBuf && !t.loading {
		t.dropState()
		t.load()
	}
	if t.engine != nil {
		t.engine.Refresh()
	}
	ui.Column(c).Fill().Children(func() {
		t.toolbar(c)
		t.body(c)
	})
}

func (t *diffTab) body(c *ui.Context) {
	switch {
	case t.err != nil && (errors.Is(t.err, errBinary) || errors.Is(t.err, errTooLarge)):
		emptyState(c, t.err.Error(), "")
	case t.err != nil:
		errorState(c, "Cannot load the diff", t.err, t.load)
	case t.state == nil || !t.engine.Ready():
		loadingState(c, "Loading diff")
	default:
		t.state.Layout = editor.DiffLayout(t.layout)
		t.g.diffOpts.apply(t.state)
		t.state.HunkActions = t.hasHunkActions() && !t.engine.Filtered()
		t.state.Annotations = t.engine.Annotations(nil)
		editor.DiffView(c.Key("diff"), t.state).Grow(1).FillWidth()
	}
}

func (t *diffTab) toolbar(c *ui.Context) {
	toolbarRow(c, "Diff Toolbar", func() {
		secondary(c, t.src.path).SingleLine().Shrink(1).MinWidth(0).TruncateStart().Tooltip(t.src.path)
		if t.loading {
			feedback.Spinner(c, "Loading diff")
		}
		design.Spacer(c)
		if d := t.state; d != nil {
			added, deleted := d.Stats()
			secondary(c, fmt.Sprintf("+%d \u2212%d", added, deleted)).SingleLine().Shrink(0)
			hunks := d.Hunks()
			i, _ := d.CurrentHunk()
			label := fmt.Sprintf("%d changes", len(hunks))
			if i >= 0 && len(hunks) > 0 {
				label = fmt.Sprintf("%d of %d", i+1, len(hunks))
			}
			secondary(c, label).SingleLine().Shrink(0)
			if iconButton(c, "Previous Change", icons.ChevronUp, i <= 0) {
				d.GoToHunk(i - 1)
			}
			if iconButton(c, "Next Change", icons.ChevronDown, i >= len(hunks)-1) {
				d.GoToHunk(i + 1)
			}
		}
		pane.Separator(c, true)
		action.Segmented(c, "Diff Layout", &t.layout, layoutNames, action.SegmentedOptions{})
		diffModeControls(c, t.engine)
		if t.engine != nil && t.engine.Filtered() && (t.live || t.hasHunkActions()) {
			secondary(c, readOnlyNote).SingleLine().Shrink(0)
		}
		t.g.diffOptionsButton(c)
		t.fileActions(c)
		if iconButton(c, "Reload Diff", icons.Refresh, t.loading) {
			t.load()
		}
	})
}

func (t *diffTab) fileActions(c *ui.Context) {
	g, rel := t.g, t.src.path
	busy := g.busy != ""
	switch t.src.kind {
	case diffWorkTree, diffUntracked:
		if iconButton(c, "Stage File", icons.Plus, busy) {
			g.doSimple("Stage", func(ctx context.Context, r repo) error { return r.StageFiles(ctx, rel) }, t.load)
		}
	case diffStaged:
		if iconButton(c, "Unstage File", icons.Minus, busy) {
			g.doSimple("Unstage", func(ctx context.Context, r repo) error { return r.UnstageFiles(ctx, rel) }, t.load)
		}
	}
}

func (t *diffTab) hunkActions(c *ui.Context, bar ui.Element, a *editor.DiffHunkActions) {
	busy := t.g.busy != ""
	bar.Gap(token.GapInline.Px()).Children(func() {
		switch t.src.kind {
		case diffWorkTree:
			if smallButton(c, "Revert", busy) {
				t.revertHunk(a)
			}
			if action.Button(c, "Stage", action.ButtonOptions{Variant: action.Primary, Small: true, Disabled: busy}).Clicked() && !busy {
				t.applyHunk(a.Hunk, false, true, false)
			}
		case diffStaged:
			if smallButton(c, "Unstage", busy) {
				t.applyHunk(a.Hunk, true, true, true)
			}
		case diffHead:
			if smallButton(c, "Revert", busy) {
				t.revertHunk(a)
			}
		}
	})
}

func (t *diffTab) revertHunk(a *editor.DiffHunkActions) {
	if t.live {
		a.Revert()
		return
	}
	t.applyHunk(a.Hunk, true, false, true)
}

func (t *diffTab) applyHunk(h editor.DiffHunk, contextFromNew, cached, reverse bool) {
	d := t.state
	if d == nil {
		return
	}
	patch := hunkPatch(t.src.path, d.Old.String(), d.New.String(), h, contextFromNew)
	if patch == nil {
		t.g.h.Notify("This change cannot be applied on its own.")
		return
	}
	t.g.doSimple("Apply Change", func(ctx context.Context, r repo) error {
		return r.Apply(ctx, patch, cached, reverse)
	}, t.load)
}
