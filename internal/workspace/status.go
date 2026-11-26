package workspace

import (
	"fmt"
	"strconv"

	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/problems"
	"github.com/egoist/mygo/ui"
)

const (
	statusBarHeight  = 22
	statusFontSize   = smallFontSize
	statusPadding    = 4
	statusItemPad    = 6
	statusItemGap    = 4
	statusIconSize   = 13
	statusItemHeight = 20
	statusRadius     = 4
	largeFileText    = "Large file mode"

	statusProblems   = "Problems"
	statusPosition   = "Cursor Position"
	statusIndent     = "Indentation"
	statusEncoding   = "Encoding"
	statusLineEnding = "End of Line"
	statusLanguage   = "Language"
)

var (
	lineEndingNames = map[string]string{"\n": "LF", "\r\n": "CRLF", "\r": "CR"}
	statusItems     = []string{statusProblems, statusPosition, statusIndent, statusEncoding, statusLineEnding, statusLanguage}
)

func unsupportedReason(e *docEntry) string {
	switch e.doc.Kind {
	case document.Binary:
		return "This file is binary."
	}
	return ""
}

type statusSpec struct {
	key, text, tooltip string
	action             func()
	menu               func(m *ui.Menu)
}

func (w *Workspace) statusBar(c *ui.Context, vibrant bool) {
	t := c.Theme()
	bar := ui.Row(c).Height(statusBarHeight).Gap(statusItemGap).Padding(0, statusPadding).Shrink(0).FillWidth().AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
	if !vibrant {
		bar.Background(t.Surface)
	}
	bar.ContextMenu(w.statusMenu)
	bar.Children(func() {
		for _, fn := range w.statusLeft {
			fn(c)
		}
		if !w.statusHidden[statusProblems] {
			w.problemsStatus(c)
		}
		if hint := w.chordHint(); hint != "" {
			statusItem(c, statusSpec{key: "notice", text: hint})
		}
		ui.Box(c).Grow(1)
		tab := w.active.ActiveTab()
		if d := w.docOf(tab); d != nil && d.Large != nil {
			statusItem(c, statusSpec{key: "large", text: largeFileText})
			return
		}
		if tab == nil || tab.Editor == nil {
			return
		}
		w.editorStatus(c, tab)
	})
}

func (w *Workspace) statusMenu(m *ui.Menu) {
	for _, name := range statusItems {
		shown := !w.statusHidden[name]
		if m.Item(name).Checked(shown).Chosen() {
			w.statusHidden[name] = shown
		}
	}
}

func (w *Workspace) problemsStatus(c *ui.Context) {
	t := c.Theme()
	counts := w.problemCounts()
	b := ui.ButtonBase(c.Key("problems")).Height(statusItemHeight).Padding(0, statusItemPad).Gap(statusItemGap).AlignItems(ui.Center).
		Radius(statusRadius).Label(fmt.Sprintf("Problems: %d errors, %d warnings", counts[problems.Error], counts[problems.Warning])).
		Tooltip(w.withKeys("Toggle Problems", "workbench.actions.view.problems"))
	pressable(b, t)
	b.Children(func() {
		for _, s := range []struct {
			icon  *ui.SVG
			color ui.Color
			n     int
		}{{icons.Error, t.Danger, counts[problems.Error]}, {icons.Warning, t.Warning, counts[problems.Warning]}, {icons.Info, t.Accent, counts[problems.Info]}} {
			if s.n == 0 && s.color == t.Accent {
				continue
			}
			ui.Icon(c, s.icon).Size(statusIconSize, statusIconSize).TextColor(s.color).Shrink(0).AccessHidden(true)
			ui.Text(c, strconv.Itoa(s.n)).FontSize(statusFontSize).SingleLine()
		}
	})
	b.OnClick(w.toggleProblems)
}

func (w *Workspace) toggleProblems() {
	if w.tools.panelOpen && w.tools.panel == ProblemsPanel {
		w.TogglePanel()
		return
	}
	w.ShowPanel(ProblemsPanel)
}

func (w *Workspace) problemCounts() map[problems.Severity]int {
	out := map[problems.Severity]int{}
	for _, e := range w.insights.problems {
		for _, p := range e.items {
			out[p.Severity]++
		}
	}
	for _, g := range w.configProblems() {
		out[problems.Error] += len(g.Entries)
	}
	return out
}

func (w *Workspace) editorStatus(c *ui.Context, tab *layout.Tab) {
	st := tab.Editor
	buf := st.Buffer
	if !w.statusHidden[statusPosition] {
		pos := st.CursorPos()
		text := fmt.Sprintf("Ln %d, Col %d", pos.Line+1, pos.Col+1)
		if n := len(st.Selections()); n > 1 {
			text += fmt.Sprintf(" (%d selections)", n)
		}
		statusItem(c, statusSpec{key: "position", text: text, tooltip: w.withKeys("Go to Line/Column", "editor.goToLine"), action: func() { w.run("editor.goToLine") }})
	}
	if !w.statusHidden[statusIndent] {
		statusItem(c, statusSpec{key: "indent", text: indentLabel(effectiveIndent(st)), tooltip: "Select Indentation", menu: func(m *ui.Menu) { w.indentMenu(m, tab) }})
	}
	d := w.doc(tab.Path)
	if !w.statusHidden[statusEncoding] {
		text := "UTF-8"
		if d != nil && d.ReadOnly {
			text = "Read-only (not UTF-8)"
		}
		statusItem(c, statusSpec{key: "encoding", text: text})
	}
	if d != nil && d.Kind == document.Plain {
		statusItem(c, statusSpec{key: "plain", text: "Large file: no highlighting"})
	}
	if !w.statusHidden[statusLineEnding] {
		statusItem(c, statusSpec{key: "eol", text: lineEndingNames[buf.LineEnding()], tooltip: "Select End of Line Sequence", menu: func(m *ui.Menu) { w.lineEndingMenu(m, tab) }})
	}
	if !w.statusHidden[statusLanguage] {
		lang := plainTextName
		if l := buf.Language(); l != nil {
			lang = l.Name
		}
		statusItem(c, statusSpec{key: "language", text: lang, tooltip: "Select Language Mode", action: w.ChangeLanguage})
	}
}

func statusItem(c *ui.Context, s statusSpec) {
	t := c.Theme()
	if s.action == nil && s.menu == nil {
		ui.Text(c.Key(s.key), s.text).FontSize(statusFontSize).TextColor(t.TextMuted).SingleLine().Padding(0, statusItemPad).MinWidth(0)
		return
	}
	b := ui.ButtonBase(c.Key(s.key)).Height(statusItemHeight).Padding(0, statusItemPad).AlignItems(ui.Center).Radius(statusRadius).Label(s.text)
	if s.tooltip != "" {
		b.Tooltip(s.tooltip)
	}
	color := t.TextMuted
	if pressable(b, t) {
		color = t.Text
	}
	b.Children(func() {
		ui.Text(c, s.text).FontSize(statusFontSize).TextColor(color).SingleLine()
	})
	if s.menu != nil {
		b.Menu(s.menu)
		return
	}
	b.OnClick(s.action)
}
