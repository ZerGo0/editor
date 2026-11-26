package gitui

import (
	"context"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

type submoduleState struct {
	open     bool
	loading  bool
	err      error
	gen      int
	items    []git.Submodule
	list     ui.ListState
	selected int
}

func submoduleStatus(code byte) string {
	switch code {
	case '-':
		return "Not initialized"
	case '+':
		return "Other commit checked out"
	case 'U':
		return "Merge conflicts"
	}
	return "Up to date"
}

func (g *Git) openSubmodules() {
	if !g.ready() {
		return
	}
	g.submodules = submoduleState{open: true, selected: -1}
	g.loadSubmodules()
}

func (g *Git) loadSubmodules() {
	s := &g.submodules
	s.gen++
	gen, r := s.gen, g.repo
	s.loading, s.err = true, nil
	go func() {
		items, err := r.Submodules(g.ctx)
		g.post(func() {
			if gen != s.gen {
				return
			}
			s.loading, s.err, s.items = false, err, items
		})
	}()
}

func (g *Git) submodulesDialog(c *ui.Context) {
	s := &g.submodules
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Submodules", dialogWidth, func() {
		switch {
		case s.err != nil:
			errorState(c, "Cannot list the submodules", s.err, g.loadSubmodules)
		case s.loading && s.items == nil:
			loadingState(c, "Loading submodules")
		case len(s.items) == 0:
			emptyState(c, "No submodules", "This repository has no submodules.")
		default:
			items := s.items
			s.list.Key = func(i int) any { return items[i].Path }
			s.list.Label = func(i int) string { return items[i].Path }
			s.list.Selected = &s.selected
			ui.List(c.Key("submodules"), &s.list, len(items)).FillWidth().Height(listHeight).Label("Submodules").Rows(func(row ui.ListRow) {
				m := items[row.Index]
				rc := row.Context
				ui.Row(rc).Height(rowHeight).FillWidth().Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(rc, m.Path).FontSize(bodyFont).SingleLine().Grow(1).Shrink(1).MinWidth(0)
					muted(rc, shortHash(m.OID)).SingleLine().Shrink(0)
					color := rc.Theme().TextMuted
					if m.Status != ' ' && m.Status != 0 {
						color = rc.Theme().Warning
					}
					ui.Text(rc, submoduleStatus(m.Status)).FontSize(smallFont).TextColor(color).SingleLine().Shrink(0)
				})
			})
		}
		busy := g.busy != ""
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			if ui.Button(c, "Update").Disabled(len(s.items) == 0 || busy).Clicked() {
				g.updateSubmodules()
			}
			ui.Box(c).Grow(1)
			if ui.Button(c, "Close").Clicked() {
				s.open = false
			}
		})
	})
}

func (g *Git) updateSubmodules() {
	g.do(op{
		title:   "Update Submodules",
		banner:  true,
		network: true,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return git.OpResult{}, r.UpdateSubmodules(ctx, true, true)
		},
		done: func(git.OpResult) {
			g.h.Notify("Updated the submodules")
			g.loadSubmodules()
		},
	})
}
