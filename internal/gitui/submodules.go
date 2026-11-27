package gitui

import (
	"context"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type submoduleState struct {
	listState[git.Submodule]
}

const submoduleClean byte = ' '

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
	g.submodules = submoduleState{listState: openList[git.Submodule]()}
	g.loadSubmodules()
}

func (g *Git) loadSubmodules() {
	s := &g.submodules
	s.gen++
	gen, r := s.begin(), g.repo
	applog.Go(func() {
		items, err := r.Submodules(g.ctx)
		g.post(func() { s.apply(gen, items, err) })
	})
}

func (g *Git) submodulesDialog(c *ui.Context) {
	s := &g.submodules
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Submodules", token.DialogWidth, func() {
		switch {
		case s.err != nil:
			errorState(c, "Cannot list the submodules", s.err, g.loadSubmodules)
		case s.loading && s.items == nil:
			boxedSkeleton(c, "Loading submodules", func(i int) list.Item {
				return submoduleItem(git.Submodule{Path: placeholderPath(i), OID: placeholderOID(i), Status: submoduleClean})
			})
		case len(s.items) == 0:
			emptyState(c, "No submodules", "This repository has no submodules.")
		default:
			items := s.items
			s.list.Key = func(i int) any { return items[i].Path }
			s.list.Label = func(i int) string { return items[i].Path }
			s.list.Selected = &s.selected
			boxedList(c, "Submodules", &s.list, len(items), func(i int) list.Item { return submoduleItem(items[i]) })
		}
		busy := g.busy != ""
		dialog.Actions(c, func() {
			if button(c, "Update", len(s.items) == 0 || busy) {
				g.updateSubmodules()
			}
			design.Spacer(c)
			if button(c, "Close", false) {
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

func submoduleItem(m git.Submodule) list.Item {
	it := list.Item{Label: m.Path, Icon: icons.Folder, Description: shortHash(m.OID)}
	status := submoduleStatus(m.Status)
	it.Trailing = func(c *ui.Context) {
		if m.Status != submoduleClean && m.Status != 0 {
			badge.Tag(c, status, badge.TagOptions{Tone: token.ToneWarning})
			return
		}
		secondary(c, status).SingleLine()
	}
	return it
}
