package workspace

import (
	"math"
	"time"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const zoomFactor = 1.2

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
	return float32(math.Pow(zoomFactor, float64(w.settings.Int(settings.ZoomLevel))))
}

func (w *Workspace) configure(st *editor.State, path string) {
	w.ConfigureEditor(st, path)
	st.OnInput = func(ev ui.InputEvent) bool { return w.editorPointer(st, ev) || w.editorInput(ev) }
	st.OnDefinition = func(int) { w.goToDefinition(st) }
	st.ContextMenu = w.editorMenu
	st.RenderFindBar = findwidget.ForEditor(st)
}

func (w *Workspace) ConfigureEditor(st *editor.State, path string) {
	s := w.settings
	st.Font = w.editorFont()
	st.HoverDelay = token.EditorHover
	st.Bindings = w.env.Keys.Current.EditorBindings()
	st.LineHeight = float32(s.Number(settings.LineHeight))
	st.TabSize, st.Indent = 0, editor.IndentAuto
	if !s.Bool(settings.DetectIndentation) {
		st.TabSize, st.Indent = s.Int(settings.TabSize), editor.IndentTabs
		if s.Bool(settings.InsertSpaces) {
			st.Indent = editor.IndentSpaces
		}
	}
	w.applyWrapping(st, path)
	st.LineNumbers = lineNumberModes[s.String(settings.LineNumbers)]
	st.Whitespace = whitespaceModes[s.String(settings.RenderWhitespace)]
	st.WrapGuides = s.Ints(settings.Rulers)
	st.NoIndentGuides = !s.Bool(settings.IndentGuides)
	st.NoScrollPastEnd = !s.Bool(settings.ScrollBeyondLastLine)
	st.NoAutoClose = !s.Bool(settings.AutoClosingBrackets)
	st.NoFolding = !s.Bool(settings.Folding)
	st.NoActiveLine = !s.Bool(settings.LineHighlight)
	st.CursorShape = cursorShapes[s.String(settings.CursorStyle)]
	if w.codeTheme != nil {
		st.Theme = w.codeTheme
	}
	w.applyEditorConfig(st, path)
	w.applyOverrides(st, path)
}

// TECHDEBT: MyGo 0.10.0 DiffState has no bracket color, guide, hover delay or wrap column option; add them here when it does.
func (w *Workspace) ConfigureDiff(d *editor.DiffState, path string) {
	var st editor.State
	w.ConfigureEditor(&st, path)
	d.Font, d.LineHeight, d.TabSize = st.Font, st.LineHeight, st.TabSize
	d.Wrap, _ = w.wrapping(true, path)
	w.trackDiff(path)
	d.ScrollPastEnd = !st.NoScrollPastEnd
	d.Bindings = withoutHostCommands
	if st.Theme != nil {
		d.Theme = st.Theme
	}
}

var withoutHostCommands = hostCommandsOff()

func hostCommandsOff() []editor.Binding {
	var out []editor.Binding
	for _, b := range editor.DefaultBindings() {
		if b.Command == editor.GoToLine || b.Command == editor.ToggleSoftWrap {
			out = append(out, editor.Binding{Mods: b.Mods, Key: b.Key, ThenKey: b.ThenKey, Command: editor.NoCommand})
		}
	}
	return out
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
