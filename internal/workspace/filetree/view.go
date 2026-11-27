package filetree

import (
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	filesLabel   = "Files"
	emptyMessage = "This folder is empty."
	newFileLabel = "New File"
)

type Paths []string

type Handlers struct {
	Open   func(p string, preview, focus bool)
	Menu   func(m *ui.Menu, p string)
	Create func(dir, name string, folder bool) error
	Rename func(from, name string) error
	Trash  func(p string)
	Paste  func(paths []string, dir string, cut bool)
	Move   func(paths []string, dir string)
	Leave  func()
	Import func(paths []string, dir string)
	Notify func(text string)
	Marker func(p string) (count int, tone token.Tone)
}

func (t *Tree) Focus() {
	t.focus = true
}

func (t *Tree) View(c *ui.Context, h Handlers) ui.Element {
	box := ui.Column(c).Grow(1).FillWidth().MinHeight(0)
	submitted := false
	box.Children(func() {
		t.state.List.Label = func(i int) string { return t.label(t.state.Item(i)) }
		t.state.List.DragRows = func(rows []int) any {
			var out Paths
			for _, i := range rows {
				if p := t.state.Item(i); !t.isPlaceholder(p) {
					out = append(out, p)
				}
			}
			if len(out) == 0 {
				return nil
			}
			return out
		}
		roots := t.visibleRows(t.Root)
		t.slotBuilt = false
		outline := list.Tree(c, &t.state, roots, t.visibleRows, list.ViewOptions{Label: filesLabel, Empty: t.empty},
			func(p string) list.Item { return t.item(c, p, h) })
		outline.ContextMenu(func(m *ui.Menu) { h.Menu(m, t.Root) })
		if paths, ok := ui.Drop[Paths](outline); ok {
			dropPaths(c, h, paths, t.Root)
		}
		if files := outline.DroppedFiles(); len(files) > 0 && h.Import != nil {
			h.Import(files, t.Root)
		}
		submitted = outline.Submitted()
		t.placeEdit(c, box, h)
	})
	t.afterBuild(c, h, submitted)
	return box
}

func (t *Tree) label(p string) string {
	if t.isPlaceholder(p) {
		return t.editLabel()
	}
	return t.name(p)
}

func (t *Tree) empty(c *ui.Context) {
	feedback.Empty(c, feedback.EmptyOptions{Message: emptyMessage, Link: newFileLabel, OnLink: func() { t.StartCreate(t.Root, false, false) }})
}

func (t *Tree) placeEdit(c *ui.Context, box ui.Element, h Handlers) {
	if t.edit == nil || !t.slotBuilt {
		return
	}
	if t.slot.W == 0 {
		c.Invalidate()
		return
	}
	b := box.LayoutBounds()
	ui.Box(c).Absolute().Left(t.slot.X - b.X).Top(t.slot.Y - b.Y).Width(t.slot.W).Children(func() { t.editInput(c, h) })
}

func (t *Tree) afterBuild(c *ui.Context, h Handlers, submitted bool) {
	if t.focus {
		t.focus = false
		t.state.List.Focus(c)
	}
	if t.pending != "" {
		for i := range t.state.Rows() {
			if t.state.Item(i) == t.pending {
				t.row = i
				t.state.List.ScrollIntoView(i)
				break
			}
		}
		t.pending = ""
	}
	if t.edit != nil {
		return
	}
	t.shortcuts(c, h, submitted)
}

func (t *Tree) shortcuts(c *ui.Context, h Handlers, submitted bool) {
	l := &t.state.List
	p := t.Selected()
	switch {
	case p == "":
		if l.Shortcut(c, ui.Cmd, ui.KeyV) {
			t.paste(t.Root, h)
		}
	case submitted:
		t.activate(p, false, h)
	case l.Shortcut(c, 0, ui.KeySpace):
		t.activate(p, true, h)
	case l.Shortcut(c, 0, ui.KeyF2):
		t.StartRename(p)
	case l.Shortcut(c, ui.Cmd, ui.KeyBackspace):
		h.Trash(p)
	case l.Shortcut(c, ui.Cmd, ui.KeyC):
		t.SetClipboard([]string{p}, false)
	case l.Shortcut(c, ui.Cmd, ui.KeyX):
		t.SetClipboard([]string{p}, true)
	case l.Shortcut(c, ui.Cmd, ui.KeyV):
		t.paste(t.dirOf(p), h)
	case t.cut && l.Shortcut(c, 0, ui.KeyEscape):
		t.ClearClipboard()
	}
}

