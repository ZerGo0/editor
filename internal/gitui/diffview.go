package gitui

import (
	"context"
	"errors"
	"fmt"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/plural"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	maxKeptTabs  = 32
	readOnlyNote = "Read only"
)

const (
	layoutSideBySide = iota
	layoutUnified
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
	bar     diffBar
	ops     *hunkOps
	focused bool
	focus   bool
}

func (g *Git) openDiff(kind diffKind, rel string) {
	g.openDiffSource(diffSource{kind: kind, path: rel})
}

func (g *Git) openDiffSource(src diffSource) {
	g.openSingleDiff(src, false, true)
}

func (g *Git) openDiffWith(src diffSource, preview, focus bool) {
	switch src.kind {
	case diffWorkTree, diffUntracked:
		g.openGroupAt(groupChanges, src.path, preview, focus)
	case diffStaged:
		g.openGroupAt(groupStaged, src.path, preview, focus)
	case diffCommit:
		g.openCommitTabAt(git.Commit{OID: src.rev}, src.path, preview, focus)
	default:
		g.openSingleDiff(src, preview, focus)
	}
}

func (g *Git) openSingleDiff(src diffSource, preview, focus bool) {
	if !g.ready() {
		return
	}
	t := g.diffTabFor(src)
	t.focus = t.focus || focus
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
	t := &diffTab{g: g, src: src, layout: g.diffOpts.layout}
	t.ops = &hunkOps{g: g, src: src, live: func() bool { return t.live }, reload: t.load}
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
	applog.Go(func() {
		if src.kind == diffCommit && src.parent == "" {
			src.parent = commitParent(g.ctx, r, src.rev)
		}
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
	})
}

func (t *diffTab) apply(oldText, newText string, live *editor.Buffer) {
	if t.state == nil || t.liveBuf != live {
		t.dropState()
		d := &editor.DiffState{Layout: editor.DiffLayout(t.layout), SplitPercent: t.g.layout.DiffSplit}
		t.ops.d = d
		d.RenderHunkActions = t.ops.render
		d.RenderFindBar = findwidget.Render
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

func (t *diffTab) view(c *ui.Context) {
	if t.g.unavailable(c) {
		return
	}
	if t.state == nil && !t.loading && t.err == nil {
		t.load()
	}
	if t.live && t.state != nil && t.g.h.Buffer(t.abs()) != t.liveBuf && !t.loading {
		t.dropState()
		t.load()
	}
	if t.engine != nil {
		t.engine.Refresh()
	}
	col := ui.Column(c).Fill()
	t.focused = col.FocusWithin()
	col.Children(func() {
		before := currentDiffChoice(t.layout)
		t.toolbar(c)
		t.g.noteDiffChoice(before, t.layout)
		t.body(c)
	})
}

func (t *diffTab) body(c *ui.Context) {
	switch {
	case t.err != nil && (errors.Is(t.err, errBinary) || errors.Is(t.err, errTooLarge)):
		emptyState(c, "Cannot show the diff", t.err.Error())
	case t.err != nil:
		errorState(c, "Cannot load the diff", t.err, t.load)
	case t.state == nil || !t.engine.Ready():
		feedback.DiffSkeleton(c, "Loading diff", multidiff.CountUnknown, multidiff.CountUnknown, t.configure)
	default:
		t.configure(t.state)
		t.state.HunkActions = t.ops.enabled() && !t.engine.Filtered()
		t.state.Annotations = t.engine.Annotations(nil)
		if t.focus {
			t.state.Focus()
			t.focus = false
		}
		split := t.state.SplitPercent
		t.diffWithBars(c)
		if t.state.SplitPercent != split {
			t.g.layout.DiffSplit = t.state.SplitPercent
		}
	}
}

func (t *diffTab) configure(d *editor.DiffState) {
	d.Layout = editor.DiffLayout(t.layout)
	t.g.h.ConfigureDiff(d, t.abs())
	t.g.diffOpts.apply(d)
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
			label := plural.Count(len(hunks), "change", "changes")
			if i >= 0 && len(hunks) > 0 {
				label = fmt.Sprintf("%d of %d", i+1, len(hunks))
			}
			secondary(c, label).SingleLine().Shrink(0)
			t.changeButton(c, previousChangeCommand, "Previous Change", icons.ChevronUp, -1)
			t.changeButton(c, nextChangeCommand, "Next Change", icons.ChevronDown, 1)
		}
		pane.Separator(c, true)
		action.Segmented(c, "Diff Layout", &t.layout, layoutNames, action.SegmentedOptions{})
		diffModeControls(c, t.engine)
		if t.engine != nil && t.engine.Filtered() && (t.live || t.ops.enabled()) {
			secondary(c, readOnlyNote).SingleLine().Shrink(0)
		}
		t.g.diffOptionsButton(c)
		t.fileActions(c)
		action.CommandIconButton(c, icons.File, openFileCommand)
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
