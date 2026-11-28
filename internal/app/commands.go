package app

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/gitui"
	"github.com/ZerGo0/editor/internal/history"
	"github.com/ZerGo0/editor/internal/issueui"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/prui"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/shellcmd"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/ZerGo0/editor/internal/workspace/symbolnav"
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
	categoryReview      = "Review"
	maxEditorIndex      = 9
)

func (a *App) registerCommands() {
	a.registerWorkbenchCommands()
	a.registerEditorGroupCommands()
	a.registerPanelCommands()
	a.registerEditorCommands()
	a.registerKeymapCommands()
	a.registerReviewCommands()
}

func appAction(fn func()) func(*mygo.Window) { return func(*mygo.Window) { fn() } }

func (a *App) addCommand(id, category, title string, run func(*mygo.Window)) {
	a.commands.Add(&command.Command{ID: id, Category: category, Title: title, Run: run})
}

func (a *App) registerWorkbenchCommands() {
	ws, app, add := a.inWorkspace, appAction, a.addCommand
	add("workbench.action.showCommands", categoryView, "Show All Commands", ws((*workspace.Workspace).ShowCommands))
	add("workbench.action.openSettings", categoryPreferences, "Open Settings (UI)", ws((*workspace.Workspace).OpenSettings))
	add("workbench.action.openSettingsJson", categoryPreferences, "Open User Settings (JSON)", ws((*workspace.Workspace).OpenSettingsJSON))
	add("workbench.action.openFolderSettings", categoryPreferences, "Open Folder Settings", ws((*workspace.Workspace).OpenFolderSettings))
	add("workbench.action.openFolderSettingsFile", categoryPreferences, "Open Folder Settings (JSON)", ws((*workspace.Workspace).OpenFolderSettingsJSON))
	add("workbench.action.openGlobalKeybindings", categoryPreferences, "Open Keyboard Shortcuts", ws((*workspace.Workspace).OpenKeybindings))
	add("workbench.action.openGlobalKeybindingsFile", categoryPreferences, "Open Keyboard Shortcuts (JSON)", ws((*workspace.Workspace).OpenKeybindingsJSON))
	add("workbench.action.files.save", categoryFile, "Save", ws((*workspace.Workspace).Save))
	add("workbench.action.files.saveAll", categoryFile, "Save All", ws((*workspace.Workspace).SaveAll))
	add("workbench.action.files.saveAs", categoryFile, "Save as...", ws((*workspace.Workspace).SaveAs))
	add("workbench.action.files.revert", categoryFile, "Revert File", ws((*workspace.Workspace).RevertFile))
	add("workbench.files.action.compareWithSaved", categoryFile, "Compare Active File with Saved", ws((*workspace.Workspace).CompareWithSaved))
	add("workbench.files.action.compareWithClipboard", categoryFile, "Compare Active File with Clipboard", ws((*workspace.Workspace).CompareWithClipboard))
	add("keybindings.editor.searchKeybindings", categoryPreferences, "Search Keybindings", ws((*workspace.Workspace).SearchKeybindings))
	add("keybindings.editor.recordSearchKeys", categoryPreferences, "Record Keys", ws((*workspace.Workspace).RecordKeybindingSearchKeys))
	add("settings.action.search", categoryPreferences, "Focus Settings Search", ws((*workspace.Workspace).SearchSettings))
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
	add("workbench.action.closeSidebar", categoryView, "Hide Primary Side Bar", ws((*workspace.Workspace).CloseSidebar))
	add("workbench.view.explorer", categoryView, "Show Explorer", ws((*workspace.Workspace).FocusExplorer))
	add("workbench.action.focusSideBar", categoryView, "Focus into Primary Side Bar", ws((*workspace.Workspace).FocusSideBar))
	add("workbench.action.editor.changeLanguageMode", categoryView, "Change Language Mode", ws((*workspace.Workspace).ChangeLanguage))
	add("explorer.openToSide", categoryExplorer, "Open to the Side", ws((*workspace.Workspace).OpenSelectedToSide))
	add("list.find", categoryExplorer, "Find", ws((*workspace.Workspace).FindInExplorer))
	add("filesExplorer.findInFolder", categoryExplorer, "Find in Folder...", ws((*workspace.Workspace).FindInSelectedFolder))
	add("selectForCompare", categoryExplorer, "Select for Compare", ws((*workspace.Workspace).SelectForCompare))
	add("compareFiles", categoryExplorer, "Compare with Selected", ws((*workspace.Workspace).CompareWithSelected))
	add("workbench.files.action.showActiveFileInExplorer", categoryExplorer, "Reveal Active File in Explorer View", ws((*workspace.Workspace).RevealInSidebar))
}

