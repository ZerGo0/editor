package workspace

import (
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/problems"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/egoist/mygo/ui"
)

const (
	breadcrumbHeight  = 22
	crumbHeight       = 18
	crumbIconSize     = 14
	crumbChevronSize  = 10
	crumbPadding      = 4
	crumbGap          = 4
	breadcrumbPadding = 10
	breadcrumbLabel   = "Breadcrumbs"
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
	t := c.Theme()
	inside := fsutil.Within(w.Root, tab.Path)
	segments := []string{filepath.Base(tab.Path)}
	if inside {
		segments = strings.Split(w.rel(tab.Path), string(filepath.Separator))
	}
	ui.Row(c).Height(breadcrumbHeight).Shrink(0).FillWidth().AlignItems(ui.Center).Padding(0, breadcrumbPadding).
		Background(t.Background).Role(ui.RoleNavigation).Label(breadcrumbLabel).Clip().Children(func() {
		for i, seg := range segments {
			last := i == len(segments)-1
			if i > 0 {
				crumbChevron(c)
			}
			p := filepath.Join(append([]string{w.Root}, segments[:i+1]...)...)
			if !inside {
				p = tab.Path
			}
			w.crumb(c, "crumb-"+seg, seg, func() { icons.FileIcon(c, seg, !last, false, crumbIconSize) }, func() {
				w.activate(g, tab)
				w.revealPath(p, !last)
			})
		}
		if sym := w.symbolAt(tab); sym != "" {
			crumbChevron(c)
			w.crumb(c, "crumb-symbol", sym, nil, func() {
				w.activate(g, tab)
				w.GoToSymbolInEditor()
			})
		}
	})
}

func crumbChevron(c *ui.Context) {
	ui.Icon(c, icons.ChevronRight).Size(crumbChevronSize, crumbChevronSize).TextColor(c.Theme().TextMuted).Shrink(0).AccessHidden(true)
}

func (w *Workspace) crumb(c *ui.Context, key, text string, icon func(), action func()) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Height(crumbHeight).Padding(0, crumbPadding).Gap(crumbGap).AlignItems(ui.Center).
		Radius(iconButtonRadius).Label(text).MinWidth(0).Shrink(1)
	color := t.TextMuted
	if pressable(b, t) {
		color = t.Text
	}
	b.Children(func() {
		if icon != nil {
			icon()
		}
		ui.Text(c, text).FontSize(smallFontSize).TextColor(color).SingleLine().MinWidth(0)
	})
	b.OnClick(action)
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
		go func() {
			syms := symbols.Scan(lang, text)
			if syms == nil {
				syms = []symbols.Symbol{}
			}
			w.update(func() { e.running, e.version, e.syms = false, version, syms })
		}()
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
