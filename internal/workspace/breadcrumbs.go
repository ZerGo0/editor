package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/problems"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
)

const (
	breadcrumbLabel = "Breadcrumbs"
	maxCrumbEntries = 200
)

type symbolEntry struct {
	version uint64
	running bool
	syms    []symbols.Symbol
}

func (w *Workspace) breadcrumbs(c *ui.Context, g *layout.Group, tab *layout.Tab) {
	if !onDisk(tab.Path) {
		return
	}
	segments, dirs := w.crumbPath(tab.Path)
	crumbs := make([]chrome.Crumb, 0, len(segments)+1)
	for i, seg := range segments {
		last := i == len(segments)-1
		p := dirs[i]
		cr := chrome.Crumb{Label: seg, Leading: func(c *ui.Context) { icons.FileIcon(c, seg, !last, false, token.IconSize.Px()) }}
		if last {
			cr.Menu = func(m *ui.Menu) { w.fileCrumbMenu(m, g, tab) }
		} else {
			cr.Menu = func(m *ui.Menu) { w.folderCrumbMenu(m, p) }
		}
		crumbs = append(crumbs, cr)
	}
	if sym := w.symbolAt(tab); sym != "" {
		crumbs = append(crumbs, chrome.Crumb{Label: sym, Icon: icons.ListView, Menu: func(m *ui.Menu) { w.symbolCrumbMenu(m, g, tab) }})
	}
	ui.Row(c).FillWidth().Shrink(0).Background(token.Of(c).Layer.Editor).Clip().Children(func() {
		chrome.Breadcrumbs(c, breadcrumbLabel, crumbs)
	})
}

func (w *Workspace) crumbPath(file string) (names, paths []string) {
	anchor := string(filepath.Separator)
	home, err := os.UserHomeDir()
	switch {
	case fsutil.Within(w.Root, file):
		anchor = w.Root
	case err == nil && fsutil.Within(home, file):
		anchor = home
		names, paths = []string{fsutil.HomePrefix}, []string{home}
	}
	rel, _ := filepath.Rel(anchor, file)
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		names = append(names, part)
		paths = append(paths, filepath.Join(anchor, filepath.Join(parts[:i+1]...)))
	}
	return names, paths
}

func (w *Workspace) folderCrumbMenu(m *ui.Menu, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		m.Item(err.Error()).Disabled(true)
		return
	}
	for _, e := range entries[:min(len(entries), maxCrumbEntries)] {
		p := filepath.Join(dir, e.Name())
		if w.excluded(p) {
			continue
		}
		if m.Item(e.Name()).Chosen() {
			if e.IsDir() {
				w.revealPath(p, true)
			} else {
				w.OpenFile(p, false, true)
			}
		}
	}
}

func (w *Workspace) fileCrumbMenu(m *ui.Menu, g *layout.Group, tab *layout.Tab) {
	if m.Item("Reveal in Explorer View").Disabled(!fsutil.Within(w.Root, tab.Path)).Chosen() {
		w.activate(g, tab)
		w.revealPath(tab.Path, false)
	}
	if m.Item("Reveal in Finder").Chosen() {
		w.revealInFinder(tab.Path)
	}
	m.Separator()
	w.pathItems(m, tab.Path)
}

func (w *Workspace) symbolCrumbMenu(m *ui.Menu, g *layout.Group, tab *layout.Tab) {
	e := w.symbols[tab.Path]
	if e == nil {
		return
	}
	for _, s := range e.syms[:min(len(e.syms), maxCrumbEntries)] {
		if m.Item(s.Name).Chosen() {
			w.activate(g, tab)
			w.goToLocation(g, session.Location{Path: tab.Path, Line: s.Line, Col: s.Col})
		}
	}
}

func (w *Workspace) revealPath(p string, dir bool) {
	if !fsutil.Within(w.Root, p) {
		w.revealInFinder(p)
		return
	}
	w.ShowSideView(ExplorerView)
	w.tools.explorerCollapsed = false
	if dir {
		w.tree.Expand(p)
	} else {
		w.tree.Reveal(p)
	}
	w.tree.Focus()
}

func (w *Workspace) symbolAt(tab *layout.Tab) string {
	st := tab.Editor
	if st == nil || st.Buffer == nil || st.Buffer.Language() == nil {
		return ""
	}
	buf := st.Buffer
	lang := buf.Language().Name
	if !symbols.Supports(lang) || buf.Len() > problems.MaxTextSize {
		return ""
	}
	e := w.symbols[tab.Path]
	if e == nil {
		e = &symbolEntry{}
		w.symbols[tab.Path] = e
	}
	if version := buf.Version(); !e.running && (e.syms == nil || e.version != version) {
		e.running = true
		text := buf.String()
		applog.Go(func() {
			syms := symbols.Scan(lang, text)
			if syms == nil {
				syms = []symbols.Symbol{}
			}
			w.update(func() { e.running, e.version, e.syms = false, version, syms })
		})
	}
	line := st.CursorPos().Line
	name := ""
	for _, s := range e.syms {
		if s.Line > line {
			break
		}
		name = s.Name
	}
	return name
}