func (t *Tree) dirOf(p string) string {
	if t.isDir[p] {
		return p
	}
	return filepath.Dir(p)
}

func (t *Tree) paste(dir string, h Handlers) {
	if len(t.clip) == 0 {
		return
	}
	paths, cut := t.Clipboard()
	h.Paste(paths, dir, cut)
	if cut {
		t.ClearClipboard()
	}
}

func dropPaths(c *ui.Context, h Handlers, paths []string, dir string) {
	if c.Modifiers()&ui.Alt != 0 && h.Import != nil {
		h.Import(paths, dir)
		return
	}
	h.Move(paths, dir)
}

func (t *Tree) activate(p string, preview bool, h Handlers) {
	if t.isDir[p] {
		t.toggle(p)
		return
	}
	h.Open(p, preview, !preview)
}

func (t *Tree) toggle(p string) {
	if t.state.Open.Has(p) {
		t.state.Open.Remove(p)
		return
	}
	t.state.Open.Add(p)
}

func (t *Tree) item(c *ui.Context, p string, h Handlers) list.Item {
	dir := t.isDir[p]
	name := t.name(p)
	it := list.Item{
		Label: name,
		Leading: func(c *ui.Context) {
			icons.FileIcon(c, filepath.Base(p), dir || t.isPlaceholder(p) && t.edit.folder, t.state.Open.Has(p), token.IconSize.Px())
		},
		Muted: t.isCut(p) || t.IsIgnored(p),
		Git:   t.gitStatus(p),
	}
	if e := t.edit; e != nil && (t.isPlaceholder(p) || !e.create && e.path == p) {
		it.Content = func(c *ui.Context) {
			slot := ui.Box(c).Grow(1).FillHeight()
			t.slot, t.slotBuilt = slot.LayoutBounds(), true
		}
		return it
	}
	if h.Marker != nil && !dir {
		it.Count, it.Tone = h.Marker(p)
	}
	if it.Tone != token.ToneNeutral {
		it.Git = token.GitNone
	}
	it.Menu = func(m *ui.Menu) { h.Menu(m, p) }
	it.OnClick = func() { t.activate(p, true, h) }
	if !dir {
		it.OnDoubleClick = func() { h.Open(p, false, true) }
		return it
	}
	it.Drop = func(row ui.Element) bool {
		if paths, ok := ui.Drop[Paths](row); ok {
			dropPaths(c, h, paths, p)
		}
		if files := row.DroppedFiles(); len(files) > 0 && h.Import != nil {
			h.Import(files, p)
		}
		_, over := ui.DragOver[Paths](row)
		return over
	}
	return it
}

func (t *Tree) editLabel() string {
	switch e := t.edit; {
	case e == nil:
		return ""
	case e.create && e.folder:
		return "New Folder"
	case e.create:
		return "New File"
	}
	return "Rename"
}

func (t *Tree) editInput(c *ui.Context, h Handlers) ui.Element {
	e := t.edit
	label := t.editLabel()
	input := field.TextInput(c, label, &e.value, field.InputOptions{Placeholder: label, AutoFocus: true, Error: e.err})
	if !e.selected && input.Focused() {
		e.selected = true
		stem := strings.TrimSuffix(e.value, filepath.Ext(e.value))
		if stem == "" {
			stem = e.value
		}
		input.SetTextSelection(0, len([]rune(stem)))
	}
	switch {
	case input.Shortcut(0, ui.KeyEscape):
		t.CancelEdit()
		if e.returnFocus {
			t.focus = false
			h.Leave()
		}
	case input.Submitted():
		t.commit(h, true)
	case input.Focused():
		e.focused = true
	case e.focused:
		t.commit(h, false)
	}
	return input
}

func (t *Tree) commit(h Handlers, keep bool) {
	e := t.edit
	name := strings.TrimSpace(e.value)
	if name == "" || !e.create && name == filepath.Base(e.path) {
		t.CancelEdit()
		return
	}
	var err error
	if e.create {
		err = h.Create(e.path, name, e.folder)
	} else {
		err = h.Rename(e.path, name)
	}
	if err == nil {
		t.CancelEdit()
		return
	}
	h.Notify(err.Error())
	if !keep {
		t.CancelEdit()
		return
	}
	e.err = err.Error()
}
