package keymap_test

import (
	"encoding/json"
	"testing"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/keymap/vscode"
	"github.com/egoist/mygo/ui/editor"
)

func TestEveryEditorCommandHasOneID(t *testing.T) {
	seen := map[string]editor.Command{}
	for cmd := editor.NoCommand + 1; cmd.String() != "unknown"; cmd++ {
		if cmd == editor.GoToLine || cmd == editor.ToggleSoftWrap {
			continue
		}
		id := keymap.EditorCommandID(cmd)
		if other, dup := seen[id]; dup {
			t.Errorf("%s and %s share the ID %s", cmd, other, id)
		}
		seen[id] = cmd
		if got, ok := keymap.EditorCommand(id); !ok || got != cmd {
			t.Errorf("EditorCommand(%s) = %v %v, want %v", id, got, ok, cmd)
		}
		if got, ok := keymap.EditorCommand("editor." + cmd.String()); !ok || got != cmd {
			t.Errorf("legacy editor.%s = %v %v, want %v", cmd, got, ok, cmd)
		}
	}
}

func TestVSCodeIDsAreCanonical(t *testing.T) {
	for id, want := range map[string]string{
		"editor.goToLine":                "workbench.action.gotoLine",
		"editor.findReplace":             "editor.action.startFindReplaceAction",
		"editor.toggleComment":           "editor.action.commentLine",
		"cursorWordRight":                "cursorWordEndRight",
		"removeSecondaryCursors":         "cancelSelection",
		"workbench.action.files.saveAll": "workbench.action.files.saveAll",
		"saveAll":                        "workbench.action.files.saveAll",
		"workbench.action.splitEditor":   "workbench.action.splitEditorRight",
		"revealFileInOS":                 "workbench.action.files.revealActiveFileInFinder",
		"workbench.action.navigateBack":  "workbench.action.navigateBack",
		"some.extension.command":         "some.extension.command",
	} {
		if got := keymap.CanonicalID(id); got != want {
			t.Errorf("CanonicalID(%s) = %s, want %s", id, got, want)
		}
	}
	for _, c := range vscode.EditorCommands {
		if got := keymap.EditorCommandID(c.Command); got != c.ID {
			t.Errorf("EditorCommandID(%s) = %s, want %s", c.Command, got, c.ID)
		}
	}
}

func TestResolveCommandArgs(t *testing.T) {
	tests := []struct {
		id, args, want string
	}{
		{"workbench.action.quickOpen", "", "workbench.action.quickOpen"},
		{"workbench.action.quickOpen", "null", "workbench.action.quickOpen"},
		{"workbench.action.quickOpen", `">"`, "workbench.action.showCommands"},
		{"workbench.action.quickOpen", `":"`, "workbench.action.gotoLine"},
		{"workbench.action.quickOpen", `"@"`, "workbench.action.gotoSymbol"},
		{"cursorMove", `{"to":"up","by":"wrappedLine"}`, "cursorUp"},
		{"cursorMove", `{"to":"left","select":true}`, "cursorLeftSelect"},
		{"cursorMove", `{"to":"nextBlankLine","by":"wrappedLine","select":true}`, "editor.selectParagraphDown"},
		{"cursorEnd", `{"sticky":false}`, "cursorEnd"},
	}
	for _, tt := range tests {
		got, err := keymap.ResolveCommand(tt.id, json.RawMessage(tt.args))
		if err != nil || got != tt.want {
			t.Errorf("ResolveCommand(%s, %s) = %q, %v, want %q", tt.id, tt.args, got, err, tt.want)
		}
	}
}

func TestResolveCommandRejectsUnsupportedArgs(t *testing.T) {
	for _, tt := range []struct{ id, args string }{
		{"workbench.action.quickOpen", `"?"`},
		{"workbench.action.quickOpen", `42`},
		{"cursorMove", `{"to":"viewPortTop"}`},
		{"cursorMove", `{"to":"down","value":5}`},
		{"cursorMove", `{"to":"left","by":"word"}`},
		{"cursorMove", `[`},
		{"type", `{"text":"x"}`},
	} {
		if got, err := keymap.ResolveCommand(tt.id, json.RawMessage(tt.args)); err == nil {
			t.Errorf("ResolveCommand(%s, %s) = %q, want an error", tt.id, tt.args, got)
		}
	}
}
