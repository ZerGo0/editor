package gitui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	arrowColumn   = 26
	arrowSize     = 20
	paneHeader    = 24
	mergeFileMode = 0o644
)

type mergeTool struct {
	g        *Git
	rel      string
	id       string
	loading  bool
	err      error
	gen      int
	model    *mergeModel
	ours     *editor.State
	result   *editor.State
	theirs   *editor.State
	version  uint64
	current  int
	rebasing bool
}

func (g *Git) openMergeTool(rel string) {
	if !g.ready() {
		return
	}
	id := "git-merge:" + rel
	t, ok := g.mergeTools[id]
	if !ok {
		t = &mergeTool{g: g, rel: rel, id: id}
		g.mergeTools[id] = t
	}
	t.rebasing = g.rebasing()
	t.load()
	name, _ := splitPath(rel)
	g.h.OpenView(id, "Merge: "+name, t.view)
}

func (t *mergeTool) load() {
	g := t.g
	t.gen++
	gen, r, rel := t.gen, g.repo, t.rel
	t.loading, t.err = true, nil
	go func() {
		base, ours, theirs, err := r.MergeVersions(g.ctx, rel)
		if err == nil {
			for _, data := range [][]byte{base, ours, theirs} {
				if err = checkText(data); err != nil {
					break
				}
			}
		}
		g.post(func() {
			if gen != t.gen {
				return
			}
			t.loading, t.err = false, err
			if err == nil {
				t.setup(string(base), string(ours), string(theirs))
			}
		})
	}()
}

func (t *mergeTool) setup(base, ours, theirs string) {
	lang := t.g.h.Language(t.g.paths.abs(t.rel), ours)
	pane := func(text string, readOnly bool) *editor.State {
		buf := editor.NewBuffer(text)
		buf.SetLanguage(lang)
		return &editor.State{Buffer: buf, ReadOnly: readOnly}
	}
	t.model = newMergeModel(base, ours, theirs)
	t.ours, t.theirs = pane(ours, true), pane(theirs, true)
	t.result = pane(t.model.result(), false)
	t.version = t.result.Buffer.Version()
	t.current = 0
}

func (t *mergeTool) sideNames() (string, string) {
	if t.rebasing {
		return "Upstream (rebasing onto)", "Your commit"
	}
	return "Yours (local)", "Theirs (incoming)"
}

func (t *mergeTool) regenerate() {
	text := t.model.result()
	t.result.Buffer.Set(text)
	t.version = t.result.Buffer.Version()
}

func (t *mergeTool) act(fn func()) {
	if t.result.Buffer.Version() == t.version {
		fn()
		t.regenerate()
		return
	}
	t.g.confirm(question{
		message:     "Discard your manual edits of the result?",
		detail:      "This action builds the result again from the chosen changes.",
		action:      "Discard Edits",
		destructive: true,
	}, func() {
		fn()
		t.regenerate()
	})
}

func (t *mergeTool) view(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		switch {
		case t.err != nil && (errors.Is(t.err, errBinary) || errors.Is(t.err, errTooLarge)):
			emptyState(c, t.err.Error(), "Use Accept Yours or Accept Theirs in the Conflicts dialog.")
		case t.err != nil:
			errorState(c, "Cannot load the conflict", t.err, t.load)
		case t.model == nil:
			loadingState(c, "Loading conflict")
		default:
			t.toolbar(c)
			t.panes(c)
		}
	})
}

func (t *mergeTool) toolbar(c *ui.Context) {
	m := t.model
	changes := m.changes()
	busy := t.g.busy != ""
	ui.Row(c).FillWidth().Height(toolbarHeight).Shrink(0).Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
		left := m.unresolved()
		status := "All conflicts are resolved"
		if left > 0 {
			status = fmt.Sprintf("%d conflicts left", left)
		}
		ui.Text(c, status).FontSize(smallFont).SingleLine().Shrink(0)
		if textButton(c, "Previous Change", len(changes) == 0).Clicked() {
			t.move(changes, -1)
		}
		if textButton(c, "Next Change", len(changes) == 0).Clicked() {
			t.move(changes, 1)
		}
		ui.Box(c).Grow(1)
		if textButton(c, "Apply Non-Conflicting Changes", false).Clicked() {
			t.act(func() { m.applyNonConflicting(true, true) })
		}
		if textButton(c, "Accept Left", false).Clicked() {
			t.act(func() { m.acceptAll(true) })
		}
		if textButton(c, "Accept Right", false).Clicked() {
			t.act(func() { m.acceptAll(false) })
		}
		if ui.PrimaryButton(c, "Apply").Disabled(busy).Clicked() {
			t.apply()
		}
	})
}

func (t *mergeTool) move(changes []int, step int) {
	if len(changes) == 0 {
		return
	}
	pos := 0
	for i, ch := range changes {
		if ch == t.current {
			pos = (i + step + len(changes)) % len(changes)
			break
		}
	}
	t.current = changes[pos]
	r := t.model.ranges(sideResult)[t.current]
	t.result.ScrollTo(r.start)
}

