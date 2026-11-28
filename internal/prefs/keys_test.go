package prefs

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	testCommandID    = "test.command"
	testCommandTitle = "Test Command"
	testViewWidth    = 900
	testViewHeight   = 500
)

func newKeysBed(t *testing.T) (*KeysView, *ui.Tester, *bool) {
	t.Helper()
	reg := command.NewRegistry()
	reg.Add(&command.Command{ID: testCommandID, Title: testCommandTitle, Run: func(*mygo.Window) {}})
	v := &KeysView{Keys: keymap.Load(filepath.Join(t.TempDir(), "keybindings.json")), Commands: reg, OpenJSON: func() {}}
	focusList := new(bool)
	tt := ui.NewTester(func(c *ui.Context) {
		v.View(c)
		if *focusList {
			v.list.Focus(c)
			*focusList = false
		}
	}, testViewWidth, testViewHeight)
	tt.Frame()
	return v, tt, focusList
}

func TestKeybindingsKeepTheCommandColumnWhenNarrow(t *testing.T) {
	t.Parallel()
	reg := command.NewRegistry()
	reg.Add(&command.Command{ID: testCommandID, Title: testCommandTitle, Run: func(*mygo.Window) {}})
	v := &KeysView{Keys: keymap.Load(filepath.Join(t.TempDir(), "keybindings.json")), Commands: reg, OpenJSON: func() {}}
	tt := ui.NewTester(v.View, narrowViewWidth, testViewHeight)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Command") || !tt.HasText("Keybinding") {
		t.Fatalf("header = %v", tt.Texts())
	}
	if tt.HasText("When") {
		t.Fatalf("the When column stays in a narrow view: %v", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, keysJSONOpen); !ok {
		t.Fatal("no JSON button")
	}
	if got := v.columns(); got.when {
		t.Fatalf("columns = %+v", got)
	}
}

func focusedRole(tt *ui.Tester) (ui.Role, bool) {
	for _, a := range tt.AccessAll() {
		if a.Focused {
			return a.Role, true
		}
	}
	return ui.RoleNone, false
}

func TestKeybindingRecorderTakesTheFocusAndReturnsItOnEscape(t *testing.T) {
	v, tt, focusList := newKeysBed(t)
	*focusList = true
	tt.Frame()
	before, ok := focusedRole(tt)
	if !ok {
		t.Fatal("the list took no focus")
	}
	v.record(keyRow{cmd: v.Commands.Get(testCommandID)})
	tt.Frame()
	dialog, ok := tt.AccessByRole(ui.RoleDialog, "Press the keys for "+testCommandTitle)
	if !ok {
		t.Fatal("no recorder dialog")
	}
	if got, _ := focusedRole(tt); got == before {
		t.Fatalf("focus stayed on role %v; the recorder did not take it (dialog %v)", got, dialog.Bounds)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if v.rec.open {
		t.Fatal("Escape left the recorder open")
	}
	if got, _ := focusedRole(tt); got != before {
		t.Fatalf("focus is on role %v after Escape, want %v", got, before)
	}
}

func TestKeybindingRecorderStaysOpenWhenTheWindowDeactivates(t *testing.T) {
	v, tt, _ := newKeysBed(t)
	v.record(keyRow{cmd: v.Commands.Get(testCommandID)})
	tt.Frame()
	tt.SetWindowActive(false)
	tt.Frame()
	if !v.rec.open {
		t.Fatal("the modal recorder closed on blur")
	}
}
