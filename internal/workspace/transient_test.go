package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/ZerGo0/editor/internal/workspace/notices"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	whereEditor   = "editor"
	whereSide     = "side"
	wherePanel    = "panel"
	whereNone     = "none"
	typedNoise    = "zq"
	secondFile    = "b.txt"
	keyedTitle    = "Search Everywhere"
	cornerMargin  = 5
	panelNoticeID = "hello"
)

type focusOrigin struct {
	name  string
	where string
	setup func(b *goToLineBed)
}

var focusOrigins = []focusOrigin{
	{"editor", whereEditor, func(b *goToLineBed) { b.w.focusTab = b.tab }},
	{"explorer", whereSide, func(b *goToLineBed) { b.w.tree.Focus() }},
	{"panel", wherePanel, func(b *goToLineBed) {
		b.w.ShowPanel(FindPanel)
		b.w.focusPart(partPanel)
	}},
	{"no editor", whereNone, func(b *goToLineBed) { b.w.CloseAllEditors() }},
}

type transientSurface struct {
	name    string
	command string
	open    func(w *Workspace)
	isOpen  func(w *Workspace) bool
	modal   bool
}

func searchTabOpen(tab finder.Tab) func(w *Workspace) bool {
	return func(w *Workspace) bool { return w.search.dialog.IsOpen() && w.search.dialog.Tab() == tab }
}

var transientSurfaces = []transientSurface{
	{name: "command palette", command: "workbench.action.showCommands", open: func(w *Workspace) { w.ShowCommands() }, isOpen: searchTabOpen(finder.TabActions)},
	{name: "quick open", command: "workbench.action.quickOpen", open: func(w *Workspace) { w.QuickOpen() }, isOpen: searchTabOpen(finder.TabFiles)},
	{name: "search everywhere", open: func(w *Workspace) { w.OpenSearch(finder.TabAll) }, isOpen: searchTabOpen(finder.TabAll)},
	{name: "find in files", open: func(w *Workspace) { w.FindInFiles() }, isOpen: searchTabOpen(finder.TabText)},
	{name: "recent files", command: "workbench.action.showRecentFiles", open: func(w *Workspace) { w.ShowRecentFiles() }, isOpen: func(w *Workspace) bool { return w.search.recentFiles.IsOpen() }},
	{name: "go to line", command: gotoLineID, open: func(w *Workspace) { w.GoToLine() }, isOpen: func(w *Workspace) bool { return w.palette.IsOpen() }},
	{name: "open recent", open: func(w *Workspace) { w.ShowOpenRecent() }, isOpen: func(w *Workspace) bool { return w.palette.IsOpen() }},
	{name: "switcher", open: func(w *Workspace) { w.showSwitcher() }, isOpen: func(w *Workspace) bool { return w.palette.IsOpen() }},
	{name: "prompt", modal: true, open: func(w *Workspace) { w.ask("Rename", "Rename", "x", func(string) error { return nil }) }, isOpen: func(w *Workspace) bool { return w.prompt.open }},
}

func newTransientBed(t *testing.T) *goToLineBed {
	t.Helper()
	env := newTestEnv(t)
	bed := &goToLineBed{}
	for _, s := range transientSurfaces {
		if s.command != "" {
			env.Commands.Add(&command.Command{ID: s.command, Title: s.name, Run: func(*mygo.Window) { s.open(bed.w) }})
		}
	}
	root := t.TempDir()
	var text strings.Builder
	for i := 1; i <= goToLineLines; i++ {
		fmt.Fprintf(&text, "line %d\n", i)
	}
	testutil.WriteFiles(t, root, map[string]string{goToLineFile: text.String(), secondFile: "b\n"})
	bed.w = newTestWorkspace(t, root, env, session.Project{})
	bed.tt = ui.NewTester(bed.w.View, testWideW, testViewH)
	bed.tt.Frame()
	bed.tt.Frame()
	bed.tab, _ = openTab(t, bed.w, filepath.Join(root, goToLineFile))
	bed.w.focusTab = bed.tab
	bed.settle()
	return bed
}

func (b *goToLineBed) startFrom(o focusOrigin) {
	o.setup(b)
	b.settle()
}

