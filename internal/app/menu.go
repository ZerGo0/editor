package app

import (
	"path/filepath"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/shellcmd"
	"github.com/egoist/mygo"
)

const (
	maxRecentMenuItems = 10
	goToLineCommand    = "workbench.action.gotoLine"
)

func (a *App) refreshMenu() {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		a.appMenu(),
		a.fileMenu(),
		{Role: mygo.RoleEditMenu},
		a.viewMenu(),
		a.goMenu(),
		a.gitMenu(),
		{Role: mygo.RoleWindowMenu},
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
	item := &mygo.MenuItem{Label: label, Click: func(_ *mygo.MenuItem, win *mygo.Window) { a.commands.Run(id, win) }}
	if a.options.NoMenuAccelerators {
		return item
	}
	a.commands.SetInMenu(id)
	if s, ok := a.keys.Current.MenuStroke(id); ok {
		item.Accelerator = s.Accelerator()
	}
	return item
}

func (a *App) appMenu() *mygo.MenuItem {
	installed := shellcmd.Installed()
	install, uninstall := a.item("workbench.action.installCommandLine", ""), a.item("workbench.action.uninstallCommandLine", "")
	install.Hidden, uninstall.Hidden = installed, !installed
	return &mygo.MenuItem{Label: "Editor", Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleAbout},
		mygo.Separator(),
		a.item("workbench.action.openSettings", "Settings"),
		a.item("workbench.action.openGlobalKeybindings", "Keyboard Shortcuts"),
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
		a.item("workbench.action.files.newUntitledFile", ""),
		a.item("explorer.newFile", ""),
		a.item("explorer.newFolder", ""),
		a.item("workbench.action.newWindow", ""),
		mygo.Separator(),
		a.item("workbench.action.files.openFolder", ""),
		{Label: "Open Recent", Submenu: a.recentItems()},
		mygo.Separator(),
		a.item("workbench.action.files.save", ""),
		a.item("workbench.action.files.saveAll", ""),
		mygo.Separator(),
		a.item("workbench.action.files.revealActiveFileInFinder", "Reveal in Finder"),
		mygo.Separator(),
		a.item("workbench.action.closeActiveEditor", ""),
		a.item("workbench.action.closeWindow", ""),
	}}
}

func (a *App) viewMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Label: "View", Submenu: []*mygo.MenuItem{
		a.item("workbench.action.showCommands", "Command Palette..."),
		mygo.Separator(),
		a.item("workbench.action.toggleSidebarVisibility", ""),
		a.item("workbench.view.explorer", "Explorer"),
		a.item("workbench.view.scm", "Source Control"),
		a.item("workbench.view.github", "GitHub"),
		a.item("workbench.action.togglePanel", ""),
		a.item("workbench.action.togglePanelPosition", ""),
		a.item("workbench.actions.view.problems", "Problems"),
		a.item("todo.show", "TODO"),
		a.item("notifications.showList", "Notifications"),
		mygo.Separator(),
		a.item("workbench.action.splitEditorRight", "Split Right"),
		a.item("workbench.action.splitEditorDown", "Split Down"),
		mygo.Separator(),
		a.item("workbench.action.nextEditor", "Next Editor"),
		a.item("workbench.action.previousEditor", "Previous Editor"),
		a.item("workbench.action.reopenClosedEditor", ""),
		mygo.Separator(),
		a.item("workbench.action.zoomIn", ""),
		a.item("workbench.action.zoomOut", ""),
		a.item("workbench.action.zoomReset", ""),
		mygo.Separator(),
		{Role: mygo.RoleToggleFullScreen},
	}}
}

func (a *App) goMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Label: "Go", Submenu: []*mygo.MenuItem{
		a.item("workbench.action.navigateBack", "Back"),
		a.item("workbench.action.navigateForward", "Forward"),
		a.item("workbench.action.navigateToLastEditLocation", "Last Edit Location"),
		mygo.Separator(),
		a.item("workbench.action.searchEverywhere", ""),
		a.item("workbench.action.quickOpen", ""),
		a.item("workbench.action.showRecentFiles", ""),
		a.item("workbench.action.showAllSymbols", ""),
		a.item("workbench.action.gotoSymbol", ""),
		a.item(goToLineCommand, ""),
		mygo.Separator(),
		a.item("workbench.action.findInFiles", ""),
		a.item("workbench.action.replaceInFiles", ""),
		a.item("workbench.action.findAction", ""),
	}}
}

func (a *App) gitMenu() *mygo.MenuItem {
	return &mygo.MenuItem{Label: "Git", Submenu: []*mygo.MenuItem{
		a.item("git.commit", "Commit..."),
		a.item("git.push", ""),
		a.item("git.update", ""),
		a.item("git.pull", ""),
		a.item("git.fetch", ""),
		mygo.Separator(),
		a.item("git.branches", ""),
		a.item("git.newBranch", ""),
		mygo.Separator(),
		a.item("git.showLog", ""),
		a.item("git.fileHistory", "Show History for File"),
		a.item("git.annotate", ""),
		mygo.Separator(),
		a.item("git.stash", ""),
		a.item("git.stashes", ""),
		a.item("git.shelve", ""),
		a.item("git.shelf", ""),
		mygo.Separator(),
		a.item("git.rollback", ""),
		a.item("git.resolveConflicts", ""),
	}}
}

func (a *App) recentItems() []*mygo.MenuItem {
	var items []*mygo.MenuItem
	for _, root := range a.store.State.Recent[:min(len(a.store.State.Recent), maxRecentMenuItems)] {
		items = append(items, &mygo.MenuItem{
			Label:    filepath.Base(root),
			Subtitle: fsutil.DisplayPath(root),
			Click:    func(*mygo.MenuItem, *mygo.Window) { a.openPath(root) },
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
