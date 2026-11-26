package gitui

import (
	"context"
	"errors"
	"fmt"

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
	if !g.ready() {
		return
	}
	t := g.diffTabFor(src)
	t.load()
	g.h.OpenView(src.id(), src.title(), t.view)
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
	tc := c.Theme()
	ui.Row(c).FillWidth().Height(toolbarHeight).Shrink(0).Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, t.src.path).FontSize(smallFont).SingleLine().Shrink(1).MinWidth(0).TruncateStart()
		if t.loading {
			ui.Spinner(c).Size(iconSize, iconSize).Label("Loading diff")
		}
		ui.Box(c).Grow(1)
		if d := t.state; d != nil {
			added, deleted := d.Stats()
			muted(c, fmt.Sprintf("+%d \u2212%d", added, deleted)).SingleLine()
			hunks := d.Hunks()
			i, _ := d.CurrentHunk()
			label := fmt.Sprintf("%d changes", len(hunks))
			if i >= 0 && len(hunks) > 0 {
				label = fmt.Sprintf("%d of %d", i+1, len(hunks))
			}
			muted(c, label).SingleLine()
			if textButton(c, "Previous", i <= 0).Clicked() {
				d.GoToHunk(i - 1)
			}
			if textButton(c, "Next", i >= len(hunks)-1).Clicked() {
				d.GoToHunk(i + 1)
			}
		}
		ui.Segmented(c, &t.layout, layoutNames...).Label("Diff Layout")
		diffmode.Controls(c, t.engine)
		if t.engine != nil && t.engine.Filtered() && (t.live || t.hasHunkActions()) {
			muted(c, readOnlyNote).SingleLine().Shrink(0)
		}
		t.g.diffOptionsButton(c)
		t.fileActions(c)
		if iconButton(c, "Reload Diff", icons.Refresh, t.loading).Clicked() {
			t.load()
		}
	})
}

func (t *diffTab) fileActions(c *ui.Context) {
	g, rel := t.g, t.src.path
	busy := g.busy != ""
	switch t.src.kind {
	case diffWorkTree, diffUntracked:
		if textButton(c, "Stage File", busy).Clicked() {
			g.doSimple("Stage", func(ctx context.Context, r repo) error { return r.StageFiles(ctx, rel) }, t.load)
		}
	case diffStaged:
		if textButton(c, "Unstage File", busy).Clicked() {
			g.doSimple("Unstage", func(ctx context.Context, r repo) error { return r.UnstageFiles(ctx, rel) }, t.load)
		}
	}
}

func (t *diffTab) hunkActions(c *ui.Context, bar ui.Element, a *editor.DiffHunkActions) {
	busy := t.g.busy != ""
	bar.Gap(4).Children(func() {
		switch t.src.kind {
		case diffWorkTree:
			if ui.Button(c, "Revert").Disabled(busy).Clicked() {
				t.revertHunk(a)
			}
			if ui.PrimaryButton(c, "Stage").Disabled(busy).Clicked() {
				t.applyHunk(a.Hunk, false, true, false)
			}
		case diffStaged:
			if ui.Button(c, "Unstage").Disabled(busy).Clicked() {
				t.applyHunk(a.Hunk, true, true, true)
			}
		case diffHead:
			if ui.Button(c, "Revert").Disabled(busy).Clicked() {
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
