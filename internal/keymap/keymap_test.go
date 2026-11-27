package keymap_test

import (
	"encoding/json"
	"testing"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/keymap/when"
)

func chord(t *testing.T, s string) keymap.Chord {
	t.Helper()
	c, err := keymap.ParseChord(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func defaultsOf(t *testing.T, entries ...keymap.Entry) []keymap.Binding {
	t.Helper()
	k := keymap.New(nil, entries)
	if len(k.Errors) > 0 {
		t.Fatal(k.Errors)
	}
	out := k.Bindings()
	for i := range out {
		out[i].User = false
	}
	return out
}

func TestUserBindingsWin(t *testing.T) {
	defaults := defaultsOf(t, keymap.Entry{Key: "cmd+p", Command: "workbench.action.quickOpen"})
	k := keymap.New(defaults, []keymap.Entry{{Key: "cmd+p", Command: "workbench.action.showCommands"}})
	if got := lookup(t, k, "cmd+p"); got != "workbench.action.showCommands" {
		t.Errorf("cmd+p runs %q, want showCommands", got)
	}
	if got := k.For("workbench.action.quickOpen"); len(got) != 0 {
		t.Errorf("quickOpen keys = %v, want none", got)
	}
}

func TestLegacyIDsResolveInUserEntries(t *testing.T) {
	k := keymap.New(nil, []keymap.Entry{{Key: "cmd+l", Command: "editor.goToLine"}})
	if got := lookup(t, k, "cmd+l"); got != "workbench.action.gotoLine" {
		t.Errorf("cmd+l runs %q, want workbench.action.gotoLine", got)
	}
	if got := k.For("editor.goToLine"); len(got) != 1 {
		t.Errorf("For(legacy) = %v, want one chord", got)
	}
}

func TestRemovalsOnlyTouchDefaults(t *testing.T) {
	defaults := defaultsOf(t,
		keymap.Entry{Key: "ctrl+g", Command: "workbench.action.gotoLine"},
		keymap.Entry{Key: "cmd+l", Command: "workbench.action.gotoLine", When: "editorTextFocus"},
		keymap.Entry{Key: "cmd+f", Command: "actions.find"},
	)
	tests := []struct {
		name    string
		user    []keymap.Entry
		removed []string
		kept    []string
	}{
		{"by command", []keymap.Entry{{Command: "-workbench.action.gotoLine"}}, []string{"ctrl+g", "cmd+l"}, []string{"cmd+f"}},
		{"by legacy command", []keymap.Entry{{Command: "-editor.goToLine"}}, []string{"ctrl+g", "cmd+l"}, []string{"cmd+f"}},
		{"by key", []keymap.Entry{{Key: "ctrl+g", Command: "-workbench.action.gotoLine"}}, []string{"ctrl+g"}, []string{"cmd+l", "cmd+f"}},
		{"by when", []keymap.Entry{{Command: "-workbench.action.gotoLine", When: "editorTextFocus "}}, []string{"cmd+l"}, []string{"ctrl+g"}},
		{"other command keeps the key", []keymap.Entry{{Key: "cmd+f", Command: "-workbench.action.gotoLine"}}, nil, []string{"ctrl+g", "cmd+f"}},
		{"user binding survives", []keymap.Entry{{Key: "cmd+j", Command: "workbench.action.gotoLine"}, {Command: "-workbench.action.gotoLine"}}, []string{"ctrl+g"}, []string{"cmd+j"}},
	}
	for _, tt := range tests {
		k := keymap.New(defaults, tt.user)
		if len(k.Errors) > 0 {
			t.Errorf("%s: errors %v", tt.name, k.Errors)
		}
		for _, key := range tt.removed {
			if got := lookup(t, k, key); got != "" {
				t.Errorf("%s: %s runs %q, want nothing", tt.name, key, got)
			}
		}
		for _, key := range tt.kept {
			if got := lookup(t, k, key); got == "" {
				t.Errorf("%s: %s runs nothing", tt.name, key)
			}
		}
	}
}

func TestWhenClausesGateBindings(t *testing.T) {
	k := keymap.New(nil, []keymap.Entry{
		{Key: "cmd+e", Command: "a.always"},
		{Key: "cmd+e", Command: "b.never", When: "terminalFocus"},
		{Key: "cmd+u", Command: "c.unknown", When: "someExtensionContext"},
		{Key: "cmd+i", Command: "d.mac", When: "isMac && !isWeb"},
	})
	if got := lookup(t, k, "cmd+e"); got != "a.always" {
		t.Errorf("cmd+e runs %q, want a.always", got)
	}
	if got := lookup(t, k, "cmd+u"); got != "" {
		t.Errorf("cmd+u with an unknown context key runs %q, want nothing", got)
	}
	if got := lookup(t, k, "cmd+i"); got != "d.mac" {
		t.Errorf("cmd+i runs %q, want d.mac", got)
	}
	b, ok := k.Resolve(chord(t, "cmd+e"), when.Context{"terminalFocus": true})
	if !ok || b.Command != "b.never" {
		t.Errorf("Resolve with terminalFocus = %q %v, want b.never", b.Command, ok)
	}
}

func TestBadUserEntriesAreReported(t *testing.T) {
	k := keymap.New(nil, []keymap.Entry{
		{Key: "cmd+p"},
		{Key: "hyper+p", Command: "x"},
		{Key: "cmd+p", Command: "x", When: "a &&"},
		{Key: "cmd+p", Command: "cursorMove", Args: json.RawMessage(`{"to":"viewPortTop"}`)},
		{Key: "cmd+j", Command: "y"},
	})
	if len(k.Errors) != 4 {
		t.Errorf("errors = %v, want 4", k.Errors)
	}
	if got := lookup(t, k, "cmd+j"); got != "y" {
		t.Errorf("cmd+j runs %q, want y", got)
	}
}

func TestChordsBuildPrefixes(t *testing.T) {
	k := keymap.New(nil, []keymap.Entry{
		{Key: "cmd+k cmd+c", Command: "editor.action.addCommentLine"},
		{Key: "cmd+k z", Command: "workbench.action.toggleZenMode"},
		{Key: "browserback", Command: "workbench.action.navigateBack"},
	})
	if got := k.Prefixes(); len(got) != 1 || got[0] != chord(t, "cmd+k")[0] {
		t.Errorf("prefixes = %v, want cmd+k", got)
	}
	if got := k.Completions(chord(t, "cmd+k")[0]); len(got) != 2 {
		t.Errorf("completions = %v, want 2", got)
	}
	if got := lookup(t, k, "browserback"); got != "workbench.action.navigateBack" {
		t.Errorf("browserback runs %q", got)
	}
	if _, ok := k.MenuStroke("workbench.action.navigateBack"); ok {
		t.Error("browserback has a menu key equivalent, want none")
	}
}

func TestParseEntries(t *testing.T) {
	data := []byte(`// comment
[
	{"key": "ctrl+g", "command": "-workbench.action.gotoLine"},
	{"key": "cmd+g", "command": "workbench.action.gotoLine", "when": "editorTextFocus",},
]`)
	entries, err := keymap.ParseEntries(data, "keybindings.json")
	if err != nil || len(entries) != 2 || entries[1].When != "editorTextFocus" {
		t.Fatalf("ParseEntries = %v, %v", entries, err)
	}
	if _, err := keymap.ParseEntries([]byte(`{"key": 1}`), "bad.json"); err == nil {
		t.Error("ParseEntries accepted an object, want an error")
	}
}

func TestActiveFollowsTheContextOutsideTheEditor(t *testing.T) {
	k := keymap.New(nil, []keymap.Entry{
		{Key: "f20", Command: "workbench.action.findInFiles"},
		{Key: "f19", Command: "workbench.action.quickOpen", When: "editorTextFocus"},
		{Key: "f19", Command: "workbench.action.showCommands", When: "!editorTextFocus && !isInEmbeddedEditor"},
		{Key: "cmd+k cmd+x", Command: "workbench.action.closeAllEditors", When: "sideBarFocus"},
	})
	outside := k.In(when.Context{"editorTextFocus": false, "sideBarFocus": true})
	inside := k.In(when.Context{"editorTextFocus": true, "sideBarFocus": false})
	for _, tc := range []struct {
		name   string
		active *keymap.Active
		key    string
		want   string
	}{
		{"unconditional outside", outside, "f20", "workbench.action.findInFiles"},
		{"unconditional inside", inside, "f20", "workbench.action.findInFiles"},
		{"negated outside, unknown key is unset", outside, "f19", "workbench.action.showCommands"},
		{"editor only inside", inside, "f19", "workbench.action.quickOpen"},
	} {
		b, ok := tc.active.Lookup(chord(t, tc.key))
		if !ok || b.Command != tc.want {
			t.Errorf("%s: %s runs %q, want %q", tc.name, tc.key, b.Command, tc.want)
		}
	}
	if got := outside.Prefixes(); len(got) != 1 || got[0] != chord(t, "cmd+k")[0] {
		t.Errorf("outside prefixes = %v, want cmd+k", got)
	}
	if got := inside.Prefixes(); len(got) != 0 {
		t.Errorf("inside prefixes = %v, want none", got)
	}
}