func (a *App) registerEditorGroupCommands() {
	ws, add := a.inWorkspace, a.addCommand
	for _, split := range []struct {
		id, title string
		side      layout.Direction
	}{
		{"workbench.action.splitEditorRight", "Split Editor Right", layout.Right},
		{"workbench.action.splitEditorDown", "Split Editor Down", layout.Down},
		{"workbench.action.splitEditorLeft", "Split Editor Left", layout.Left},
		{"workbench.action.splitEditorUp", "Split Editor Up", layout.Up},
	} {
		add(split.id, categoryView, split.title, ws(func(w *workspace.Workspace) { w.SplitEditor(split.side) }))
	}
	for _, move := range []struct {
		id, title string
		side      layout.Direction
	}{
		{"workbench.action.moveEditorToLeftGroup", "Move Editor into Left Group", layout.Left},
		{"workbench.action.moveEditorToRightGroup", "Move Editor into Right Group", layout.Right},
		{"workbench.action.moveEditorToAboveGroup", "Move Editor into Group Above", layout.Up},
		{"workbench.action.moveEditorToBelowGroup", "Move Editor into Group Below", layout.Down},
	} {
		add(move.id, categoryView, move.title, ws(func(w *workspace.Workspace) { w.MoveEditorToSide(move.side) }))
	}
	add("workbench.action.moveEditorToNextGroup", categoryView, "Move Editor into Next Group", ws(func(w *workspace.Workspace) { w.MoveEditorToAdjacentGroup(1) }))
	add("workbench.action.moveEditorToPreviousGroup", categoryView, "Move Editor into Previous Group", ws(func(w *workspace.Workspace) { w.MoveEditorToAdjacentGroup(-1) }))
	add("workbench.action.moveEditorToFirstGroup", categoryView, "Move Editor into First Group", ws(func(w *workspace.Workspace) { w.MoveEditorToEdgeGroup(false) }))
	add("workbench.action.moveEditorToLastGroup", categoryView, "Move Editor into Last Group", ws(func(w *workspace.Workspace) { w.MoveEditorToEdgeGroup(true) }))
	add("workbench.action.moveEditorLeftInGroup", categoryView, "Move Editor Left", ws(func(w *workspace.Workspace) { w.MoveEditorInGroup(-1) }))
	add("workbench.action.moveEditorRightInGroup", categoryView, "Move Editor Right", ws(func(w *workspace.Workspace) { w.MoveEditorInGroup(1) }))
	add("workbench.action.nextEditor", categoryView, "Open Next Editor", ws(func(w *workspace.Workspace) { w.CycleTab(1) }))
	add("workbench.action.previousEditor", categoryView, "Open Previous Editor", ws(func(w *workspace.Workspace) { w.CycleTab(-1) }))
	for i := range maxEditorIndex {
		n := strconv.Itoa(i + 1)
		add("workbench.action.openEditorAtIndex"+n, categoryView, "Open Editor at Index "+n, ws(func(w *workspace.Workspace) { w.OpenEditorAtIndex(i) }))
	}
	add("workbench.action.lastEditorInGroup", categoryView, "Open Last Editor in Group", ws((*workspace.Workspace).LastEditorInGroup))
	add("workbench.action.showAllEditors", categoryView, "Show All Editors by Appearance", ws((*workspace.Workspace).ShowAllEditors))
	add("workbench.action.toggleMaximizeEditorGroup", categoryView, "Toggle Maximize Editor Group", ws((*workspace.Workspace).ToggleMaximizeEditorGroup))
	add("workbench.action.evenEditorWidths", categoryView, "Reset Editor Group Sizes", ws((*workspace.Workspace).EvenEditorWidths))
	add("workbench.action.toggleEditorWidths", categoryView, "Toggle Editor Group Sizes", ws((*workspace.Workspace).ToggleEditorWidths))
	for _, r := range []struct {
		id, title     string
		delta         float32
		width, height bool
	}{
		{"workbench.action.increaseViewSize", "Increase Current View Size", workspace.ViewSizeStep, true, true},
		{"workbench.action.decreaseViewSize", "Decrease Current View Size", -workspace.ViewSizeStep, true, true},
		{"workbench.action.increaseViewWidth", "Increase Current View Width", workspace.ViewSizeStep, true, false},
		{"workbench.action.decreaseViewWidth", "Decrease Current View Width", -workspace.ViewSizeStep, true, false},
		{"workbench.action.increaseViewHeight", "Increase Current View Height", workspace.ViewSizeStep, false, true},
		{"workbench.action.decreaseViewHeight", "Decrease Current View Height", -workspace.ViewSizeStep, false, true},
	} {
		add(r.id, categoryView, r.title, ws(func(w *workspace.Workspace) { w.ResizeView(r.delta, r.width, r.height) }))
	}
	add("workbench.action.maximizeEditorHideSidebar", categoryView, "Maximize Editor Group and Hide Side Bars", ws((*workspace.Workspace).ToggleHideToolWindows))
	for i, name := range []string{"First", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth"} {
		add("workbench.action.focus"+name+"EditorGroup", categoryView, "Focus "+name+" Editor Group", ws(func(w *workspace.Workspace) { w.FocusGroup(i) }))
	}
	add("workbench.action.focusPreviousGroup", categoryView, "Focus Previous Editor Group", ws(func(w *workspace.Workspace) { w.FocusRelativeGroup(-1) }))
	add("workbench.action.focusNextGroup", categoryView, "Focus Next Editor Group", ws(func(w *workspace.Workspace) { w.FocusRelativeGroup(1) }))
	add("workbench.action.files.revealActiveFileInFinder", categoryFile, "Reveal Active File in Finder", ws((*workspace.Workspace).RevealActiveInFinder))
	add("workbench.action.files.copyPathOfActiveFile", categoryFile, "Copy Path of Active File", ws(func(w *workspace.Workspace) { w.CopyActivePath(false) }))
	add("workbench.action.files.copyRelativePathOfActiveFile", categoryFile, "Copy Relative Path of Active File", ws(func(w *workspace.Workspace) { w.CopyActivePath(true) }))
}

