package keymap_test

import (
	"testing"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo/ui/editor"
)

func label(t *testing.T, key string) string {
	t.Helper()
	chord, err := keymap.ParseChord(key)
	if err != nil {
		t.Fatal(err)
	}
	return chord.Label()
}

func TestLabelsShowContextKeys(t *testing.T) {
	k := keymap.New(keymap.Defaults(keymap.PresetVSCode), nil)
	for command, key := range map[string]string{
		"editor.action.revealDefinition":             "f12",
		"editor.action.peekDefinition":               "alt+f12",
		"editor.action.goToReferences":               "shift+f12",
		"references-view.findReferences":             "shift+alt+f12",
		"editor.action.goToImplementation":           "cmd+f12",
		"workbench.action.toggleMaximizeEditorGroup": "cmd+k cmd+m",
		"keybindings.editor.searchKeybindings":       "cmd+f",
		"keybindings.editor.recordSearchKeys":        "alt+cmd+k",
		"workbench.action.maximizeEditorHideSidebar": "shift+cmd+f12",
		"editor.action.marker.nextInFiles":           "f8",
		"workbench.action.openEditorAtIndex1":        "ctrl+1",
		"workbench.action.showAllEditors":            "cmd+k cmd+p",
		"actions.find":                               "cmd+f",
	} {
		if got, want := k.Label(command), label(t, key); got != want {
			t.Errorf("%s label = %q, want %q", command, got, want)
		}
	}
}

func TestLabelOfSearchEverywhereIsDoubleShift(t *testing.T) {
	k := keymap.New(keymap.Defaults(keymap.PresetVSCode), nil)
	if got := k.Label(keymap.SearchEverywhereCommand); got != "\u21e7\u21e7" {
		t.Errorf("Search Everywhere label = %q", got)
	}
	k = keymap.New(keymap.Defaults(keymap.PresetVSCode), []keymap.Entry{{Key: "cmd+alt+o", Command: keymap.SearchEverywhereCommand}})
	if got, want := k.Label(keymap.SearchEverywhereCommand), label(t, "cmd+alt+o"); got != want {
		t.Errorf("rebound Search Everywhere label = %q, want %q", got, want)
	}
}

func TestLabelHidesKeyTakenByUser(t *testing.T) {
	k := keymap.New(keymap.Defaults(keymap.PresetVSCode), []keymap.Entry{{Key: "f12", Command: "workbench.action.togglePanel"}})
	if got := k.Label("editor.action.revealDefinition"); got != "" {
		t.Errorf("Go to Definition label = %q, want none after the user took F12", got)
	}
}

func TestJetBrainsContextKeys(t *testing.T) {
	k := keymap.New(keymap.Defaults(keymap.PresetVSCode), nil)
	ctx := keymap.DefaultContext()
	if got := lookup(t, k, "alt+f7"); got != "references-view.findReferences" {
		t.Errorf("alt+f7 runs %q", got)
	}
	ctx["isInDiffEditor"], ctx["textCompareEditorActive"] = true, true
	diff := k.In(ctx)
	for key, want := range map[string]string{"f4": "git.openFile2", "f7": "editor.action.accessibleDiffViewer.next", "shift+f7": "editor.action.accessibleDiffViewer.prev", "alt+f5": "workbench.action.compareEditor.nextChange"} {
		chord, _ := keymap.ParseChord(key)
		if b, _ := diff.Lookup(chord); b.Command != want {
			t.Errorf("in a diff %s runs %q, want %s", key, b.Command, want)
		}
	}
	ctx["editorTextFocus"] = false
	chord, _ := keymap.ParseChord("shift+cmd+f12")
	if b, _ := k.In(ctx).Lookup(chord); b.Command != "workbench.action.maximizeEditorHideSidebar" {
		t.Errorf("outside the editor text shift+cmd+f12 runs %q", b.Command)
	}
}

func TestEditorTitlesFollowVSCode(t *testing.T) {
	for cmd, want := range map[editor.Command]string{
		editor.SelectLarger:            "Expand Selection",
		editor.DuplicateLinesDown:      "Copy Line Down",
		editor.SelectNext:              "Add Selection to Next Find Match",
		editor.SplitSelectionIntoLines: "Add Cursors to Line Ends",
		editor.UniqueLines:             "Delete Duplicate Lines",
		editor.MoveToBracket:           "Go to Bracket",
		editor.Uppercase:               "Transform to Uppercase",
		editor.UndoSelection:           "Cursor Undo",
		editor.NextHunk:                "Go to Next Change",
		editor.ToggleSoftWrap:          "Toggle Word Wrap",
	} {
		if got, _ := keymap.EditorTitle(cmd); got != want {
			t.Errorf("%s title = %q, want %q", cmd, got, want)
		}
	}
}
