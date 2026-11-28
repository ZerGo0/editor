package workspace

import (
	"fmt"
	"strconv"

	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/plural"
	"github.com/ZerGo0/editor/internal/problems"
	"github.com/egoist/mygo/ui"
)

const (
	largeFileText = "Large file mode"
	readOnlyText  = "Read-only"
	lockedTooltip = "You do not have permission to change this file"
	largeFileTip  = "This file is large, so it opens in a lighter editor without syntax highlighting."
	plainFileText = "Large file: no highlighting"
	plainFileTip  = "This file is large, so syntax highlighting is off."
	encodingTip   = "File Encoding"
	gotoLineID    = "workbench.action.gotoLine"
	problemsID    = "workbench.actions.view.problems"
	todoID        = "todo.show"

	statusProblems   = "Problems"
	statusTODO       = "TODO"
	statusPosition   = "Cursor Position"
	statusIndent     = "Indentation"
	statusEncoding   = "Encoding"
	statusLineEnding = "End of Line"
	statusLanguage   = "Language"
)

var (
	lineEndingNames = map[string]string{"\n": "LF", "\r\n": "CRLF", "\r": "CR"}
	statusItems     = []string{statusProblems, statusTODO, statusPosition, statusIndent, statusEncoding, statusLineEnding, statusLanguage}
)

func unsupportedReason(e *docEntry) string {
	switch e.doc.Kind {
	case document.Binary:
		return "This file is binary."
	}
	return ""
}

func (w *Workspace) statusBar(c *ui.Context) {
	p := token.Of(c)
	bar := ui.Row(c).Height(token.StatusBarHeight.Px()+token.BorderWidth.Px()).Padding(0, token.PadInputX.Px()).Shrink(0).FillWidth().AlignItems(ui.Center).
		Background(p.Layer.StatusBar).BorderWidth(token.BorderWidth.Px(), 0, 0, 0).BorderColor(p.Border.Default).
		Role(ui.RoleToolbar).Label("Status Bar")
	bar.ContextMenu(w.statusMenu)
	bar.Children(func() {
		ui.ScrollHorizontal(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().Children(func() {
			ui.Row(c).FillHeight().AlignItems(ui.Center).Children(func() {
				for _, fn := range w.statusLeft {
					fn(c)
				}
				if !w.statusHidden[statusProblems] {
					w.problemsStatus(c)
				}
				if !w.statusHidden[statusTODO] {
					n := w.todoCount()
					chrome.StatusParts(c, plural.Count(n, "TODO comment", "TODO comments"), []chrome.StatusPart{{Icon: icons.Todo, Text: strconv.Itoa(n)}},
						chrome.StatusOptions{ID: "todo", Command: todoID, Tooltip: "Toggle TODO", OnClick: func() { w.togglePanel(TODOPanel) }})
				}
				if hint := w.chordHint(); hint != "" {
					chrome.StatusItem(c, hint, chrome.StatusOptions{ID: "chord", Tooltip: hint})
				}
			})
		})
		ui.ScrollHorizontal(c).Shrink(1).MinWidth(0).FillHeight().Children(func() {
			ui.Row(c).FillHeight().AlignItems(ui.Center).Children(func() {
				w.activeStatus(c)
				w.notificationsStatus(c)
			})
		})
	})
}

func (w *Workspace) activeStatus(c *ui.Context) {
	tab := w.active.ActiveTab()
	if d := w.docOf(tab); d != nil && d.Large != nil {
		chrome.StatusItem(c, largeFileText, chrome.StatusOptions{ID: "large", Tooltip: largeFileTip})
		return
	}
	if tab != nil && tab.Editor != nil {
		w.editorStatus(c, tab)
	}
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
	counts := w.problemCounts()
	parts := []chrome.StatusPart{
		{Icon: icons.Error, Text: strconv.Itoa(counts[problems.Error]), Tone: token.ToneError},
		{Icon: icons.Warning, Text: strconv.Itoa(counts[problems.Warning]), Tone: token.ToneWarning},
	}
	if n := counts[problems.Info]; n > 0 {
		parts = append(parts, chrome.StatusPart{Icon: icons.Info, Text: strconv.Itoa(n), Tone: token.ToneInfo})
	}
	label := "Problems: " + plural.Count(counts[problems.Error], "error", "errors") + ", " + plural.Count(counts[problems.Warning], "warning", "warnings")
	chrome.StatusParts(c, label, parts, chrome.StatusOptions{ID: "problems", Command: problemsID, OnClick: w.toggleProblems,
		Tooltip: "Toggle Problems"})
}

func (w *Workspace) toggleProblems() {
	w.togglePanel(ProblemsPanel)
}

func (w *Workspace) togglePanel(id string) {
	if w.tools.panelOpen && w.tools.panel == id {
		w.TogglePanel()
		return
	}
	w.ShowPanel(id)
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
		chrome.StatusItem(c, text, chrome.StatusOptions{ID: "position", Command: gotoLineID, Tooltip: "Go to Line/Column"})
	}
	if !w.statusHidden[statusIndent] {
		indent := indentLabel(effectiveIndent(st))
		chrome.StatusItem(c, indent, chrome.StatusOptions{ID: "indent", Label: statusIndent + ": " + indent, Tooltip: "Select Indentation", Menu: func(m *ui.Menu) { w.indentMenu(m, tab) }})
	}
	d := w.doc(tab.Path)
	if d != nil && d.Locked {
		chrome.StatusItem(c, readOnlyText, chrome.StatusOptions{ID: "read-only", Icon: icons.Lock, Tooltip: lockedTooltip})
	}
	if !w.statusHidden[statusEncoding] {
		text := "UTF-8"
		if d != nil && d.NotUTF8 {
			text = "Read-only (not UTF-8)"
		}
		chrome.StatusItem(c, text, chrome.StatusOptions{ID: "encoding", Tooltip: encodingTip})
	}
	if d != nil && d.Kind == document.Plain {
		chrome.StatusItem(c, plainFileText, chrome.StatusOptions{ID: "plain", Tooltip: plainFileTip})
	}
	if !w.statusHidden[statusLineEnding] {
		eol := lineEndingNames[buf.LineEnding()]
		chrome.StatusItem(c, eol, chrome.StatusOptions{ID: "eol", Label: statusLineEnding + ": " + eol, Tooltip: "Select End of Line Sequence", Menu: func(m *ui.Menu) { w.lineEndingMenu(m, tab) }})
	}
	if !w.statusHidden[statusLanguage] {
		lang := plainTextName
		if l := buf.Language(); l != nil {
			lang = l.Name
		}
		chrome.StatusItem(c, lang, chrome.StatusOptions{ID: "language", Label: statusLanguage + ": " + lang, Tooltip: "Select Language Mode", OnClick: w.ChangeLanguage})
	}
}
