package workspace

import (
	"time"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const zoomStep = 0.1

var (
	lineNumberModes = map[string]editor.LineNumbers{
		settings.NumbersOn: editor.LineNumbersOn, settings.NumbersRelative: editor.LineNumbersRelative, settings.NumbersOff: editor.LineNumbersOff,
	}
	whitespaceModes = map[string]editor.Whitespace{
		settings.WhitespaceNone: editor.WhitespaceNone, settings.WhitespaceSelection: editor.WhitespaceSelection,
		settings.WhitespaceTrailing: editor.WhitespaceTrailing, settings.WhitespaceAll: editor.WhitespaceAll,
	}
	cursorShapes = map[string]editor.CursorShape{
		settings.CursorLine: editor.CursorBar, settings.CursorBlock: editor.CursorBlock, settings.CursorUnderline: editor.CursorUnderline,
	}
)

func (w *Workspace) zoom() float32 {
	return 1 + float32(w.settings.Int(settings.ZoomLevel))*zoomStep
}

func (w *Workspace) configure(st *editor.State) {
	s := w.settings
	st.Font = ui.Font{Family: cmpOr(s.String(settings.FontFamily), token.FontEditor), Size: float32(s.Int(settings.FontSize))}
	st.HoverDelay = token.EditorHover
	st.LineHeight = float32(s.Number(settings.LineHeight))
	st.TabSize, st.Indent = 0, editor.IndentAuto
	if !s.Bool(settings.DetectIndentation) {
		st.TabSize, st.Indent = s.Int(settings.TabSize), editor.IndentTabs
		if s.Bool(settings.InsertSpaces) {
			st.Indent = editor.IndentSpaces
		}
	}
	st.SoftWrap = s.String(settings.WordWrap) == settings.WrapOn
	st.LineNumbers = lineNumberModes[s.String(settings.LineNumbers)]
	st.Whitespace = whitespaceModes[s.String(settings.RenderWhitespace)]
	st.WrapGuides = s.Ints(settings.Rulers)
	st.NoIndentGuides = !s.Bool(settings.IndentGuides)
	st.NoScrollPastEnd = !s.Bool(settings.ScrollBeyondLastLine)
	st.NoAutoClose = !s.Bool(settings.AutoClosingBrackets)
	st.NoFolding = !s.Bool(settings.Folding)
	st.NoActiveLine = !s.Bool(settings.LineHighlight)
	st.CursorShape = cursorShapes[s.String(settings.CursorStyle)]
	st.Bindings = w.env.Keys.Current.EditorBindings()
	st.OnInput = w.editorInput
	st.ContextMenu = w.editorMenu
	st.RenderFindBar = findwidget.ForEditor(st)
}

func (w *Workspace) editorMenu(m *ui.Menu) {
	m.Separator()
	w.commandItem(m, "Go to Symbol in Editor...", "workbench.action.gotoSymbol")
	w.commandItem(m, "Go to Line/Column...", gotoLineID)
	w.gitEditorItems(m)
	w.githubEditorItems(m)
	m.Separator()
	w.commandItem(m, "Command Palette...", "workbench.action.showCommands")
}

func (w *Workspace) prepareSave(d *document.Document) {
	trim, finalNewline, lineEnding := w.saveRules(d)
	if trim {
		if st := w.editorFor(d.Path); st != nil {
			st.Run(editor.TrimTrailingWhitespace)
		}
	}
	if finalNewline {
		d.EnsureFinalNewline()
	}
	if lineEnding != "" && d.Buffer.LineEnding() != lineEnding {
		d.Buffer.SetLineEnding(lineEnding)
	}
}

func (w *Workspace) editorFor(path string) *editor.State {
	if t := w.active.ActiveTab(); t != nil && t.Path == path && t.Editor != nil {
		return t.Editor
	}
	for _, t := range w.tabs() {
		if t.Path == path && t.Editor != nil {
			return t.Editor
		}
	}
	return nil
}

func (w *Workspace) autoSave(focused *layout.Tab) {
	mode := w.settings.String(settings.AutoSave)
	if mode == settings.AutoSaveFocusChange && focused != w.lastFocused {
		if t := w.lastFocused; t != nil {
			if d := w.doc(t.Path); d != nil && d.Modified() {
				w.save([]*document.Document{d})
			}
		}
	}
	w.lastFocused = focused
	if mode != settings.AutoSaveAfterDelay {
		return
	}
	delay := time.Duration(w.settings.Int(settings.AutoSaveDelay)) * time.Millisecond
	for _, e := range w.docs {
		d := e.doc
		if d == nil || !d.Modified() {
			continue
		}
		version := d.Buffer.Version()
		if e.autoSaved == version {
			continue
		}
		e.autoSaved = version
		time.AfterFunc(delay, func() {
			w.update(func() {
				if d.Modified() && d.Buffer.Version() == version {
					w.save([]*document.Document{d})
				}
			})
		})
	}
}
