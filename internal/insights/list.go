package insights

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	noGroup      = -1
	filterNoHits = "No results match the filter."
	clearFilter  = "Clear Filter"
)

type Entry struct {
	Icon      *ui.SVG
	Tone      token.Tone
	Title     string
	Detail    string
	Line, Col int
}

type Group struct {
	Path    string
	Entries []Entry
}

type rowKey struct {
	path  string
	entry int
}

type List struct {
	Filter    string
	EntryMenu func(m *ui.Menu, path string, e Entry)
	state     ui.OutlineState[rowKey]
	row       int
	key       uint64
	filtered  string
	built     bool
	all       []Group
	groups    []Group
	byPath    map[string]int
	roots     []rowKey
	seen      map[string]bool
}

func (l *List) CollapseAll() {
	l.state.Open.Clear()
}

func (l *List) build(key uint64, groups func() []Group) {
	if !l.built || key != l.key {
		l.all, l.key, l.built = groups(), key, true
		l.filtered = "\x00"
	}
	if l.filtered == l.Filter {
		return
	}
	l.filtered = l.Filter
	l.groups = filterGroups(l.all, l.Filter)
	l.byPath = map[string]int{}
	l.roots = l.roots[:0]
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	for i, g := range l.groups {
		l.byPath[g.Path] = i
		k := rowKey{path: g.Path, entry: noGroup}
		l.roots = append(l.roots, k)
		if !l.seen[g.Path] {
			l.seen[g.Path] = true
			l.state.Open.Add(k)
		}
	}
	l.state.Generation++
}

func filterGroups(groups []Group, filter string) []Group {
	q := strings.ToLower(strings.TrimSpace(filter))
	if q == "" {
		return groups
	}
	var out []Group
	for _, g := range groups {
		pathMatch := strings.Contains(strings.ToLower(g.Path), q)
		kept := Group{Path: g.Path}
		for _, e := range g.Entries {
			if pathMatch || strings.Contains(strings.ToLower(e.Title), q) || strings.Contains(strings.ToLower(e.Detail), q) {
				kept.Entries = append(kept.Entries, e)
			}
		}
		if len(kept.Entries) > 0 {
			out = append(out, kept)
		}
	}
	return out
}

func (l *List) children(k rowKey) []rowKey {
	if k.entry != noGroup {
		return nil
	}
	g := l.groups[l.byPath[k.path]]
	out := make([]rowKey, len(g.Entries))
	for i := range g.Entries {
		out[i] = rowKey{path: k.path, entry: i}
	}
	return out
}

func (l *List) entry(k rowKey) (Group, Entry, bool) {
	i, ok := l.byPath[k.path]
	if !ok {
		return Group{}, Entry{}, false
	}
	g := l.groups[i]
	if k.entry < 0 || k.entry >= len(g.Entries) {
		return g, Entry{}, false
	}
	return g, g.Entries[k.entry], true
}

func (l *List) View(c *ui.Context, label, empty string, key uint64, groups func() []Group, rel func(string) string, open func(path string, line, col int)) {
	l.build(key, groups)
	if len(l.groups) == 0 {
		l.empty(c, empty)
		return
	}
	if l.state.List.Selected == nil {
		l.row = -1
		l.state.List.Selected = &l.row
	}
	l.state.List.Label = func(i int) string {
		k := l.state.Item(i)
		if k.entry == noGroup {
			return rel(k.path)
		}
		_, e, _ := l.entry(k)
		return e.Title
	}
	submitted := list.Tree(c, &l.state, l.roots, l.children, list.ViewOptions{Label: label}, func(k rowKey) list.Item {
		if k.entry == noGroup {
			return l.groupItem(c, k, rel, open)
		}
		return l.entryItem(c, k, rel, open)
	}).Submitted()
	l.shortcuts(c, rel, open, submitted)
}

