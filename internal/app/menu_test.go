package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/testutil"
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

func TestFileMenuOpensFilesBeforeFolders(t *testing.T) {
	a := newMenuApp(t)
	a.store, _ = session.Load(filepath.Join(t.TempDir(), sessionFile))
	var labels []string
	for _, it := range a.fileMenu().Submenu {
		labels = append(labels, it.Label)
	}
	file, folder := slices.Index(labels, "Open File..."), slices.Index(labels, "Open Folder...")
	if file < 0 || folder != file+1 {
		t.Fatalf("File menu = %v, want Open File... right before Open Folder...", labels)
	}
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

func TestResizeCommandsUseVSCodeIDs(t *testing.T) {
	a := newMenuApp(t)
	for _, id := range []string{
		"workbench.action.evenEditorWidths", "workbench.action.toggleEditorWidths",
		"workbench.action.toggleMaximizeEditorGroup", "workbench.action.toggleMaximizedPanel",
		"workbench.action.increaseViewSize", "workbench.action.decreaseViewSize",
		"workbench.action.increaseViewWidth", "workbench.action.decreaseViewWidth",
		"workbench.action.increaseViewHeight", "workbench.action.decreaseViewHeight",
	} {
		if a.commands.Get(id) == nil {
			t.Errorf("no command %s", id)
		}
	}
}

func labels(items []*mygo.MenuItem) []string {
	return testutil.Map(items, func(it *mygo.MenuItem) string { return it.Label })
}

func TestHelpMenuListsCommandsShortcutsAndLogs(t *testing.T) {
	a := newMenuApp(t)
	help := a.helpMenu()
	if help.Role != mygo.RoleHelp {
		t.Fatalf("Help menu role = %v", help.Role)
	}
	want := []string{"Show All Commands", "Keyboard Shortcuts", "", "Open Logs Folder"}
	if got := labels(help.Submenu); !slices.Equal(got, want) {
		t.Fatalf("Help menu = %v, want %v", got, want)
	}
}

func TestEditAndSelectionMenusHoldTheEditorCommands(t *testing.T) {
	a := newMenuApp(t)
	edit := labels(a.editMenu().Submenu)
	for _, want := range []string{"Find", "Replace", "Find Next", "Find Previous", "Use Selection for Find", "Find in Files", "Replace in Files", "Toggle Line Comment", "Toggle Block Comment"} {
		if !slices.Contains(edit, want) {
			t.Errorf("Edit menu has no %q: %v", want, edit)
		}
	}
	selection := labels(a.selectionMenu().Submenu)
	for _, want := range []string{"Expand Selection", "Shrink Selection", "Copy Line Up", "Move Line Down", "Add Cursor Above", "Add Next Occurrence", "Select All Occurrences"} {
		if !slices.Contains(selection, want) {
			t.Errorf("Selection menu has no %q: %v", want, selection)
		}
	}
}

func TestMenuItemsFollowWhatTheFocusedWindowCanDo(t *testing.T) {
	a := newMenuApp(t)
	a.store, _ = session.Load(filepath.Join(t.TempDir(), sessionFile))
	file, edit, view, gitm := a.fileMenu().Submenu, a.editMenu().Submenu, a.viewMenu().Submenu, a.gitMenu().Submenu
	save, find, sidebar, commit := findItem(file, "Save"), findItem(edit, "Find"), findItem(view, "Primary Side Bar"), findItem(gitm, "Commit...")
	if save.IsEnabled() || find.IsEnabled() || sidebar.IsEnabled() || commit.IsEnabled() {
		t.Fatal("items that need a window stay enabled with no window")
	}
	if !findItem(file, "Open Folder...").IsEnabled() {
		t.Fatal("Open Folder... is disabled with no window")
	}
	a.applyViewState(workspace.ViewState{Project: true, Editor: true, Sidebar: true})
	if !save.IsEnabled() || find.IsEnabled() || !sidebar.IsChecked() || commit.IsEnabled() {
		t.Fatal("a non-text editor without Git does not enable Save only")
	}
	a.applyViewState(workspace.ViewState{Project: true, Editor: true, TextEditor: true, Git: true})
	if !find.IsEnabled() || !commit.IsEnabled() || sidebar.IsChecked() {
		t.Fatal("a text editor in a repository does not enable Find and Commit")
	}
}

func TestFullScreenItemSaysWhatItWillDo(t *testing.T) {
	a := newMenuApp(t)
	item := findItem(a.viewMenu().Submenu, "Enter Full Screen")
	if item == nil || item.Role != mygo.RoleToggleFullScreen {
		t.Fatalf("View has no Enter Full Screen item: %+v", item)
	}
	a.setFullScreen(true)
	if item.Label != "Exit Full Screen" {
		t.Fatalf("label in full screen = %q", item.Label)
	}
}
