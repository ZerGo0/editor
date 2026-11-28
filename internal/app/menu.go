package app

import (
	"path/filepath"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/shellcmd"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
)

const (
	maxRecentMenuItems = 10
	goToLineCommand    = "workbench.action.gotoLine"
	wordWrapCommand    = "editor.action.toggleWordWrap"
	sidebarCommand     = "workbench.action.toggleSidebarVisibility"
	panelCommand       = "workbench.action.togglePanel"
	enterFullScreen    = "Enter Full Screen"
	exitFullScreen     = "Exit Full Screen"
)

type menuState struct {
	view       workspace.ViewState
	items      []stateItem
	fullScreen *mygo.MenuItem
	inFull     bool
}

type stateItem struct {
	item    *mygo.MenuItem
	enabled func(workspace.ViewState) bool
	checked func(workspace.ViewState) bool
}

func inProject(s workspace.ViewState) bool    { return s.Project }
func withEditor(s workspace.ViewState) bool   { return s.Editor }
func withText(s workspace.ViewState) bool     { return s.TextEditor }
func withFile(s workspace.ViewState) bool     { return s.OnDisk }
func withGit(s workspace.ViewState) bool      { return s.Git }
func withGitFile(s workspace.ViewState) bool  { return s.Git && s.OnDisk }
func canGoBack(s workspace.ViewState) bool    { return s.Back }
func canGoForward(s workspace.ViewState) bool { return s.Forward }

func (a *App) refreshMenu() {
	a.menu = menuState{view: a.viewState(), inFull: a.menu.inFull}
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		a.appMenu(),
		a.fileMenu(),
		a.editMenu(),
		a.selectionMenu(),
		a.viewMenu(),
		a.goMenu(),
		a.gitMenu(),
		{Role: mygo.RoleWindowMenu},
		a.helpMenu(),
	}))
}

func (a *App) item(id, label string) *mygo.MenuItem {
	cmd := a.commands.Get(id)
	if cmd == nil {
		panic("menu: unknown command " + id)
	}
	if label == "" {
		label = cmd.Title
	}
	item := &mygo.MenuItem{Label: label, Click: func(_ *mygo.MenuItem, win *mygo.Window) { a.runMenuCommand(id, win) }}
	if a.options.NoMenuAccelerators {
		return item
	}
	a.commands.SetInMenu(id)
	if s, ok := a.keys.Current.MenuStroke(id); ok {
		item.Accelerator = s.Accelerator()
	}
	return item
}

func (a *App) editorItem(cmd editor.Command, label string, enabled func(workspace.ViewState) bool) *mygo.MenuItem {
	id := keymap.EditorCommandID(cmd)
	if a.commands.Get(id) == nil {
		panic("menu: unknown command " + id)
	}
	item := &mygo.MenuItem{Label: label, Click: func(_ *mygo.MenuItem, win *mygo.Window) { a.runMenuCommand(id, win) }}
	if !a.options.NoMenuAccelerators {
		item.Badge = a.keys.Current.Label(id)
	}
	return a.when(item, enabled)
}

func (a *App) when(item *mygo.MenuItem, enabled func(workspace.ViewState) bool) *mygo.MenuItem {
	item.Disabled = !enabled(a.menu.view)
	a.menu.items = append(a.menu.items, stateItem{item: item, enabled: enabled})
	return item
}

func (a *App) checkItem(id, label string, enabled, checked func(workspace.ViewState) bool) *mygo.MenuItem {
	item := a.item(id, label)
	item.Type, item.Checked, item.Disabled = mygo.MenuItemCheckbox, checked(a.menu.view), !enabled(a.menu.view)
	run := item.Click
	item.Click = func(it *mygo.MenuItem, win *mygo.Window) {
		run(it, win)
		a.syncViewMenu()
	}
	a.menu.items = append(a.menu.items, stateItem{item: item, enabled: enabled, checked: checked})
	return item
}

func (a *App) viewState() workspace.ViewState {
	if pw := a.workspaceFor(nil); pw != nil {
		return pw.ws.ViewState()
	}
	return workspace.ViewState{}
}

func (a *App) syncViewMenu() {
	a.applyViewState(a.viewState())
}

func (a *App) applyViewState(s workspace.ViewState) {
	a.menu.view = s
	for _, it := range a.menu.items {
		if on := it.enabled(s); on != it.item.IsEnabled() {
			it.item.SetEnabled(on)
		}
		if it.checked != nil {
			if on := it.checked(s); on != it.item.IsChecked() {
				it.item.SetChecked(on)
			}
		}
	}
}

func (a *App) setFullScreen(on bool) {
	a.menu.inFull = on
	if a.menu.fullScreen != nil {
		a.menu.fullScreen.SetLabel(fullScreenLabel(on))
	}
}

func fullScreenLabel(on bool) string {
	if on {
		return exitFullScreen
	}
	return enterFullScreen
}

func (a *App) watchFullScreen(win *mygo.Window) {
	win.OnEnterFullScreen(func() { a.setFullScreen(true) })
	win.OnLeaveFullScreen(func() { a.setFullScreen(false) })
	win.OnFocus(func() { a.setFullScreen(win.IsFullScreen()) })
}

