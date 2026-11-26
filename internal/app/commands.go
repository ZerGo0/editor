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
	searchview "github.com/ZerGo0/editor/internal/search/view"
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
	add("workbench.action.openSettings", categoryPreferences, "Open Settings", ws((*workspace.Workspace).OpenSettings))
	add("workbench.action.openSettingsJson", categoryPreferences, "Open Settings (JSON)", ws((*workspace.Workspace).OpenSettingsJSON))
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
	add("workbench.action.keepEditor", categoryView, "Keep Editor Open", ws((*workspace.Workspace).KeepOpen))
	add("workbench.action.pinEditor", categoryView, "Pin Editor", ws((*workspace.Workspace).TogglePinActive))
	add("workbench.action.unpinEditor", categoryView, "Unpin Editor", ws((*workspace.Workspace).UnpinActive))
	add("workbench.action.closeWindow", categoryView, "Close Window", func(win *mygo.Window) { a.closeWindow(nil, win) })
	add("workbench.action.newWindow", categoryFile, "New Window", app(a.showWelcome))
	add("workbench.action.files.openFolder", categoryFile, "Open Folder...", a.openFolderDialog)
	add("workbench.action.files.newUntitledFile", categoryFile, "New Text File", ws((*workspace.Workspace).NewUntitled))
	add("explorer.newFile", categoryFile, "New File...", ws((*workspace.Workspace).NewFile))
	add("explorer.newFolder", categoryFile, "New Folder...", ws((*workspace.Workspace).NewFolder))
	add("workbench.action.toggleSidebarVisibility", categoryView, "Toggle Sidebar Visibility", ws((*workspace.Workspace).ToggleSidebar))
	add("workbench.view.explorer", categoryView, "Show Explorer", ws((*workspace.Workspace).FocusExplorer))
	add("workbench.files.action.showActiveFileInExplorer", categoryExplorer, "Reveal Active File in Sidebar", ws((*workspace.Workspace).RevealInSidebar))
	add("workbench.action.splitEditorRight", categoryView, "Split Editor Right", ws(func(w *workspace.Workspace) { w.Split(false) }))
	add("workbench.action.splitEditorDown", categoryView, "Split Editor Down", ws(func(w *workspace.Workspace) { w.Split(true) }))
	add("workbench.action.nextEditor", categoryView, "Open Next Editor", ws(func(w *workspace.Workspace) { w.CycleTab(1) }))
	add("workbench.action.previousEditor", categoryView, "Open Previous Editor", ws(func(w *workspace.Workspace) { w.CycleTab(-1) }))
	for i, name := range []string{"First", "Second", "Third"} {
		add("workbench.action.focus"+name+"EditorGroup", categoryView, "Focus "+name+" Editor Group", ws(func(w *workspace.Workspace) { w.FocusGroup(i) }))
	}
	add("workbench.action.focusPreviousGroup", categoryView, "Focus Previous Editor Group", ws(func(w *workspace.Workspace) { w.FocusRelativeGroup(-1) }))
	add("workbench.action.focusNextGroup", categoryView, "Focus Next Editor Group", ws(func(w *workspace.Workspace) { w.FocusRelativeGroup(1) }))
	add("workbench.action.files.revealActiveFileInFinder", categoryFile, "Reveal Active File in Finder", ws((*workspace.Workspace).RevealActiveInFinder))
	add("workbench.action.files.copyPathOfActiveFile", categoryFile, "Copy Path of Active File", ws(func(w *workspace.Workspace) { w.CopyActivePath(false) }))
	add("workbench.action.files.copyRelativePathOfActiveFile", categoryFile, "Copy Relative Path of Active File", ws(func(w *workspace.Workspace) { w.CopyActivePath(true) }))
	search := func(scope searchview.Scope) func(*mygo.Window) {
		return ws(func(w *workspace.Workspace) { w.FocusSearch(scope) })
	}
	add("workbench.view.search", categoryView, "Show Search", ws((*workspace.Workspace).ShowSearch))
	add("workbench.action.searchEverywhere", categoryGo, "Search Everywhere", search(searchview.ScopeAll))
	add("workbench.action.quickOpen", categoryGo, "Go to File...", search(searchview.ScopeFiles))
	add("workbench.action.findAction", categoryGo, "Find Action...", search(searchview.ScopeActions))
	add("workbench.action.showAllSymbols", categoryGo, "Go to Symbol in Workspace...", search(searchview.ScopeSymbols))
	add("workbench.action.findInFiles", categoryGo, "Find in Files", ws((*workspace.Workspace).FindInFiles))
	add("workbench.action.gotoSymbol", categoryGo, "Go to Symbol in Editor...", ws((*workspace.Workspace).GoToSymbolInEditor))
	add("search.action.focusNextSearchResult", categorySearch, "Focus Next Search Result", ws((*workspace.Workspace).NextSearchResult))
	add("search.action.focusPreviousSearchResult", categorySearch, "Focus Previous Search Result", ws((*workspace.Workspace).PreviousSearchResult))
	add("workbench.action.togglePanel", categoryView, "Toggle Panel", ws((*workspace.Workspace).TogglePanel))
	add("workbench.actions.view.problems", categoryView, "Show Problems", ws(func(w *workspace.Workspace) { w.ShowPanel(workspace.ProblemsPanel) }))
	add("todo.show", categoryView, "Show TODO", ws(func(w *workspace.Workspace) { w.ShowPanel(workspace.TODOPanel) }))
	for _, act := range history.Actions {
		add(act.ID, act.Category, act.Title, ws(func(w *workspace.Workspace) { w.RunHistory(act.ID) }))
	}
	add("workbench.view.scm", categoryView, "Show Source Control", ws(func(w *workspace.Workspace) { w.ShowSideView(workspace.SourceControlView) }))
	add("workbench.view.github", categoryView, "Show GitHub", ws(func(w *workspace.Workspace) { w.ShowSideView(workspace.GitHubView) }))
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
	add("workbench.action.installCommandLine", categoryFile, "Install '"+shellcmd.Name+"' Command in PATH", func(win *mygo.Window) { a.installCommand(nil, win) })
	add("workbench.action.uninstallCommandLine", categoryFile, "Uninstall '"+shellcmd.Name+"' Command", func(win *mygo.Window) { a.uninstallCommand(nil, win) })
	for cmd := editor.NoCommand + 1; cmd.String() != "unknown"; cmd++ {
		add(keymap.EditorCommandID(cmd), categoryEditor, humanize(cmd.String()), ws(func(w *workspace.Workspace) { w.RunEditor(cmd) }))
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
