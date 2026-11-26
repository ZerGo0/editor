package filetree

import (
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	RowHeight     = 22
	iconSize      = 16
	chevronSize   = 12
	indentWidth   = 12
	arrowWidth    = 16
	iconGap       = 6
	rowInset      = 6
	editHeight    = 20
	editFontSize  = 13
	cutOpacity    = 0.5
	emptyPadding  = 16
	emptyGap      = 8
	emptyFontSize = 12
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
	Notify func(text string)
}

func (t *Tree) Focus() {
	t.focus = true
}

func (t *Tree) View(c *ui.Context, vibrant bool, h Handlers) ui.Element {
	th := c.Theme()
	focused := t.state.List.Handle.FocusWithin(c)
	local := *th
	if !focused {
		local.Accent, local.InactiveAccent, local.AccentText = th.SurfacePressed, th.SurfacePressed, th.Text
	}
	box := ui.Themed(c, &local).Grow(1).FillWidth().MinHeight(0).Background(ui.Transparent)
	hovered := box.Hovered()
	submitted := false
	box.Children(func() {
		var cur ui.OutlineRow
		t.state.Prefix = func(r ui.OutlineRow) { cur = r }
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
		roots := t.rows(t.Root)
		t.slotBuilt = false
		outline := ui.Outline(c, &t.state, roots, t.rows, func(p string) { t.item(c, p, cur, h, focused, hovered) }).
			Grow(1).FillWidth().Padding(0, rowInset).Label("Files")
		if vibrant {
			outline.Background(ui.Transparent)
		}
		outline.ContextMenu(func(m *ui.Menu) { h.Menu(m, t.Root) })
		if paths, ok := ui.Drop[Paths](outline); ok {
			h.Move(paths, t.Root)
		}
		outline.Children(func() {
			if len(roots) == 0 {
				t.empty(c)
			}
		})
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
	return filepath.Base(p)
}

func (t *Tree) empty(c *ui.Context) {
	th := c.Theme()
	ui.Column(c).Padding(emptyPadding).Gap(emptyGap).FillWidth().Children(func() {
		ui.Text(c, "This folder is empty.").FontSize(emptyFontSize).TextColor(th.TextMuted)
		if ui.Button(c, "New File").Label("Create a New File").Clicked() {
			t.StartCreate(t.Root, false)
		}
	})
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
	t.editInput(c, h).Absolute().Left(t.slot.X - b.X).Top(t.slot.Y - b.Y).Width(t.slot.W)
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

func (t *Tree) item(c *ui.Context, p string, r ui.OutlineRow, h Handlers, focused, hovered bool) {
	th := c.Theme()
	dir := t.isDir[p]
	onAccent := focused && p == t.Selected()
	row := ui.Row(c).Height(RowHeight).FillWidth().AlignItems(ui.Center)
	row.Children(func() {
		guides(c, r.Depth, hovered)
		arrow := r.Arrow().Size(arrowWidth, RowHeight).Center()
		if r.Branch {
			arrow.Children(func() {
				svg := icons.ChevronRight
				if r.Open {
					svg = icons.ChevronDown
				}
				icon := ui.Icon(c, svg).Size(chevronSize, chevronSize)
				if !onAccent {
					icon.TextColor(th.TextMuted)
				}
			})
		}
		icons.FileIcon(c, filepath.Base(p), dir || t.isPlaceholder(p) && t.edit.folder, r.Open, iconSize)
		ui.Box(c).Width(iconGap).Shrink(0)
		if e := t.edit; e != nil && (t.isPlaceholder(p) || !e.create && e.path == p) {
			slot := ui.Box(c).Grow(1).Height(editHeight)
			t.slot, t.slotBuilt = slot.LayoutBounds(), true
			return
		}
		name := ui.Text(c, filepath.Base(p)).SingleLine().Grow(1).MinWidth(0)
		if !onAccent && t.IsIgnored(p) {
			name.TextColor(th.TextMuted)
		}
	})
	if t.isCut(p) {
		row.Opacity(cutOpacity)
	}
	if t.isPlaceholder(p) {
		return
	}
	switch {
	case row.RowDoubleClicked() && !dir:
		h.Open(p, false, true)
	case row.RowClicked():
		t.activate(p, true, h)
	}
	row.ContextMenu(func(m *ui.Menu) { h.Menu(m, p) })
	if !dir {
		return
	}
	if _, over := ui.DragOver[Paths](row); over {
		row.Background(th.Selection).Radius(th.Radius)
	}
	if paths, ok := ui.Drop[Paths](row); ok {
		h.Move(paths, p)
	}
}

func guides(c *ui.Context, depth int, visible bool) {
	if depth == 0 {
		return
	}
	box := ui.Box(c).Width(float32(depth) * indentWidth).Height(RowHeight).Shrink(0)
	if !visible {
		return
	}
	color := c.Theme().Border
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		for i := range depth {
			x := r.X + float32(i)*indentWidth + arrowWidth/2
			p.Fill(ui.Rect{X: x, Y: r.Y, W: 1, H: r.H}, color, 0)
		}
	})
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
	input := ui.TextInput(c, &e.value).AutoFocus().Height(editHeight).FontSize(editFontSize).
		Padding(0, 4).Label(label).Placeholder(label).Invalid(e.err != "")
	if e.err != "" {
		input.Tooltip(e.err)
	}
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