func (b *goToLineBed) where() string {
	switch {
	case b.w.focusedTab != nil:
		return whereEditor
	case b.w.focus.side:
		return whereSide
	case b.w.focus.panel:
		return wherePanel
	}
	return whereNone
}

func (b *goToLineBed) focusInDialog() bool {
	var focused, dialogs []ui.Rect
	for _, a := range b.tt.AccessAll() {
		if a.Focused {
			focused = append(focused, a.Bounds)
		}
		if a.Role == ui.RoleDialog {
			dialogs = append(dialogs, a.Bounds)
		}
	}
	for _, f := range focused {
		for _, d := range dialogs {
			if f.X >= d.X && f.Y >= d.Y && f.X+f.W <= d.X+d.W+1 && f.Y+f.H <= d.Y+d.H+1 {
				return true
			}
		}
	}
	return false
}

func (b *goToLineBed) pressCommand(id string) {
	for _, chord := range b.w.env.Keys.Current.For(id) {
		if len(chord) == 1 {
			b.press(chord[0].Mods, chord[0].Key)
			return
		}
	}
	panic("no single-stroke binding for " + id)
}

func (b *goToLineBed) openSurfaces() int {
	n := 0
	for _, open := range []bool{b.w.palette.IsOpen(), b.w.search.dialog.IsOpen(), b.w.search.recentFiles.IsOpen(), b.w.prompt.open} {
		if open {
			n++
		}
	}
	return n
}

func TestTransientSurfacesTakeFocusAndLeaveItWhereItWas(t *testing.T) {
	for _, s := range transientSurfaces {
		for _, o := range focusOrigins {
			t.Run(s.name+"/"+o.name, func(t *testing.T) {
				b := newTransientBed(t)
				b.startFrom(o)
				before := b.bufferText()
				s.open(b.w)
				b.tt.Frame()
				if !s.isOpen(b.w) {
					t.Fatal("did not open")
				}
				if !b.focusInDialog() {
					t.Error("the keyboard focus is not in the surface in the frame it opened")
				}
				b.type_(typedNoise)
				if b.bufferText() != before {
					t.Error("typing reached the document under the surface")
				}
				b.press(0, ui.KeyEscape)
				if b.openSurfaces() != 0 {
					t.Error("Escape left the surface open")
				}
				if got := b.where(); got != o.where {
					t.Errorf("after Escape focus is in %s, want %s", got, o.where)
				}
			})
		}
	}
}

func TestTransientSurfacesCloseWhenTheyLoseTheUser(t *testing.T) {
	for _, s := range transientSurfaces {
		t.Run(s.name, func(t *testing.T) {
			b := newTransientBed(t)
			s.open(b.w)
			b.settle()
			b.tt.SetWindowActive(false)
			b.settle()
			if s.isOpen(b.w) != s.modal {
				t.Errorf("after the window was deactivated open = %v, want %v", s.isOpen(b.w), s.modal)
			}
			b.tt.SetWindowActive(true)
			b.settle()
			b.tt.ClickAt(cornerMargin, float32(testViewH)-cornerMargin)
			b.settle()
			if s.isOpen(b.w) && !s.modal {
				t.Error("a click outside left the surface open")
			}
		})
	}
}

func TestShortcutsOpenAnotherSurfaceWhileOneIsOpen(t *testing.T) {
	var keyed []transientSurface
	for _, s := range transientSurfaces {
		if s.command != "" {
			keyed = append(keyed, s)
		}
	}
	for _, first := range keyed {
		for _, second := range keyed {
			if first.name == second.name {
				continue
			}
			for _, o := range focusOrigins[:3] {
				t.Run(first.name+" then "+second.name+"/"+o.name, func(t *testing.T) {
					b := newTransientBed(t)
					b.startFrom(o)
					b.pressCommand(first.command)
					if !first.isOpen(b.w) {
						t.Fatalf("%s did not open", first.name)
					}
					b.pressCommand(second.command)
					if !second.isOpen(b.w) || b.openSurfaces() != 1 {
						t.Errorf("%s did not replace %s: %d surfaces open", second.name, first.name, b.openSurfaces())
					}
					if !b.focusInDialog() {
						t.Error("focus is not in the second surface")
					}
					b.press(0, ui.KeyEscape)
					if got := b.where(); got != o.where || b.openSurfaces() != 0 {
						t.Errorf("after Escape focus is in %s with %d surfaces open, want %s and none", got, b.openSurfaces(), o.where)
					}
				})
			}
		}
	}
}

