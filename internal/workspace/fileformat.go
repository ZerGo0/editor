package workspace

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	defaultTabSize    = 4
	plainTextName     = "Plain Text"
	languagePickTitle = "Select Language Mode"
	lineFeed          = "\n"
	carriageReturnLF  = "\r\n"
)

var tabSizes = []int{2, 4, 8}

type indentOverride struct {
	tabs bool
	size int
}

type languagePick struct {
	lang *editor.Language
}

func (w *Workspace) applyOverrides(tab *layout.Tab) {
	o, ok := w.indents[tab.Path]
	if !ok || tab.Editor == nil {
		return
	}
	tab.Editor.TabSize, tab.Editor.Indent = o.size, editor.IndentSpaces
	if o.tabs {
		tab.Editor.Indent = editor.IndentTabs
	}
}

func effectiveIndent(st *editor.State) indentOverride {
	lang := st.Buffer.Language()
	size := st.TabSize
	if size <= 0 && lang != nil {
		size = lang.TabSize
	}
	if size <= 0 {
		size = defaultTabSize
	}
	tabs := st.Indent == editor.IndentTabs || st.Indent == editor.IndentAuto && lang != nil && lang.HardTabs
	return indentOverride{tabs: tabs, size: size}
}

func indentLabel(o indentOverride) string {
	if o.tabs {
		return "Tab Size: " + strconv.Itoa(o.size)
	}
	return "Spaces: " + strconv.Itoa(o.size)
}

func (w *Workspace) indentMenu(m *ui.Menu, tab *layout.Tab) {
	cur := effectiveIndent(tab.Editor)
	set := func(o indentOverride) {
		w.indents[tab.Path] = o
		w.focusTab = tab
	}
	if m.Item("Indent Using Spaces").Radio(!cur.tabs).Chosen() {
		set(indentOverride{tabs: false, size: cur.size})
	}
	if m.Item("Indent Using Tabs").Radio(cur.tabs).Chosen() {
		set(indentOverride{tabs: true, size: cur.size})
	}
	m.Separator()
	for _, n := range tabSizes {
		if m.Item("Tab Size: " + strconv.Itoa(n)).Radio(cur.size == n).Chosen() {
			set(indentOverride{tabs: cur.tabs, size: n})
		}
	}
	if _, ok := w.indents[tab.Path]; ok {
		m.Separator()
		if m.Item("Reset to Settings").Chosen() {
			delete(w.indents, tab.Path)
			w.focusTab = tab
		}
	}
}

func (w *Workspace) lineEndingMenu(m *ui.Menu, tab *layout.Tab) {
	buf := tab.Editor.Buffer
	cur := buf.LineEnding()
	for _, end := range []string{lineFeed, carriageReturnLF} {
		if m.Item(lineEndingNames[end]).Radio(cur == end).Disabled(tab.Editor.ReadOnly).Chosen() && cur != end {
			buf.SetLineEnding(end)
			w.focusTab = tab
		}
	}
}

func (w *Workspace) ChangeLanguage() {
	t := w.active.ActiveTab()
	if t == nil || t.Editor == nil {
		return
	}
	cur := t.Editor.Buffer.Language()
	items := []palette.Item{{Label: plainTextName, Value: languagePick{}}}
	for _, l := range editor.Languages() {
		item := palette.Item{Label: l.Name, Value: languagePick{lang: l}}
		if l == cur {
			item.Detail = "Current"
		}
		items = append(items, item)
	}
	w.palette.Show(languagePickTitle, palette.Tab{
		Placeholder: "Select a language mode",
		Search:      palette.Static(items),
		Hint:        "Enter to select",
	})
}

func (w *Workspace) setLanguage(pick languagePick) {
	t := w.active.ActiveTab()
	if t == nil || t.Editor == nil {
		return
	}
	t.Editor.Buffer.SetLanguage(pick.lang)
	if e := w.insights.problems[t.Path]; e != nil {
		e.items = nil
	}
	w.focusTab = t
}