func (a *App) appMenu() *mygo.MenuItem {
	installed := shellcmd.Installed()
	install, uninstall := a.item("workbench.action.installCommandLine", ""), a.item("workbench.action.uninstallCommandLine", "")
	install.Hidden, uninstall.Hidden = installed, !installed
	return &mygo.MenuItem{Label: "Editor", Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleAbout},
		mygo.Separator(),
		a.when(a.item(settingsID, "Settings"), inProject),
		a.when(a.item(keybindingsID, "Keyboard Shortcuts"), inProject),
		a.item(importKeybindingsCommand, ""),
		mygo.Separator(),
		install,
		uninstall,
		mygo.Separator(),
		{Role: mygo.RoleHide},
		{Role: mygo.RoleHideOthers},
		{Role: mygo.RoleUnhide},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	}}
}

func (a *App) fileMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Label: "File", Submenu: []*mygo.MenuItem{
		a.when(a.item("workbench.action.files.newUntitledFile", ""), inProject),
		a.when(a.item("explorer.newFile", ""), inProject),
		a.when(a.item("explorer.newFolder", ""), inProject),
		a.item("workbench.action.newWindow", ""),
		mygo.Separator(),
		a.item(openFileID, ""),
		a.item(openFolderID, ""),
		{Label: "Open Recent", Submenu: a.recentItems()},
		mygo.Separator(),
		a.when(a.item("workbench.action.files.save", ""), withEditor),
		a.when(a.item("workbench.action.files.saveAs", ""), withEditor),
		a.when(a.item("workbench.action.files.saveAll", ""), inProject),
		a.when(a.item("workbench.action.files.revert", ""), withFile),
		mygo.Separator(),
		a.when(a.item("workbench.action.files.revealActiveFileInFinder", "Reveal in Finder"), withFile),
		mygo.Separator(),
		a.when(a.item("workbench.action.closeActiveEditor", ""), withEditor),
		a.item("workbench.action.closeWindow", ""),
	}}
}

func (a *App) editMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Role: mygo.RoleEditMenu, Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleUndo},
		{Role: mygo.RoleRedo},
		mygo.Separator(),
		{Role: mygo.RoleCut},
		{Role: mygo.RoleCopy},
		{Role: mygo.RolePaste},
		mygo.Separator(),
		a.editorItem(editor.Find, "Find", withText),
		a.editorItem(editor.FindReplace, "Replace", withText),
		a.editorItem(editor.FindNext, "Find Next", withText),
		a.editorItem(editor.FindPrevious, "Find Previous", withText),
		a.editorItem(editor.UseSelectionForFind, "Use Selection for Find", withText),
		mygo.Separator(),
		a.when(a.item("workbench.action.findInFiles", ""), inProject),
		a.when(a.item("workbench.action.replaceInFiles", ""), inProject),
		mygo.Separator(),
		a.editorItem(editor.ToggleComment, "Toggle Line Comment", withText),
		a.editorItem(editor.ToggleBlockComment, "Toggle Block Comment", withText),
	}}
}

func (a *App) selectionMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Label: "Selection", Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleSelectAll},
		a.editorItem(editor.SelectLarger, "Expand Selection", withText),
		a.editorItem(editor.SelectSmaller, "Shrink Selection", withText),
		mygo.Separator(),
		a.editorItem(editor.DuplicateLinesUp, "Copy Line Up", withText),
		a.editorItem(editor.DuplicateLinesDown, "Copy Line Down", withText),
		a.editorItem(editor.MoveLinesUp, "Move Line Up", withText),
		a.editorItem(editor.MoveLinesDown, "Move Line Down", withText),
		a.editorItem(editor.DuplicateSelection, "Duplicate Selection", withText),
		mygo.Separator(),
		a.editorItem(editor.AddCursorAbove, "Add Cursor Above", withText),
		a.editorItem(editor.AddCursorBelow, "Add Cursor Below", withText),
		a.editorItem(editor.SplitSelectionIntoLines, "Add Cursors to Line Ends", withText),
		a.editorItem(editor.SelectNext, "Add Next Occurrence", withText),
		a.editorItem(editor.SelectPrevious, "Add Previous Occurrence", withText),
		a.editorItem(editor.SelectAllMatches, "Select All Occurrences", withText),
	}}
}