func (a *App) registerPanelCommands() {
	ws, add := a.inWorkspace, a.addCommand
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
	add("workbench.action.toggleMaximizedPanel", categoryView, "Toggle Maximized Panel", ws((*workspace.Workspace).ToggleMaximizedPanel))
	add("breadcrumbs.focus", categoryView, "Focus Breadcrumbs", ws(func(w *workspace.Workspace) { w.FocusBreadcrumbs(false) }))
	add("breadcrumbs.focusAndSelect", categoryView, "Focus and Select Breadcrumbs", ws(func(w *workspace.Workspace) { w.FocusBreadcrumbs(true) }))
	add("editor.action.marker.next", "", "Go to Next Problem (Error, Warning, Info)", ws(func(w *workspace.Workspace) { w.GoToProblem(1, false) }))
	add("editor.action.marker.prev", "", "Go to Previous Problem (Error, Warning, Info)", ws(func(w *workspace.Workspace) { w.GoToProblem(-1, false) }))
	add("editor.action.marker.nextInFiles", "", "Go to Next Problem in Files (Error, Warning, Info)", ws(func(w *workspace.Workspace) { w.GoToProblem(1, true) }))
	add("editor.action.marker.prevInFiles", "", "Go to Previous Problem in Files (Error, Warning, Info)", ws(func(w *workspace.Workspace) { w.GoToProblem(-1, true) }))
	add("workbench.action.navigateBack", categoryGo, "Go Back", ws((*workspace.Workspace).NavigateBack))
	add("workbench.action.navigateForward", categoryGo, "Go Forward", ws((*workspace.Workspace).NavigateForward))
	add("workbench.action.navigateToLastEditLocation", categoryGo, "Go to Last Edit Location", ws((*workspace.Workspace).NavigateToLastEdit))
	add("workbench.action.focusNextPart", categoryView, "Focus Next Part", ws((*workspace.Workspace).FocusNextPart))
	add("workbench.action.focusPreviousPart", categoryView, "Focus Previous Part", ws((*workspace.Workspace).FocusPreviousPart))
	add("notifications.showList", categoryView, "Show Notifications", ws((*workspace.Workspace).ToggleNotifications))
	add("notifications.clearAll", categoryView, "Clear All Notifications", ws((*workspace.Workspace).ClearNotifications))
	add("notifications.hideList", categoryView, "Hide Notifications", ws((*workspace.Workspace).HideNotifications))
	add("notifications.hideToasts", categoryView, "Hide Notification Toasts", ws((*workspace.Workspace).HideToasts))
	add("notifications.focusToasts", categoryView, "Focus Notification Toast", ws((*workspace.Workspace).FocusToasts))
	add("notifications.toggleDoNotDisturbMode", categoryView, "Toggle Do Not Disturb Mode", ws((*workspace.Workspace).ToggleDoNotDisturb))
	add("workbench.actions.view.problems", categoryView, "Show Problems", ws(func(w *workspace.Workspace) { w.FocusPanel(workspace.ProblemsPanel) }))
	add("todo.show", categoryView, "Show TODO", ws(func(w *workspace.Workspace) { w.FocusPanel(workspace.TODOPanel) }))
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
}

