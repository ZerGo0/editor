package keymap_test

import (
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/gitui"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/keymap/vscode"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func lookup(t *testing.T, k *keymap.Keymap, key string) string {
	t.Helper()
	chord, err := keymap.ParseChord(key)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := k.Lookup(chord)
	return b.Command
}

func TestDefaultsBindGitActionKeys(t *testing.T) {
	for _, preset := range []string{keymap.PresetVSCode, keymap.PresetVSCodeWindows} {
		k := keymap.New(keymap.Defaults(preset), nil)
		for _, a := range gitui.Actions {
			if a.Key == "" {
				continue
			}
			if got := lookup(t, k, a.Key); got != a.ID {
				t.Errorf("%s: %s runs %q, want %s", preset, a.Key, got, a.ID)
			}
		}
	}
}

func TestMacPresetMatchesVSCode(t *testing.T) {
	k := keymap.New(keymap.Defaults(keymap.PresetVSCode), nil)
	for key, want := range map[string]string{
		"ctrl+g":            "workbench.action.gotoLine",
		"cmd+f":             "actions.find",
		"cmd+alt+f":         "editor.action.startFindReplaceAction",
		"cmd+shift+h":       "workbench.action.replaceInFiles",
		"cmd+shift+f":       "workbench.action.findInFiles",
		"cmd+g":             "editor.action.nextMatchFindAction",
		"cmd+shift+g":       "editor.action.previousMatchFindAction",
		"cmd+d":             "editor.action.addSelectionToNextFindMatch",
		"cmd+shift+l":       "editor.action.selectHighlights",
		"cmd+/":             "editor.action.commentLine",
		"shift+alt+a":       "editor.action.blockComment",
		"alt+up":            "editor.action.moveLinesUpAction",
		"alt+down":          "editor.action.moveLinesDownAction",
		"shift+alt+up":      "editor.action.copyLinesUpAction",
		"shift+alt+down":    "editor.action.copyLinesDownAction",
		"cmd+shift+k":       "editor.action.deleteLines",
		"cmd+enter":         "editor.action.insertLineAfter",
		"cmd+shift+enter":   "editor.action.insertLineBefore",
		"ctrl+-":            "workbench.action.navigateBack",
		"ctrl+shift+-":      "workbench.action.navigateForward",
		"browserback":       "workbench.action.navigateBack",
		"browserforward":    "workbench.action.navigateForward",
		"cmd+alt+[":         "editor.fold",
		"cmd+alt+]":         "editor.unfold",
		"cmd+k cmd+0":       "editor.foldAll",
		"cmd+k cmd+j":       "editor.unfoldAll",
		"cmd+k cmd+l":       "editor.toggleFold",
		"cmd+p":             "workbench.action.quickOpen",
		"cmd+shift+p":       "workbench.action.showCommands",
		"cmd+s":             "workbench.action.files.save",
		"cmd+alt+s":         "workbench.action.files.saveAll",
		"cmd+o":             "workbench.action.files.openFolder",
		"cmd+\\":            "workbench.action.splitEditorRight",
		"cmd+k cmd+\\":      "workbench.action.splitEditorDown",
		"cmd+k cmd+left":    "workbench.action.focusPreviousGroup",
		"cmd+k r":           "workbench.action.files.revealActiveFileInFinder",
		"enter":             "editor.newline",
		"escape":            "cancelSelection",
		"ctrl+h":            "deleteLeft",
		"end":               "cursorEnd",
		"cmd+w":             "workbench.action.closeActiveEditor",
		"cmd+k shift+enter": "workbench.action.pinEditor",
		"f12":               "",
		"cmd+k v":           "markdown.showPreviewToSide",
		"cmd+0":             "",
		"cmd+numpad_add":    "workbench.action.zoomIn",
		"cmd+numpad0":       "workbench.action.zoomReset",
	} {
		if got := lookup(t, k, key); got != want {
			t.Errorf("%s runs %q, want %q", key, got, want)
		}
	}
}

func TestWindowsPresetMatchesVSCode(t *testing.T) {
	k := keymap.New(keymap.Defaults(keymap.PresetVSCodeWindows), nil)
	for key, want := range map[string]string{
		"ctrl+h":       "editor.action.startFindReplaceAction",
		"ctrl+p":       "workbench.action.quickOpen",
		"ctrl+shift+p": "workbench.action.showCommands",
		"ctrl+g":       "workbench.action.gotoLine",
		"ctrl+/":       "editor.action.commentLine",
		"ctrl+d":       "editor.action.addSelectionToNextFindMatch",
		"ctrl+s":       "workbench.action.files.save",
		"ctrl+k s":     "workbench.action.files.saveAll",
		"alt+left":     "workbench.action.navigateBack",
		"f3":           "editor.action.nextMatchFindAction",
		"ctrl+shift+h": "workbench.action.replaceInFiles",
		"cmd+p":        "",
	} {
		if got := lookup(t, k, key); got != want {
			t.Errorf("%s runs %q, want %q", key, got, want)
		}
	}
}

func TestPresetTablesParse(t *testing.T) {
	unrepresentable := []string{"capslock"}
	for name, table := range map[string][]vscode.Entry{"macOS": vscode.MacOS, "Windows": vscode.Windows} {
		for _, e := range table {
			if _, err := keymap.ParseChord(e.Key); err != nil {
				if !containsAny(e.Key, unrepresentable) {
					t.Errorf("%s: %s %s: %v", name, e.Key, e.Command, err)
				}
				continue
			}
			k := keymap.New(nil, []keymap.Entry{{Key: e.Key, Command: e.Command, When: e.When, Args: []byte(e.Args)}})
			if len(k.Errors) > 0 {
				t.Errorf("%s: %s %s: %v", name, e.Key, e.Command, k.Errors)
			}
		}
	}
}

func containsAny(s string, parts []string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func TestUnknownPresetFallsBackToVSCode(t *testing.T) {
	if got, want := len(keymap.Defaults("emacs")), len(keymap.Defaults(keymap.PresetVSCode)); got != want {
		t.Errorf("unknown preset has %d bindings, want %d", got, want)
	}
}

func TestEditorBindingsFollowThePreset(t *testing.T) {
	cmd, ctrl, alt, sh := ui.Super, ui.Ctrl, ui.Alt, ui.Shift
	tests := []struct {
		preset string
		mods   ui.Modifiers
		key    ui.Key
		want   editor.Command
	}{
		{keymap.PresetVSCode, ctrl, ui.KeyG, editor.GoToLine},
		{keymap.PresetVSCode, cmd | alt, ui.KeyF, editor.FindReplace},
		{keymap.PresetVSCode, cmd | sh, ui.KeyH, editor.NoCommand},
		{keymap.PresetVSCode, ctrl, ui.KeyH, editor.Backspace},
		{keymap.PresetVSCode, cmd, ui.KeySlash, editor.ToggleComment},
		{keymap.PresetVSCode, 0, ui.KeyEnter, editor.Newline},
		{keymap.PresetVSCode, ctrl, ui.KeyL, editor.NoCommand},
		{keymap.PresetVSCode, cmd, ui.KeyK, editor.NoCommand},
		{keymap.PresetVSCodeWindows, ctrl, ui.KeyH, editor.FindReplace},
		{keymap.PresetVSCodeWindows, ctrl, ui.KeySlash, editor.ToggleComment},
		{keymap.PresetVSCodeWindows, cmd, ui.KeySlash, editor.NoCommand},
		{keymap.PresetVSCodeWindows, ctrl, ui.KeyC, editor.Copy},
	}
	for _, tt := range tests {
		got, found := editor.NoCommand, false
		bindings := keymap.New(keymap.Defaults(tt.preset), nil).EditorBindings()
		for _, b := range bindings {
			if b.Mods == tt.mods && b.Key == tt.key && b.ThenKey == 0 {
				got, found = b.Command, true
			}
		}
		if !found {
			for _, b := range editor.DefaultBindings() {
				if b.Mods == tt.mods && b.Key == tt.key && b.ThenKey == 0 {
					got, found = b.Command, true
				}
			}
		}
		if got != tt.want || tt.want == editor.NoCommand && !found {
			t.Errorf("%s: %v+%v runs %v (found %v), want %v", tt.preset, tt.mods, tt.key, got, found, tt.want)
		}
	}
}
