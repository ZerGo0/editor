package gitui

import (
	"context"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type hunkOps struct {
	g      *Git
	src    diffSource
	d      *editor.DiffState
	live   func() bool
	reload func()
}

func (h *hunkOps) enabled() bool {
	switch h.src.kind {
	case diffWorkTree, diffStaged, diffHead:
		return true
	}
	return false
}

func (h *hunkOps) render(c *ui.Context, bar ui.Element, a *editor.DiffHunkActions) {
	busy := h.g.busy != ""
	bar.Gap(token.GapInline.Px()).Children(func() {
		switch h.src.kind {
		case diffWorkTree:
			if smallButton(c, "Revert", busy) {
				h.revert(a)
			}
			if action.Button(c, "Stage", action.ButtonOptions{Variant: action.Primary, Small: true, Disabled: busy}).Clicked() && !busy {
				h.apply(a.Hunk, false, true, false)
			}
		case diffStaged:
			if smallButton(c, "Unstage", busy) {
				h.apply(a.Hunk, true, true, true)
			}
		case diffHead:
			if smallButton(c, "Revert", busy) {
				h.revert(a)
			}
		}
	})
}

func (h *hunkOps) revert(a *editor.DiffHunkActions) {
	if h.live() {
		a.Revert()
		return
	}
	h.apply(a.Hunk, true, false, true)
}

func (h *hunkOps) apply(hunk editor.DiffHunk, contextFromNew, cached, reverse bool) {
	d := h.d
	if d == nil {
		return
	}
	patch := hunkPatch(h.src.path, d.Old.String(), d.New.String(), hunk, contextFromNew)
	if patch == nil {
		h.g.h.Notify("This change cannot be applied on its own.")
		return
	}
	h.g.doSimple("Apply Change", func(ctx context.Context, r repo) error {
		return r.Apply(ctx, patch, cached, reverse)
	}, h.reload)
}
