package workspace

import (
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/errtext"
	"github.com/ZerGo0/editor/internal/largefile"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	loadingLabel  = "Loading"
	tryAgainLabel = "Try Again"
)

func (w *Workspace) group(c *ui.Context, g *layout.Group) {
	ui.Column(c.Key(g.ID)).Fill().MinWidth(0).Children(func() {
		tab := g.ActiveTab()
		if tab != nil {
			w.tabBar(c, g)
			w.breadcrumbs(c, g, tab)
		}
		area := ui.Column(c).Grow(1).FillWidth().MinHeight(0).MinWidth(0).Clip()
		area.Children(func() {
			if tab == nil {
				w.watermark(c)
			} else {
				w.content(c, g, tab)
			}
			w.areaDrop(c, area, g)
		})
	})
}

func (w *Workspace) content(c *ui.Context, g *layout.Group, tab *layout.Tab) {
	w.drawingTab = tab.Path
	defer func() { w.drawingTab = "" }()
	if v, ok := w.virtual[tab.Path]; ok {
		if w.focusTab == tab && v.focus != nil {
			w.focusTab = nil
			v.focus()
		}
		v.view(c)
		return
	}
	if isVirtual(tab.Path) {
		placeholder(c, "This view is closed", "Open it again from the View menu.")
		return
	}
	e := w.ensureEditor(tab)
	switch {
	case e.loading:
		w.applyDiffBase(tab)
		if wait := token.SpinnerDelay - time.Since(e.since); wait > 0 {
			c.After(wait)
			return
		}
		detail.Loading(c, loadingLabel)
	case e.err != nil:
		w.openFailed(c, tab.Path, e.err)
	case e.kind.ReplacesEditor():
		w.viewerContent(c, g, tab, e)
	case e.doc.Large != nil:
		if e.large == nil {
			e.large = &largefile.State{Doc: e.doc.Large}
		}
		largefile.View(c.Key(tab.ID), e.large).Grow(1).FillWidth()
		tab.Preview = tab.Preview && !e.doc.Modified()
		w.focusContent(c, g, tab, e.large.Handle())
	case tab.Editor == nil:
		placeholder(c, "Cannot show this file", unsupportedReason(e))
	default:
		if tab.Preview && e.doc.Modified() {
			tab.Preview = false
		}
		w.configure(tab.Editor, tab.Path)
		w.decorateCode(c, tab)
		w.applyDiffBase(tab)
		editor.View(c.Key(tab.ID), tab.Editor).Grow(1).FillWidth().Label(w.tabTitle(tab.Path)).Bind(&tab.Handle)
		w.focusContent(c, g, tab, &tab.Handle)
	}
}

func (w *Workspace) focusContent(c *ui.Context, g *layout.Group, tab *layout.Tab, h *ui.Handle) {
	if w.focusTab == tab {
		h.Focus(c)
		w.focusTab = nil
	}
	if h.FocusWithin(c) {
		w.active = g
		w.focusedTab = tab
	}
}

func (w *Workspace) openFailed(c *ui.Context, path string, err error) {
	o := feedback.EmptyOptions{Title: "Cannot open " + filepath.Base(path), Message: errtext.Text(err), Action: tryAgainLabel,
		OnAction: func() { delete(w.docs, path) }}
	if !errors.Is(err, fs.ErrNotExist) {
		o.Link, o.OnLink = revealInFinderLabel, func() { w.revealInFinder(path) }
	}
	detail.Centered(c, func() { feedback.Empty(c, o) })
}

func placeholder(c *ui.Context, title, message string) {
	detail.Centered(c, func() { feedback.Empty(c, feedback.EmptyOptions{Title: title, Message: message}) })
}
