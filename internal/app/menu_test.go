package app

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
)

func newMenuApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	a := New(Options{NoMenuAccelerators: true})
	a.settings = settings.Load(filepath.Join(dir, settingsFile))
	a.keys = keymap.Load(filepath.Join(dir, keybindingsFile))
	a.commands = command.NewRegistry()
	a.registerCommands()
	return a
}

func findItem(items []*mygo.MenuItem, label string) *mygo.MenuItem {
	for _, it := range items {
		if it.Label == label {
			return it
		}
	}
	return nil
}

func TestViewMenuWordWrapShowsTheFocusedFileState(t *testing.T) {
	a := newMenuApp(t)
	item := findItem(a.viewMenu().Submenu, "Word Wrap")
	if item == nil || item.Type != mygo.MenuItemCheckbox {
		t.Fatalf("View has no Word Wrap checkbox: %+v", item)
	}
	if item.Checked {
		t.Fatal("Word Wrap starts checked with no workspace")
	}
	a.applyViewState(workspace.ViewState{WordWrap: true})
	if !item.Checked {
		t.Fatal("Word Wrap is not checked after the focused file wraps")
	}
	a.applyViewState(workspace.ViewState{})
	if item.Checked {
		t.Fatal("Word Wrap stays checked after the focused file stops wrapping")
	}
}
