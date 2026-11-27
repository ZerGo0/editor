package workspace

import (
	"slices"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/gotoline"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/egoist/mygo/ui/editor"
)

const (
	goToLineLabel = "Go to Line/Column"
	halfViewport  = 2
)

type goToLine struct {
	tab         *layout.Tab
	scrollY     float64
	backgrounds []editor.LineBackground
	query       string
}

func (w *Workspace) GoToLine() {
	w.closeSearchPopups()
	tab := w.active.ActiveTab()
	if tab == nil || tab.Editor == nil {
		w.palette.Show(palette.Mode{
			Label:       goToLineLabel,
			Placeholder: gotoline.NoEditor,
			Search:      func(_ string, done func([]palette.Item)) { done([]palette.Item{{Label: gotoline.NoEditor}}) },
		})
		return
	}
	g := &goToLine{tab: tab, scrollY: tab.Editor.ScrollY(), backgrounds: tab.Editor.LineBackgrounds}
	w.goTo = g
	w.palette.Show(palette.Mode{
		Label:       goToLineLabel,
		Placeholder: g.label(gotoline.Target{}),
		Search:      func(query string, done func([]palette.Item)) { done([]palette.Item{g.item(query)}) },
		Accept:      func(it palette.Item) { w.finishGoToLine(g, it.Value.(gotoline.Target)) },
		Dismiss:     func() { w.cancelGoToLine(g) },
	})
}

func (g *goToLine) buffer() *editor.Buffer {
	return g.tab.Editor.Buffer
}

func (g *goToLine) resolve(query string) gotoline.Target {
	buf := g.buffer()
	return gotoline.Parse(query, buf.LineCount(), func(line int) int { return utf8.RuneCountInString(buf.Line(line)) })
}

func (g *goToLine) label(t gotoline.Target) string {
	buf := g.buffer()
	pos := g.tab.Editor.CursorPos()
	line := buf.Line(pos.Line)
	column := utf8.RuneCountInString(line[:min(pos.Col, len(line))]) + 1
	return t.Label(pos.Line+1, column, buf.LineCount())
}

func (g *goToLine) item(query string) palette.Item {
	t := g.resolve(query)
	return palette.Item{Label: g.label(t), Value: t, Disabled: !t.LineOK}
}

func (w *Workspace) syncGoToLine() {
	g := w.goTo
	if g == nil || w.palette.Query() == g.query {
		return
	}
	g.query = w.palette.Query()
	st := g.tab.Editor
	t := g.resolve(g.query)
	if !t.LineOK {
		st.LineBackgrounds = g.backgrounds
		return
	}
	line := t.Line - 1
	st.LineBackgrounds = append(slices.Clone(g.backgrounds), editor.LineBackground{First: line, Last: line, Color: w.codeTheme.Occurrence})
	top := st.LineTop(line)
	height := st.LineTop(line+1) - top
	st.SetScrollY(top - (st.ViewportHeight()-height)/halfViewport)
}

func (w *Workspace) finishGoToLine(g *goToLine, t gotoline.Target) {
	w.endGoToLine(g)
	column := 0
	if t.ColumnOK {
		column = gotoline.ByteColumn(g.buffer().Line(t.Line-1), t.Column)
	}
	g.tab.Editor.GoTo(editor.Pos{Line: t.Line - 1, Col: column})
}

func (w *Workspace) cancelGoToLine(g *goToLine) {
	w.endGoToLine(g)
	g.tab.Editor.SetScrollY(g.scrollY)
}

func (w *Workspace) endGoToLine(g *goToLine) {
	if w.goTo == g {
		w.goTo = nil
	}
	g.tab.Editor.LineBackgrounds = g.backgrounds
	w.focusTab = g.tab
}
