package keymap

import (
	"encoding/json"
	"slices"

	"github.com/ZerGo0/editor/internal/keymap/vscode"
	"github.com/ZerGo0/editor/internal/keymap/when"
)

const (
	newlineCommand       = "editor.newline"
	someQuickDiffChanges = "1"
	findActionCommand    = "workbench.action.findAction"
	replaceCommand       = "workbench.action.replaceInFiles"
	recentFilesCommand   = "workbench.action.showRecentFiles"
	previewCommand       = "markdown.showPreview"
	previewToSideCommand = "markdown.showPreviewToSide"
	amendCommand         = "git.amend"
	branchesCommand      = "git.branches"
	newBranchCommand     = "git.newBranch"
	amendKey             = "ctrl+alt+m"
	branchesKey          = "ctrl+shift+`"
	newBranchKey         = "ctrl+alt+n"
)

var presetTables = map[string][]vscode.Entry{
	PresetVSCode:        vscode.MacOS,
	PresetVSCodeWindows: vscode.Windows,
}

var appDefaults = map[string][]Entry{
	PresetVSCode: {
		{Key: "enter", Command: newlineCommand},
		{Key: "shift+enter", Command: newlineCommand},
		{Key: "cmd+shift+a", Command: findActionCommand},
		{Key: "cmd+shift+v", Command: previewCommand},
		{Key: "cmd+k v", Command: previewToSideCommand},
		{Key: amendKey, Command: amendCommand},
		{Key: branchesKey, Command: branchesCommand},
		{Key: newBranchKey, Command: newBranchCommand},
	},
	PresetVSCodeWindows: {
		{Key: "enter", Command: newlineCommand},
		{Key: "shift+enter", Command: newlineCommand},
		{Key: "ctrl+shift+a", Command: findActionCommand},
		{Key: "ctrl+shift+v", Command: previewCommand},
		{Key: "ctrl+k v", Command: previewToSideCommand},
		{Key: amendKey, Command: amendCommand},
		{Key: branchesKey, Command: branchesCommand},
		{Key: newBranchKey, Command: newBranchCommand},
	},
}

var jetBrainsDefaults = map[string][]Entry{
	PresetVSCode: {
		{Key: "cmd+shift+o", Command: quickOpenCommand},
		{Key: "cmd+shift+r", Command: replaceCommand},
		{Key: "cmd+e", Command: recentFilesCommand},
	},
	PresetVSCodeWindows: {
		{Key: "ctrl+shift+o", Command: quickOpenCommand},
		{Key: "ctrl+shift+r", Command: replaceCommand},
		{Key: "ctrl+e", Command: recentFilesCommand},
	},
}

func DefaultContext() when.Context {
	return when.Context{
		"textInputFocus":                      true,
		"editorTextFocus":                     true,
		"editorFocus":                         true,
		"editorIsOpen":                        true,
		"editorHasSelection":                  true,
		"editorHasMultipleSelections":         true,
		"editorHasCompletionItemProvider":     true,
		"foldingEnabled":                      true,
		"hasSearchResult":                     true,
		"canNavigateBack":                     true,
		"canNavigateForward":                  true,
		"isMac":                               true,
		"isMacNative":                         true,
		"openFolderWorkspaceSupport":          true,
		"workbench.panel.markers.view.active": true,
		"viewContainer.workbench.view.explorer.enabled": true,
		"workbench.scm.active":                          true,
		"quickDiffDecorationCount":                      someQuickDiffChanges,
		"editorReadonly":                                false,
		"editorTabMovesFocus":                           false,
		"editorHasDefinitionProvider":                   false,
		"editorHasReferenceProvider":                    false,
		"editorHasImplementationProvider":               false,
		"editorHasTypeDefinitionProvider":               false,
		"referenceSearchVisible":                        false,
		"inReferenceSearchEditor":                       false,
		"isInEmbeddedEditor":                            false,
		"config.editor.stablePeek":                      false,
		"findInputFocussed":                             false,
		"replaceInputFocussed":                          false,
		"findWidgetVisible":                             false,
		"suggestWidgetVisible":                          false,
		"isComposing":                                   false,
		"isWeb":                                         false,
		"isWindows":                                     false,
		"isLinux":                                       false,
		"textCompareEditorActive":                       false,
		"isInDiffEditor":                                false,
		"inSearchEditor":                                false,
		"multipleEditorGroups":                          false,
		"isSessionsWindow":                              false,
		"accessibilityHelpIsShown":                      false,
		"accessibleViewIsShown":                         false,
		"accessibilityModeEnabled":                      false,
		"activeEditorIsPinned":                          false,
		"layoutMode":                                    false,
		"notebookEditorFocused":                         false,
		"inQuickOpen":                                   false,
		"sideBarFocus":                                  false,
		"panelFocus":                                    false,
		"listFocus":                                     false,
		"inputFocus":                                    false,
		"terminalFocus":                                 false,
		"explorerViewletFocus":                          false,
		"filesExplorerFocus":                            false,
		"notificationCenterVisible":                     false,
		"sideBarVisible":                                true,
		"editorLangId":                                  "",
		"workbenchState":                                "folder",
		"emptyWorkspaceSupport":                         true,
	}
}

func Defaults(preset string) []Binding {
	table, ok := presetTables[preset]
	if !ok {
		preset, table = PresetVSCode, presetTables[PresetVSCode]
	}
	var out []Binding
	var taken []Chord
	for _, e := range jetBrainsDefaults[preset] {
		b := mustBinding(e)
		out, taken = append(out, b), append(taken, b.Chord)
	}
	for _, e := range table {
		if chord, err := ParseChord(e.Key); err == nil && slices.ContainsFunc(taken, chord.Equal) {
			continue
		}
		entry := Entry{Key: e.Key, Command: e.Command, When: e.When}
		if e.Args != "" {
			entry.Args = json.RawMessage(e.Args)
		}
		if b, err := newBinding(entry); err == nil {
			out = append(out, b)
		}
	}
	for _, e := range appDefaults[preset] {
		out = append(out, mustBinding(e))
	}
	return out
}

func mustBinding(e Entry) Binding {
	b, err := newBinding(e)
	if err != nil {
		panic("keymap: default " + e.Key + ": " + err.Error())
	}
	return b
}