func (a *App) registerEditorCommands() {
	ws, app, add := a.inWorkspace, appAction, a.addCommand
	add("markdown.showPreview", categoryView, "Open Preview", ws((*workspace.Workspace).OpenPreview))
	add("markdown.showPreviewToSide", categoryView, "Open Preview to the Side", ws((*workspace.Workspace).OpenPreviewToSide))
	add("workbench.action.zoomIn", categoryView, "Zoom In", app(func() { a.zoom(1) }))
	add("workbench.action.zoomOut", categoryView, "Zoom Out", app(func() { a.zoom(-1) }))
	add("workbench.action.zoomReset", categoryView, "Reset Zoom", app(func() { a.zoom(0) }))
	add("workbench.action.openLogsFolder", categoryDeveloper, "Open Logs Folder", app(openLogsFolder))
	add("workbench.action.installCommandLine", categoryFile, "Install '"+shellcmd.Name+"' Command in PATH", func(win *mygo.Window) { a.installCommand(nil, win) })
	add("workbench.action.uninstallCommandLine", categoryFile, "Uninstall '"+shellcmd.Name+"' Command", func(win *mygo.Window) { a.uninstallCommand(nil, win) })
	add("workbench.action.gotoLine", categoryGo, "Go to Line/Column...", ws((*workspace.Workspace).GoToLine))
	for _, act := range symbolnav.Actions {
		add(act.ID, "", act.Title, ws(func(w *workspace.Workspace) { w.RunSymbolNav(act) }))
	}
	for cmd := editor.NoCommand + 1; cmd.String() != "unknown"; cmd++ {
		if cmd == editor.GoToLine || cmd == editor.GoToDefinition {
			continue
		}
		title, ok := keymap.EditorTitle(cmd)
		if !ok {
			title = humanize(cmd.String())
		}
		add(keymap.EditorCommandID(cmd), categoryEditor, title, ws(func(w *workspace.Workspace) { w.RunEditor(cmd) }))
	}
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
