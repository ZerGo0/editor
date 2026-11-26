package workspace

import (
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo/ui"
)

func (w *Workspace) View(c *ui.Context) {
	c.SetZoom(w.zoom())
	vibrant := c.Vibrancy()
	if vibrant {
		c.Root().Background(ui.Transparent)
	}
	w.syncEdited()
	w.updateProblems()
	c.Root().ObserveInput(w.observeInput)
	w.shortcuts(c)
	w.focusedTab = nil
	ui.Column(c).Fill().Children(func() {
		w.titleBar(c, vibrant)
		ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			w.activityBar(c, vibrant)
			if w.sidebarHidden {
				w.main(c)
				return
			}
			ui.Split(c, &w.sidebarWidth, func() { w.sidebar(c, vibrant) }, func() { w.main(c) }).
				SplitReset(defaultSidebarWidth).Grow(1).FillHeight()
		})
		w.statusBar(c, vibrant)
	})
	w.endViewerFrame()
	w.gitFrame()
	w.autoSave(w.focusedTab)
	w.overlays(c)
	paletteOpen := w.palette.IsOpen()
	w.palette.View(c, w.paletteTop(), w.acceptPalette)
	if paletteOpen && !w.palette.IsOpen() && w.focusTab == nil {
		w.focusTab = w.active.ActiveTab()
		c.Invalidate()
	}
	w.promptView(c)
}

func (w *Workspace) overlays(c *ui.Context) {
	if w.git != nil {
		w.git.Overlay(c)
	}
	if w.prs != nil {
		w.prs.Overlay(c)
	}
}

func (w *Workspace) syncEdited() {
	edited := w.HasUnsaved()
	if edited == w.edited || w.win == nil {
		return
	}
	w.edited = edited
	w.win.SetDocumentEdited(edited)
}

func (w *Workspace) main(c *ui.Context) {
	ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
		w.withPanel(c, func() { w.node(c, w.layout) })
	})
}

func (w *Workspace) node(c *ui.Context, n *layout.Node) {
	if n.Group != nil {
		w.group(c, n.Group)
		return
	}
	first := func() { w.node(c, n.First) }
	second := func() { w.node(c, n.Second) }
	if n.Vertical {
		ui.SplitVerticalPercent(c, &n.Percent, first, second).SplitReset(halfPercent).Fill()
		return
	}
	ui.SplitPercent(c, &n.Percent, first, second).SplitReset(halfPercent).Fill()
}
