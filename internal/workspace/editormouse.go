package workspace

import (
	"sort"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const fastScrollSensitivity = 5

type gutterDrag struct {
	st   *editor.State
	line int
}

func (w *Workspace) editorPointer(st *editor.State, ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputScroll:
		if ev.Mods == ui.Alt && ev.DY != 0 {
			st.SetScrollY(st.ScrollY() + float64(ev.DY)*(fastScrollSensitivity-1))
		}
	case ui.InputPointerMove:
		if d := w.gutter; d != nil && d.st == st {
			d.extend(ev.Y)
			return true
		}
	case ui.InputPointerUp:
		w.gutter = nil
	}
	return false
}

func (d *gutterDrag) extend(y float32) {
	buf := d.st.Buffer
	line := lineAtY(d.st, y)
	start, end := lineOffset(buf, min(d.line, line)), lineOffset(buf, max(d.line, line)+1)
	if line < d.line {
		start, end = end, start
	}
	d.st.Select(start, end)
}

func lineAtY(st *editor.State, y float32) int {
	top := float64(y) + st.ScrollY()
	lines := st.Buffer.LineCount()
	line := sort.Search(lines, func(i int) bool { return st.LineTop(i+1) > top })
	return min(line, lines-1)
}

func lineOffset(buf *editor.Buffer, line int) int {
	if line >= buf.LineCount() {
		return buf.Len()
	}
	return buf.Offset(editor.Pos{Line: line})
}