func TestEnterInAPickerFocusesTheChosenEditor(t *testing.T) {
	for _, o := range focusOrigins[:3] {
		t.Run(o.name, func(t *testing.T) {
			b := newTransientBed(t)
			openTab(t, b.w, filepath.Join(b.w.Root, secondFile))
			b.startFrom(o)
			b.w.showSwitcher()
			b.settle()
			b.press(0, ui.KeyDown)
			b.press(0, ui.KeyEnter)
			if b.openSurfaces() != 0 {
				t.Fatal("Enter left the switcher open")
			}
			if b.w.active.ActiveTab() != b.tab {
				t.Fatal("Enter did not choose the second editor")
			}
			if got := b.where(); got != whereEditor {
				t.Fatalf("focus is in %s after choosing an editor, want the editor", got)
			}
		})
	}
}

func TestOpeningFromTheCommandCenterReturnsFocusToThePreviousPart(t *testing.T) {
	for _, o := range focusOrigins[:3] {
		t.Run(o.name, func(t *testing.T) {
			b := newTransientBed(t)
			action.SetCommands(searchCommands{run: func() { b.w.OpenSearch(finder.TabAll) }})
			t.Cleanup(func() { action.SetCommands(nil) })
			b.startFrom(o)
			if err := b.tt.Click(keyedTitle); err != nil {
				t.Fatal(err)
			}
			b.w.runPendingUpdates()
			b.settle()
			if !b.w.search.dialog.IsOpen() || !b.focusInDialog() {
				t.Fatal("the click did not open Search Everywhere with the focus in it")
			}
			b.press(0, ui.KeyEscape)
			if got := b.where(); got != o.where {
				t.Errorf("after Escape focus is in %s, want %s", got, o.where)
			}
		})
	}
}

func TestNotificationCenterTakesFocusAndLeavesItWhereItWas(t *testing.T) {
	for _, o := range focusOrigins {
		t.Run(o.name, func(t *testing.T) {
			b := newTransientBed(t)
			b.startFrom(o)
			before := b.bufferText()
			b.w.notify(notices.Notice{Title: panelNoticeID})
			b.settle()
			b.w.ToggleNotifications()
			b.tt.Frame()
			if !b.focusInDialog() {
				t.Error("the notification center did not take the focus")
			}
			b.type_(typedNoise)
			if b.bufferText() != before {
				t.Error("typing reached the document under the notification center")
			}
			b.tt.SetWindowActive(false)
			b.settle()
			if b.w.noteCenter {
				t.Error("the notification center stayed open in an inactive window")
			}
			b.tt.SetWindowActive(true)
			b.w.ToggleNotifications()
			b.settle()
			b.press(0, ui.KeyEscape)
			if b.w.noteCenter {
				t.Error("Escape left the notification center open")
			}
			if got := b.where(); got != o.where {
				t.Errorf("after Escape focus is in %s, want %s", got, o.where)
			}
		})
	}
}

func TestFindWidgetKeepsItsFocusUntilEscape(t *testing.T) {
	b := newTransientBed(t)
	b.tab.Editor.GoTo(editor.Pos{Line: 5, Col: 2})
	b.settle()
	before := b.bufferText()
	b.press(ui.Cmd, ui.KeyF)
	in, ok := b.tt.AccessByRole(ui.RoleTextField, "Find")
	if !ok || !in.Focused {
		t.Fatal("Cmd+F did not focus the find field")
	}
	b.type_("line 7")
	if b.bufferText() != before {
		t.Error("typing reached the document")
	}
	b.tt.SetWindowActive(false)
	b.settle()
	if _, ok := b.tt.AccessByRole(ui.RoleTextField, "Find"); !ok {
		t.Error("deactivating the window closed the find widget")
	}
	b.tt.SetWindowActive(true)
	b.press(0, ui.KeyEscape)
	if _, ok := b.tt.AccessByRole(ui.RoleTextField, "Find"); ok {
		t.Error("Escape left the find widget open")
	}
	if got := b.where(); got != whereEditor {
		t.Errorf("after Escape focus is in %s, want the editor", got)
	}
}