func (a *App) viewMenu() *mygo.MenuItem {
	a.menu.fullScreen = &mygo.MenuItem{Role: mygo.RoleToggleFullScreen, Label: fullScreenLabel(a.menu.inFull)}
	return &mygo.MenuItem{Label: "View", Submenu: []*mygo.MenuItem{
		a.when(a.item("workbench.action.showCommands", "Command Palette..."), inProject),
		mygo.Separator(),
		a.when(a.item("workbench.view.explorer", "Explorer"), inProject),
		a.when(a.item("workbench.view.scm", "Source Control"), inProject),
		a.when(a.item("workbench.view.github", "GitHub"), inProject),
		a.when(a.item("workbench.actions.view.problems", "Problems"), inProject),
		a.when(a.item("todo.show", "TODO"), inProject),
		a.when(a.item("notifications.showList", "Notifications"), inProject),
		mygo.Separator(),
		a.checkItem(sidebarCommand, "Primary Side Bar", inProject, func(s workspace.ViewState) bool { return s.Sidebar }),
		a.checkItem(panelCommand, "Panel", inProject, func(s workspace.ViewState) bool { return s.Panel }),
		a.when(a.item("workbench.action.togglePanelPosition", ""), inProject),
		mygo.Separator(),
		a.when(a.item("workbench.action.splitEditorRight", "Split Right"), withEditor),
		a.when(a.item("workbench.action.splitEditorDown", "Split Down"), withEditor),
		mygo.Separator(),
		a.when(a.item("workbench.action.nextEditor", "Next Editor"), withEditor),
		a.when(a.item("workbench.action.previousEditor", "Previous Editor"), withEditor),
		a.when(a.item("workbench.action.reopenClosedEditor", ""), inProject),
		mygo.Separator(),
		a.item("workbench.action.zoomIn", ""),
		a.item("workbench.action.zoomOut", ""),
		a.item("workbench.action.zoomReset", ""),
		mygo.Separator(),
		a.checkItem(wordWrapCommand, "Word Wrap", inProject, func(s workspace.ViewState) bool { return s.WordWrap }),
		mygo.Separator(),
		a.menu.fullScreen,
	}}
}

func (a *App) goMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Label: "Go", Submenu: []*mygo.MenuItem{
		a.when(a.item("workbench.action.navigateBack", "Back"), canGoBack),
		a.when(a.item("workbench.action.navigateForward", "Forward"), canGoForward),
		a.when(a.item("workbench.action.navigateToLastEditLocation", "Last Edit Location"), inProject),
		mygo.Separator(),
		a.when(a.item("workbench.action.searchEverywhere", ""), inProject),
		a.when(a.item("workbench.action.quickOpen", ""), inProject),
		a.when(a.item("workbench.action.showRecentFiles", ""), inProject),
		a.when(a.item("workbench.action.showAllSymbols", ""), inProject),
		a.when(a.item("workbench.action.gotoSymbol", ""), withText),
		a.when(a.item(goToLineCommand, ""), withText),
		mygo.Separator(),
		a.when(a.item("workbench.action.findAction", ""), inProject),
	}}
}

func (a *App) gitMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Label: "Git", Submenu: []*mygo.MenuItem{
		a.when(a.item("git.commit", "Commit..."), withGit),
		a.when(a.item("git.push", ""), withGit),
		a.when(a.item("git.update", ""), withGit),
		a.when(a.item("git.pull", ""), withGit),
		a.when(a.item("git.fetch", ""), withGit),
		a.when(a.item("git.clone", "Clone..."), inProject),
		mygo.Separator(),
		a.when(a.item("git.branches", ""), withGit),
		a.when(a.item("git.newBranch", ""), withGit),
		mygo.Separator(),
		a.when(a.item("git.showLog", ""), withGit),
		a.when(a.item("git.fileHistory", "Show History for File"), withGitFile),
		a.when(a.item("git.annotate", ""), withGitFile),
		mygo.Separator(),
		a.when(a.item("git.stash", ""), withGit),
		a.when(a.item("git.stashes", ""), withGit),
		a.when(a.item("git.shelve", ""), withGit),
		a.when(a.item("git.shelf", ""), withGit),
		mygo.Separator(),
		a.when(a.item("git.rollback", ""), withGit),
		a.when(a.item("git.resolveConflicts", ""), withGit),
	}}
}

func (a *App) helpMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Role: mygo.RoleHelp, Submenu: []*mygo.MenuItem{
		a.when(a.item("workbench.action.showCommands", ""), inProject),
		a.when(a.item(keybindingsID, "Keyboard Shortcuts"), inProject),
		mygo.Separator(),
		a.item("workbench.action.openLogsFolder", ""),
	}}
}

func (a *App) recentItems() []*mygo.MenuItem {
	var items []*mygo.MenuItem
	for _, root := range a.store.State.Recent[:min(len(a.store.State.Recent), maxRecentMenuItems)] {
		items = append(items, &mygo.MenuItem{
			Label:    filepath.Base(root),
			Subtitle: fsutil.DisplayPath(root),
			Click:    func(_ *mygo.MenuItem, win *mygo.Window) { a.openFolder(win, root, true) },
		})
	}
	if len(items) > 0 {
		items = append(items, mygo.Separator())
	}
	return append(items, &mygo.MenuItem{Label: "Clear Recent", Disabled: len(a.store.State.Recent) == 0, Click: a.clearRecent})
}

func (a *App) clearRecent(*mygo.MenuItem, *mygo.Window) {
	a.store.State.Recent = nil
	mygo.App.ClearRecentDocuments()
	a.saveSession()
	a.refreshMenu()
}

func (a *App) closeWindow(_ *mygo.MenuItem, win *mygo.Window) {
	if win == nil {
		win = mygo.FocusedWindow()
	}
	if win != nil {
		win.Close()
	}
}
