package app

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
)

func TestWelcomeShowsStaleProjectsAndTakesTheKeyboard(t *testing.T) {
	a := newMenuApp(t)
	a.store, _ = session.Load(filepath.Join(t.TempDir(), sessionFile))
	gone := filepath.Join(t.TempDir(), "gone-project")
	a.store.State.Recent = []string{t.TempDir(), gone}
	a.checkRecent()
	a.home.notice = &unopened{message: "The folder gone-project no longer exists", recent: gone}
	focus := true
	tt := ui.NewTester(func(c *ui.Context) {
		a.welcomeView(c)
		if focus {
			focus = false
			a.home.list.Focus(c)
		}
	}, welcomeWidth, welcomeHeight)
	tt.Frame()
	tt.Frame()
	texts := tt.Texts()
	for _, want := range []string{missingFolder, "The folder gone-project no longer exists", removeRecent, "Open Folder...", "Open File...", newFileTitle + "..."} {
		if !tt.HasText(want) {
			t.Errorf("Welcome lacks %q: %v", want, texts)
		}
	}
	tt.KeyDown(ui.KeyDown)
	tt.Frame()
	if a.home.row != 1 {
		t.Fatalf("Down in Recent Projects chose row %d, want 1", a.home.row)
	}
}
