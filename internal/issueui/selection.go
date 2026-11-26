package issueui

import (
	"github.com/egoist/mygo/ui/editor"
)

func selectionLines(buf *editor.Buffer, sel editor.Selection) (start, end int) {
	from, to := buf.Pos(sel.Start()), buf.Pos(sel.End())
	if !sel.Empty() && to.Col == 0 && to.Line > from.Line {
		to.Line--
	}
	return from.Line, to.Line
}

func (i *Issues) withActiveSelection(fn func(path string, startLine, endLine int)) {
	path, st := i.h.ActivePath(), i.h.ActiveEditor()
	if path == "" || st == nil || st.Buffer == nil {
		i.h.Notify("Open a file in the editor first.")
		return
	}
	start, end := selectionLines(st.Buffer, st.Selection())
	fn(path, start, end)
}

func (i *Issues) createFromActiveSelection() {
	i.withActiveSelection(func(path string, startLine, endLine int) {
		i.CreateFromSelection(path, startLine, endLine, i.h.ActiveEditor().SelectedText())
	})
}
