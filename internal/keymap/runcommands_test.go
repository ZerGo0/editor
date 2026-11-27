package keymap_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
)

func resolveRun(t *testing.T, args string) []string {
	t.Helper()
	got, err := keymap.ResolveCommand("runCommands", json.RawMessage(args))
	if err != nil {
		t.Fatalf("ResolveCommand(runCommands, %s): %v", args, err)
	}
	ids, ok := command.ParseSequenceID(got)
	if !ok {
		t.Fatalf("ResolveCommand(runCommands, %s) = %q, want a sequence", args, got)
	}
	return ids
}

func TestRunCommandsResolvesItems(t *testing.T) {
	ids := resolveRun(t, `{"commands": [
		"workbench.action.closeSidebar",
		{"command": "cursorMove", "args": {"to": "up", "by": "wrappedLine"}},
		{"command": "saveAll"},
		{"command": "cursorMove", "args": {"to": "viewPortTop"}},
		"workbench.action.terminal.toggleTerminal"
	]}`)
	want := []string{"workbench.action.closeSidebar", "cursorUp", "workbench.action.files.saveAll", "workbench.action.terminal.toggleTerminal"}
	if !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if got := resolveRun(t, `{"commands": []}`); len(got) != 0 {
		t.Errorf("empty list = %v", got)
	}
}

func TestRunCommandItemsReportsWhatFailed(t *testing.T) {
	items, err := keymap.RunCommandItems(json.RawMessage(`{"commands": ["a", {"command": "cursorMove", "args": {"to": "viewPortTop"}}, 5]}`))
	if err != nil || len(items) != 3 {
		t.Fatalf("items = %v, err = %v", items, err)
	}
	if items[0].Err != nil || items[0].ID != "a" || items[0].Resolved != "a" {
		t.Errorf("first = %+v", items[0])
	}
	if items[1].Err == nil || items[1].ID != "cursorMove" {
		t.Errorf("second = %+v, want an argument error", items[1])
	}
	if items[2].Err == nil {
		t.Errorf("third = %+v, want an error for a number", items[2])
	}
}

func TestRunCommandsNeedsCommandsArg(t *testing.T) {
	for _, args := range []string{`{}`, `[]`, `"a"`, `{"commands": "a"}`, `[`} {
		if got, err := keymap.ResolveCommand("runCommands", json.RawMessage(args)); err == nil {
			t.Errorf("ResolveCommand(runCommands, %s) = %q, want an error", args, got)
		}
	}
	if got, err := keymap.ResolveCommand("runCommands", nil); err != nil || got != "runCommands" {
		t.Errorf("without args = %q, %v", got, err)
	}
}

func TestRunCommandsBindingsFollowWhenClauses(t *testing.T) {
	user := []keymap.Entry{
		{Key: "ctrl+e", Command: "runCommands", When: "sideBarVisible", Args: json.RawMessage(`{"commands": ["workbench.action.closeSidebar", "workbench.action.closeAuxiliaryBar"]}`)},
		{Key: "ctrl+e", Command: "runCommands", When: "!sideBarVisible", Args: json.RawMessage(`{"commands": ["workbench.action.toggleSidebarVisibility"]}`)},
	}
	k := keymap.New(nil, user)
	if len(k.Errors) > 0 {
		t.Fatalf("errors: %v", k.Errors)
	}
	chord, err := keymap.ParseChord("ctrl+e")
	if err != nil {
		t.Fatal(err)
	}
	ctx := keymap.DefaultContext()
	b, ok := k.Resolve(chord, ctx)
	if !ok || b.Command != command.SequenceID([]string{"workbench.action.closeSidebar", "workbench.action.closeAuxiliaryBar"}) {
		t.Errorf("visible side bar: %q, %v", b.Command, ok)
	}
	ctx["sideBarVisible"] = false
	b, ok = k.Resolve(chord, ctx)
	if !ok || b.Command != command.SequenceID([]string{"workbench.action.toggleSidebarVisibility"}) {
		t.Errorf("hidden side bar: %q, %v", b.Command, ok)
	}
}

func TestGitCheckoutIsTheBranchPicker(t *testing.T) {
	if got := keymap.CanonicalID("git.checkout"); got != "git.branches" {
		t.Errorf("CanonicalID(git.checkout) = %q, want git.branches", got)
	}
	if got, err := keymap.ResolveCommand("git.checkout", nil); err != nil || strings.HasPrefix(got, "git.checkout") {
		t.Errorf("ResolveCommand = %q, %v", got, err)
	}
}