func (t *mergeTool) chunkColor(c *ui.Context, i int) ui.Color {
	th := c.Theme()
	ch := t.model.chunks[i]
	switch {
	case ch.Kind == git.ChunkConflict && t.model.choices[i] == choiceNone:
		return th.Danger.Alpha(tintAlpha)
	case t.model.choices[i] != choiceNone:
		return th.Success.Alpha(tintAlpha)
	}
	return th.Accent.Alpha(tintAlpha)
}

func (t *mergeTool) backgrounds(c *ui.Context, side mergeSide) []editor.LineBackground {
	ranges := t.model.ranges(side)
	var out []editor.LineBackground
	for _, i := range t.model.changes() {
		r := ranges[i]
		if r.end > r.start {
			out = append(out, editor.LineBackground{First: r.start, Last: r.end - 1, Color: t.chunkColor(c, i)})
		}
	}
	return out
}

func (t *mergeTool) panes(c *ui.Context) {
	left, right := t.sideNames()
	t.ours.LineBackgrounds = t.backgrounds(c, sideOurs)
	t.theirs.LineBackgrounds = t.backgrounds(c, sideTheirs)
	if t.result.Buffer.Version() == t.version {
		t.result.LineBackgrounds = t.backgrounds(c, sideResult)
	}
	ui.Row(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		t.pane(c, left, t.ours)
		t.arrows(c, t.ours, sideOurs, "\u00bb", choiceOurs)
		t.pane(c, "Result", t.result)
		t.arrows(c, t.theirs, sideTheirs, "\u00ab", choiceTheirs)
		t.pane(c, right, t.theirs)
	})
}

func (t *mergeTool) pane(c *ui.Context, title string, st *editor.State) {
	ui.Column(c.Key(title)).Grow(1).Basis(0).FillHeight().MinWidth(0).Children(func() {
		ui.Row(c).Height(paneHeader).FillWidth().Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
			sectionTitle(c, title)
		})
		editor.View(c.Key("editor"), st).Grow(1).FillWidth().MinHeight(0).Label(title)
	})
}

func (t *mergeTool) arrows(c *ui.Context, st *editor.State, side mergeSide, glyph string, choice mergeChoice) {
	ranges := t.model.ranges(side)
	ui.Column(c.Key(side)).Width(arrowColumn).FillHeight().Shrink(0).Children(func() {
		ui.Box(c).Height(paneHeader).Shrink(0)
		ui.Box(c).Grow(1).FillWidth().Clip().Children(func() {
			for _, i := range t.model.changes() {
				r := ranges[i]
				if r.end == r.start && t.model.chunks[i].Kind != git.ChunkConflict {
					continue
				}
				y := float32(st.LineTop(r.start) - st.ScrollY())
				if y < 0 {
					continue
				}
				label := fmt.Sprintf("Accept change %d from %s", i+1, sideLabel(side))
				b := ui.ButtonBase(c.Key(i)).Absolute().Top(y).Left((arrowColumn-arrowSize)/2).Size(arrowSize, arrowSize).
					Radius(buttonRadius).Center().Label(label).Tooltip(label)
				if b.Hovered() {
					b.Background(c.Theme().SurfaceHover)
				}
				b.Children(func() { ui.Text(c, glyph).FontSize(bodyFont).Bold() })
				if b.Clicked() {
					idx := i
					t.current = idx
					t.act(func() { t.model.choose(idx, choice) })
				}
				b.ContextMenu(func(m *ui.Menu) {
					idx := i
					if m.Item("Ignore Change").Chosen() {
						t.act(func() { t.model.choose(idx, choiceNone) })
					}
				})
			}
		})
	})
}

func sideLabel(side mergeSide) string {
	if side == sideOurs {
		return "left"
	}
	return "right"
}

func (t *mergeTool) apply() {
	left := t.model.unresolved()
	if t.result.Buffer.Version() != t.version {
		left = 0
	}
	if left == 0 {
		t.save()
		return
	}
	t.g.confirm(question{
		message: fmt.Sprintf("%d conflicts are not resolved. Save the result and mark the file as resolved?", left),
		detail:  "Unresolved changes keep the base version.",
		action:  "Save Anyway",
	}, t.save)
}

func (t *mergeTool) save() {
	g := t.g
	text := t.result.Buffer.String()
	abs := g.paths.abs(t.rel)
	rel := t.rel
	g.doSimple("Resolve Conflict", func(ctx context.Context, r repo) error {
		mode := fs.FileMode(mergeFileMode)
		if info, err := os.Stat(abs); err == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(abs, []byte(text), mode); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
		return r.MarkResolved(ctx, rel)
	}, func() {
		if buf := g.h.Buffer(abs); buf != nil {
			buf.Reset(text)
		}
		delete(g.mergeTools, t.id)
		g.h.CloseView(t.id)
		g.h.Notify("Resolved " + rel)
		if g.conflictCount() <= 1 && g.snap.op.Kind != git.StateNone {
			g.h.Notify("All conflicts are resolved. Continue the " + g.snap.op.Kind.String() + ".")
		}
	})
}
