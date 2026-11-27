package app

import (
	"strings"
	"unicode"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/gitui"
	"github.com/ZerGo0/editor/internal/history"
	"github.com/ZerGo0/editor/internal/issueui"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/prui"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/shellcmd"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
)

const (
	categoryFile        = "File"
	categoryView        = "View"
	categoryPreferences = "Preferences"
	categoryEditor      = "Editor"
	categoryExplorer    = "Explorer"
	categoryGo          = "Go"
	categorySearch      = "Search"
	categoryDeveloper   = "Developer"
)

func (a *App) registerCommands() {
	ws := func(fn func(*workspace.Workspace)) func(*mygo.Window) {
		return func(win *mygo.Window) {
			if pw := a.workspaceFor(win); pw != nil {
				pw.win.Update(func() { fn(pw.ws) })
			}
		}
	}
	app := func(fn func()) func(*mygo.Window) { return func(*mygo.Window) { fn() } }
	add := func(id, category, title string, run func(*mygo.Window)) {
		a.commands.Add(&command.Command{ID: id, Category: category, Title: title, Run: run})
	}
	add("workbench.action.showCommands", categoryView, "Show All Commands", ws((*workspace.Workspace).ShowCommands))
	add("workbench.action.openSettings", categoryPreferences, "Open Settings (UI)", ws((*workspace.Workspace).OpenSettings))
	add("workbench.action.openSettingsJson", categoryPreferences, "Open User Settings (JSON)", ws((*workspace.Workspace).OpenSettingsJSON))
	add("workbench.action.openFolderSettings", categoryPreferences, "Open Folder Settings", ws((*workspace.Workspace).OpenFolderSettings))
	add("workbench.action.openFolderSettingsFile", categoryPreferences, "Open Folder Settings (JSON)", ws((*workspace.Workspace).OpenFolderSettingsJSON))
	add("workbench.action.openGlobalKeybindings", categoryPreferences, "Open Keyboard Shortcuts", ws((*workspace.Workspace).OpenKeybindings))
	add("workbench.action.openGlobalKeybindingsFile", categoryPreferences, "Open Keyboard Shortcuts (JSON)", ws((*workspace.Workspace).OpenKeybindingsJSON))
	add("workbench.action.files.save", categoryFile, "Save", ws((*workspace.Workspace).Save))
	add("workbench.action.files.saveAll", categoryFile, "Save All", ws((*workspace.Workspace).SaveAll))
	add("workbench.action.closeActiveEditor", categoryView, "Close Editor", ws((*workspace.Workspace).CloseActiveTab))
	add("workbench.action.closeAllEditors", categoryView, "Close All Editors", ws((*workspace.Workspace).CloseAllEditors))
	add("workbench.action.closeEditorsInGroup", categoryView, "Close All Editors in Group", ws((*workspace.Workspace).CloseGroup))
	add("workbench.action.closeUnmodifiedEditors", categoryView, "Close Saved Editors in Group", ws((*workspace.Workspace).CloseSaved))
	add("workbench.action.closeOtherEditors", categoryView, "Close Other Editors in Group", ws((*workspace.Workspace).CloseOthers))
	add("workbench.action.reopenClosedEditor", categoryView, "Reopen Closed Editor", ws((*workspace.Workspace).ReopenClosed))
	add("workbench.action.keepEditor", categoryView, "Keep Open", ws((*workspace.Workspace).KeepOpen))
	add("workbench.action.pinEditor", categoryView, "Pin", ws((*workspace.Workspace).TogglePinActive))
	add("workbench.action.unpinEditor", categoryView, "Unpin", ws((*workspace.Workspace).UnpinActive))
	add("workbench.action.closeWindow", categoryView, "Close Window", func(win *mygo.Window) { a.closeWindow(nil, win) })
	add("workbench.action.newWindow", categoryFile, "New Window", app(a.showWelcome))
	add("workbench.action.files.openFolder", categoryFile, "Open Folder...", a.openFolderDialog)
	add("workbench.action.files.openFile", categoryFile, "Open File...", a.openFileDialog)
	add("workbench.action.quit", categoryFile, "Quit", app(mygo.App.Quit))
	add("workbench.action.openRecent", categoryFile, "Open Recent...", ws((*workspace.Workspace).ShowOpenRecent))
	add("workbench.action.files.newUntitledFile", categoryFile, "New Text File", ws((*workspace.Workspace).NewUntitled))
	add("explorer.newFile", categoryFile, "New File...", ws((*workspace.Workspace).NewFile))
	add("explorer.newFolder", categoryFile, "New Folder...", ws((*workspace.Workspace).NewFolder))
	add("workbench.action.toggleSidebarVisibility", categoryView, "Toggle Primary Side Bar Visibility", ws((*workspace.Workspace).ToggleSidebar))
	add("workbench.view.explorer", categoryView, "Show Explorer", ws((*workspace.Workspace).FocusExplorer))
	add("workbench.action.focusSideBar", categoryView, "Focus into Primary Side Bar", ws((*workspace.Workspace).FocusSideBar))
	add("workbench.action.editor.changeLanguageMode", categoryView, "Change Language Mode", ws((*workspace.Workspace).ChangeLanguage))
	add("workbench.files.action.showActiveFileInExplorer", categoryExplorer, "Reveal Active File in Explorer View", ws((*workspace.Workspace).RevealInSidebar))
	add("workbench.action.splitEditorRight", categoryView, "Split Editor Right", ws(func(w *workspace.Workspace) { w.Split(false) }))
	add("workbench.action.splitEditorDown", categoryView, "Split Editor Down", ws(func(w *workspace.Workspace) { w.Split(true) }))
	add("workbench.action.nextEditor", categoryView, "Open Next Editor", ws(func(w *workspace.Workspace) { w.CycleTab(1) }))
	add("workbench.action.previousEditor", categoryView, "Open Previous Editor", ws(func(w *workspace.Workspace) { w.CycleTab(-1) }))
	for i, name := range []string{"First", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth"} {
		add("workbench.action.focus"+name+"EditorGroup", categoryView, "Focus "+name+" Editor Group", ws(func(w *workspace.Workspace) { w.FocusGroup(i) }))
	}
	add("workbench.action.focusPreviousGroup", categoryView, "Focus Previous Editor Group", ws(func(w *workspace.Workspace) { w.FocusRelativeGroup(-1) }))
	add("workbench.action.focusNextGroup", categoryView, "Focus Next Editor Group", ws(func(w *workspace.Workspace) { w.FocusRelativeGroup(1) }))
	add("workbench.action.files.revealActiveFileInFinder", categoryFile, "Reveal Active File in Finder", ws((*workspace.Workspace).RevealActiveInFinder))
	add("workbench.action.files.copyPathOfActiveFile", categoryFile, "Copy Path of Active File", ws(func(w *workspace.Workspace) { w.CopyActivePath(false) }))
	add("workbench.action.files.copyRelativePathOfActiveFile", categoryFile, "Copy Relative Path of Active File", ws(func(w *workspace.Workspace) { w.CopyActivePath(true) }))
	search := func(tab finder.Tab) func(*mygo.Window) {
		return ws(func(w *workspace.Workspace) { w.OpenSearch(tab) })
	}
	add("workbench.view.search", categoryView, "Show Search", ws((*workspace.Workspace).ShowSearch))
	add("workbench.action.searchEverywhere", categoryGo, "Search Everywhere", search(finder.TabAll))
	add("workbench.action.quickOpen", categoryGo, "Go to File...", search(finder.TabFiles))
	add("workbench.action.findAction", categoryGo, "Find Action...", search(finder.TabActions))
	add("workbench.action.showRecentFiles", categoryGo, "Recent Files", ws((*workspace.Workspace).ShowRecentFiles))
	add("search.showFindWindow", categorySearch, "Show Find Window", ws(func(w *workspace.Workspace) { w.ShowPanel(workspace.FindPanel) }))
	add("workbench.action.showAllSymbols", categoryGo, "Go to Symbol in Workspace...", ws((*workspace.Workspace).GoToSymbolInWorkspace))
	add("workbench.action.findInFiles", categoryGo, "Find in Files", ws((*workspace.Workspace).FindInFiles))
	add("workbench.action.gotoSymbol", categoryGo, "Go to Symbol in Editor...", ws((*workspace.Workspace).GoToSymbolInEditor))
	add("search.action.focusNextSearchResult", categorySearch, "Focus Next Search Result", ws((*workspace.Workspace).NextSearchResult))
	add("search.action.focusPreviousSearchResult", categorySearch, "Focus Previous Search Result", ws((*workspace.Workspace).PreviousSearchResult))
	add("workbench.action.togglePanel", categoryView, "Toggle Panel Visibility", ws((*workspace.Workspace).TogglePanel))
	add("workbench.action.togglePanelPosition", categoryView, "Toggle Panel Position", ws((*workspace.Workspace).TogglePanelPosition))
	add("workbench.action.navigateBack", categoryGo, "Go Back", ws((*workspace.Workspace).NavigateBack))
	add("workbench.action.navigateForward", categoryGo, "Go Forward", ws((*workspace.Workspace).NavigateForward))
	add("workbench.action.navigateToLastEditLocation", categoryGo, "Go to Last Edit Location", ws((*workspace.Workspace).NavigateToLastEdit))
	add("workbench.action.focusNextPart", categoryView, "Focus Next Part", ws((*workspace.Workspace).FocusNextPart))
	add("workbench.action.focusPreviousPart", categoryView, "Focus Previous Part", ws((*workspace.Workspace).FocusPreviousPart))
	add("notifications.showList", categoryView, "Show Notifications", ws((*workspace.Workspace).ToggleNotifications))
	add("notifications.clearAll", categoryView, "Clear All Notifications", ws((*workspace.Workspace).ClearNotifications))
	add("workbench.actions.view.problems", categoryView, "Show Problems", ws(func(w *workspace.Workspace) { w.ShowPanel(workspace.ProblemsPanel) }))
	add("todo.show", categoryView, "Show TODO", ws(func(w *workspace.Workspace) { w.ShowPanel(workspace.TODOPanel) }))
	for _, act := range history.Actions {
		add(act.ID, act.Category, act.Title, ws(func(w *workspace.Workspace) { w.RunHistory(act.ID) }))
	}
	add("workbench.view.scm", categoryView, "Show Source Control", ws(func(w *workspace.Workspace) { w.FocusSideView(workspace.SourceControlView) }))
	add("workbench.view.github", categoryView, "Show GitHub", ws(func(w *workspace.Workspace) { w.FocusSideView(workspace.GitHubView) }))
	for _, act := range gitui.Actions {
		add(act.ID, act.Category, act.Title, ws(func(w *workspace.Workspace) { w.RunGit(act.ID) }))
	}
	for _, act := range prui.Actions {
		add(act.ID, act.Category, act.Title, ws(func(w *workspace.Workspace) { w.RunPullRequests(act.ID) }))
	}
	for _, act := range issueui.Actions {
		add(act.ID, act.Category, act.Title, ws(func(w *workspace.Workspace) { w.RunIssues(act.ID) }))
	}
	add("markdown.showPreview", categoryView, "Open Preview", ws((*workspace.Workspace).OpenPreview))
	add("markdown.showPreviewToSide", categoryView, "Open Preview to the Side", ws((*workspace.Workspace).OpenPreviewToSide))
	add("workbench.action.zoomIn", categoryView, "Zoom In", app(func() { a.zoom(1) }))
	add("workbench.action.zoomOut", categoryView, "Zoom Out", app(func() { a.zoom(-1) }))
	add("workbench.action.zoomReset", categoryView, "Reset Zoom", app(func() { a.zoom(0) }))
	add("workbench.action.openLogsFolder", categoryDeveloper, "Open Logs Folder", app(openLogsFolder))
	add("workbench.action.installCommandLine", categoryFile, "Install '"+shellcmd.Name+"' Command in PATH", func(win *mygo.Window) { a.installCommand(nil, win) })
	add("workbench.action.uninstallCommandLine", categoryFile, "Uninstall '"+shellcmd.Name+"' Command", func(win *mygo.Window) { a.uninstallCommand(nil, win) })
	add("workbench.action.gotoLine", categoryGo, "Go to Line/Column...", ws((*workspace.Workspace).GoToLine))
	for cmd := editor.NoCommand + 1; cmd.String() != "unknown"; cmd++ {
		if cmd == editor.GoToLine {
			continue
		}
		add(keymap.EditorCommandID(cmd), categoryEditor, humanize(cmd.String()), ws(func(w *workspace.Workspace) { w.RunEditor(cmd) }))
	}
	a.registerKeymapCommands()
}

func (a *App) zoom(step int) {
	level := 0
	if step != 0 {
		level = a.settings.Int(settings.ZoomLevel) + step
	}
	if err := a.settings.Set(settings.ZoomLevel, level); err != nil {
		go mygo.Dialog.Error("Cannot change the zoom", err.Error())
	}
	a.settingsChanged()
}

func humanize(name string) string {
	var b strings.Builder
	for i, r := range name {
		switch {
		case i == 0:
			r = unicode.ToUpper(r)
		case unicode.IsUpper(r) || unicode.IsDigit(r) && !unicode.IsDigit(rune(name[i-1])):
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
