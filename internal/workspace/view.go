package workspace

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
)

var appearances = map[string]design.Appearance{
	settings.ThemeLight: design.AppearanceLight,
	settings.ThemeDark:  design.AppearanceDark,
}

func ApplyTheme(c *ui.Context, s *settings.Store) *token.Palette {
	return design.Apply(c, design.Settings{Appearance: appearances[s.String(settings.ColorTheme)], SystemAccent: true})
}

func (w *Workspace) View(c *ui.Context) {
	c.SetZoom(w.zoom())
	p := ApplyTheme(c, w.env.Settings)
	c.Root().Background(p.Layer.Editor)
	w.syncEdited()
	w.syncTitle()
	w.updateProblems()
	w.beginDragFrame()
	c.Root().ObserveInput(w.observeInput)
	w.escape(c)
	w.shortcuts(c)
	w.focusedTab = nil
	w.trackNavigation()
	ui.Column(c).Fill().Children(func() {
		w.titleBar(c)
		ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			if w.sidebarHidden {
				w.main(c)
				return
			}
			w.clampSidebar(c)
			ui.Split(c, &w.sidebarWidth, func() { w.sidebar(c) }, func() { w.main(c) }).
				SplitReset(defaultSidebarWidth).Grow(1).FillHeight().MinWidth(0)
		})
		w.statusBar(c)
	})
	w.endViewerFrame()
	w.gitFrame()
	w.autoSave(w.focusedTab)
	w.syncBackups()
	w.overlays(c)
	w.quickInputView(c)
	w.promptView(c)
	w.showNotices(c)
	feedback.ToastHost(c)
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
	ui.Column(c).Fill().MinWidth(0).Background(token.Of(c).Layer.Editor).Children(func() {
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