func (l *List) empty(c *ui.Context, empty string) {
	if l.Filter == "" || len(l.all) == 0 {
		feedback.Empty(c, feedback.EmptyOptions{Message: empty})
		return
	}
	feedback.Empty(c, feedback.EmptyOptions{Message: filterNoHits, Link: clearFilter, OnLink: func() { l.Filter = "" }})
}

func (l *List) shortcuts(c *ui.Context, rel func(string) string, open func(path string, line, col int), submitted bool) {
	if l.row < 0 || l.row >= l.state.Rows() {
		return
	}
	k := l.state.Item(l.row)
	list := &l.state.List
	switch {
	case submitted, list.Shortcut(c, 0, ui.KeySpace):
		l.activate(k, open)
	case list.Shortcut(c, ui.Cmd, ui.KeyC):
		l.copy(c, k, rel, false)
	}
}

func (l *List) activate(k rowKey, open func(path string, line, col int)) {
	if k.entry == noGroup {
		if l.state.Open.Has(k) {
			l.state.Open.Remove(k)
		} else {
			l.state.Open.Add(k)
		}
		return
	}
	if _, e, ok := l.entry(k); ok {
		open(k.path, e.Line, e.Col)
	}
}

func (l *List) copy(c *ui.Context, k rowKey, rel func(string) string, message bool) {
	if k.entry == noGroup {
		c.WriteClipboard(rel(k.path))
		return
	}
	_, e, ok := l.entry(k)
	if !ok {
		return
	}
	text := e.Title
	if !message {
		text = fmt.Sprintf("%s:%d:%d: %s", rel(k.path), e.Line+1, e.Col+1, e.Title)
		if e.Detail != "" {
			text += " (" + e.Detail + ")"
		}
	}
	c.WriteClipboard(text)
}

func (l *List) groupItem(c *ui.Context, k rowKey, rel func(string) string, open func(path string, line, col int)) list.Item {
	g := l.groups[l.byPath[k.path]]
	name := filepath.Base(g.Path)
	dir := filepath.Dir(rel(g.Path))
	if dir == "." {
		dir = ""
	}
	return list.Item{
		Label:       name,
		Description: dir,
		Leading:     func(c *ui.Context) { icons.FileIcon(c, name, false, false, token.IconSize.Px()) },
		Count:       len(g.Entries),
		OnClick:     func() { l.activate(k, open) },
		Menu: func(m *ui.Menu) {
			if m.Item("Open File").Chosen() {
				open(g.Path, -1, 0)
			}
			if m.Item("Copy Path").Chosen() {
				l.copy(c, k, rel, false)
			}
			m.Separator()
			if m.Item("Collapse All").Chosen() {
				l.CollapseAll()
			}
		},
	}
}

func (l *List) entryItem(c *ui.Context, k rowKey, rel func(string) string, open func(path string, line, col int)) list.Item {
	_, e, ok := l.entry(k)
	if !ok {
		return list.Item{}
	}
	detail := fmt.Sprintf("[Ln %d, Col %d]", e.Line+1, e.Col+1)
	if e.Detail != "" {
		detail = e.Detail + "  " + detail
	}
	it := list.Item{
		Label:       e.Title,
		Description: detail,
		OnClick:     func() { open(k.path, e.Line, e.Col) },
		Menu: func(m *ui.Menu) {
			if m.Item("Copy").Shortcut(ui.Cmd, ui.KeyC).Chosen() {
				l.copy(c, k, rel, false)
			}
			if m.Item("Copy Message").Chosen() {
				l.copy(c, k, rel, true)
			}
			if l.EntryMenu != nil {
				l.EntryMenu(m, k.path, e)
			}
		},
	}
	if e.Icon != nil {
		tone := e.Tone
		it.Leading = func(c *ui.Context) {
			ui.Icon(c, e.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(token.Of(c).Tone(tone)).Shrink(0).AccessHidden(true)
		}
	}
	return it
}
