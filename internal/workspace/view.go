package workspace

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
)

const mainLabel = "Editor"

var appearances = map[string]design.Appearance{
	settings.ThemeLight:     design.AppearanceLight,
	settings.ThemeDark:      design.AppearanceDark,
	settings.ThemeHighLight: design.AppearanceLight,
	settings.ThemeHighDark:  design.AppearanceDark,
}

func ApplyTheme(c *ui.Context, s *settings.Store) *token.Palette {
	theme := s.String(settings.ColorTheme)
	high := theme == settings.ThemeHighDark || theme == settings.ThemeHighLight
	return design.Apply(c, design.Settings{Appearance: appearances[theme], HighContrast: high, FollowHighContrast: s.Bool(settings.AutoDetectContrast), SystemAccent: true})
}

func (w *Workspace) view(c *ui.Context) {
	w.frames.BuildStarted()
	w.runPendingUpdates()
	c.SetZoom(w.zoom())
	p := ApplyTheme(c, w.env.Settings)
	c.Root().Background(p.Layer.Editor).DrawOver(func(*ui.Painter, ui.Rect) { w.frames.Painted() })
	w.syncEdited()
	w.syncTitle()
	w.updateProblems()
	w.codeTheme = p.CodeTheme()
	root := c.Root().TabStop(false).FocusRing(false).Role(ui.RoleNone).ObserveInput(w.observeInput)
	w.keys.rootFocus = root.Focused()
	if w.active.ActiveTab() == nil && !root.FocusWithin() {
		root.Focus()
	}
	w.syncGoToLine()
	w.nextDiffFrame()
	w.shortcuts(c, scopeMain)
	w.focusedTab = nil
	w.trackNavigation()
	ui.Column(c).Fill().ObserveInput(w.observeMainKey).Children(func() {
		w.titleBar(c)
		ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			w.sidebarSplit(c).Grow(1).FillHeight().MinWidth(0)
		})
		w.statusBar(c)
	})
	w.endViewerFrame()
	w.gitFrame()
	w.autoSave(w.focusedTab)
	w.publishViewState()
	w.syncBackups()
	w.syncOutsideWatches()
	w.overlays(c)
	w.holdFocus()
	w.quickInputView(c)
	w.promptView(c)
	w.releaseFocus(c)
	w.showNotices(c)
	w.toastHost(c)
	w.endKeyFrame()
	w.publishStateChange()
}

func (w *Workspace) overlays(c *ui.Context) {
	if w.git != nil {
		w.git.Overlay(c)
	}
	if w.prs != nil {
		w.prs.Overlay(c)
	}
	w.peekView(c)
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
	ui.Column(c).Fill().MinWidth(0).Background(token.Of(c).Layer.Editor).Role(ui.RoleMain).Label(mainLabel).Children(func() {
		w.withPanel(c, func() {
			if g := w.maximizedGroup(); g != nil {
				w.group(c, g)
				return
			}
			w.node(c, w.layout)
		})
	})
}

func (w *Workspace) node(c *ui.Context, n *layout.Node) {
	if n.Group != nil {
		w.group(c, n.Group)
		return
	}
	firstNode, secondNode := n.First, n.Second
	first := func() { w.node(c, firstNode) }
	second := func() { w.node(c, secondNode) }
	w.nodeSplit(c, n, first, second).Fill()
}
