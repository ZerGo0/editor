package vscodeimport

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo/ui"
)

const (
	testViewW = 800
	testViewH = 600
)

var testNow = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

func knownCommands(id string) bool {
	switch id {
	case "workbench.action.gotoLine", "workbench.action.quickOpen", "workbench.action.showCommands",
		"editor.action.startFindReplaceAction", "editor.action.commentLine", "cursorUp":
		return true
	}
	return false
}

const fixture = `// VS Code keybindings
[
	{ "key": "ctrl+h", "command": "editor.action.startFindReplaceAction" },
	{ "key": "ctrl+g", "command": "-workbench.action.gotoLine" },
	{ "key": "cmd+g", "command": "workbench.action.gotoLine", "when": "editorTextFocus && !editorReadonly" },
	{ "key": "ctrl+e", "command": "workbench.action.quickOpen", "args": ">" },
	{ "key": "ctrl+shift+e", "command": "workbench.action.quickOpen", "args": "?" },
	{ "key": "f18", "command": "editor.action.commentLine" },
	{ "key": "ctrl+t", "command": "workbench.action.terminal.new" },
	{ "key": "alt+k", "command": "cursorUp", "when": "terminalFocus || vim.active" },
	{ "key": "ctrl+h", "command": "editor.action.startFindReplaceAction" },
	{ "key": "ctrl+u", "command": "-some.extension.command" },
	{ "key": "", "command": "editor.action.commentLine" },
]
`

func writeSource(t *testing.T, home, keybindings, settings string) Source {
	t.Helper()
	src := Sources(home)[0]
	if err := os.MkdirAll(src.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src.Keybindings(), []byte(keybindings), 0o644); err != nil {
		t.Fatal(err)
	}
	if settings != "" {
		if err := os.WriteFile(src.Settings(), []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return src
}

func TestConvertMapsAndSkips(t *testing.T) {
	entries, err := keymap.ParseEntries([]byte(fixture), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	ours := []keymap.Entry{{Key: "Ctrl+E", Command: "workbench.action.quickOpen", Args: json.RawMessage(` ">" `)}}
	p := Convert(entries, ours, knownCommands)
	var added []string
	for _, e := range p.Add {
		added = append(added, e.Key+" "+e.Command)
	}
	want := []string{
		"ctrl+h editor.action.startFindReplaceAction",
		"ctrl+g -workbench.action.gotoLine",
		"cmd+g workbench.action.gotoLine",
		"alt+k cursorUp",
	}
	if strings.Join(added, "|") != strings.Join(want, "|") {
		t.Errorf("added %q, want %q", added, want)
	}
	if p.Duplicates != 2 {
		t.Errorf("duplicates = %d, want 2", p.Duplicates)
	}
	if len(p.Inactive) != 1 || p.Inactive[0].Command != "cursorUp" {
		t.Errorf("inactive = %v, want the alt+k cursorUp entry", p.Inactive)
	}
	reasons := map[string]string{}
	for _, s := range p.Skipped {
		reasons[s.Entry.Key] = s.Reason
	}
	for key, part := range map[string]string{
		"ctrl+shift+e": "prefix",
		"f18":          "unsupported key",
		"ctrl+t":       "unsupported command",
		"ctrl+u":       "unsupported command",
		"":             "no key",
	} {
		if !strings.Contains(reasons[key], part) {
			t.Errorf("skip reason for %q = %q, want it to mention %q", key, reasons[key], part)
		}
	}
}

func TestAppendKeepsComments(t *testing.T) {
	entries := []keymap.Entry{{Key: "ctrl+h", Command: "editor.action.startFindReplaceAction", When: "a && b"}}
	tests := []struct{ name, in string }{
		{"missing", ""},
		{"comment only", "// mine\n"},
		{"empty array", "[]"},
		{"empty with comment", "// mine\n[\n\t// keep\n]\n"},
		{"entries", "// mine\n[\n\t{ \"key\": \"cmd+1\", \"command\": \"x\" } // trailing\n]\n"},
		{"trailing comma", "[\n\t{ \"key\": \"cmd+1\", \"command\": \"x\" },\n\t/* block ] */\n]"},
	}
	for _, tt := range tests {
		out, err := Append([]byte(tt.in), entries)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		got, err := keymap.ParseEntries(out, tt.name)
		if err != nil {
			t.Errorf("%s: result does not parse: %v\n%s", tt.name, err, out)
			continue
		}
		if last := got[len(got)-1]; last.Key != "ctrl+h" || last.When != "a && b" {
			t.Errorf("%s: last entry = %+v\n%s", tt.name, last, out)
		}
		for _, keep := range []string{"// mine", "// keep", "// trailing", "/* block ] */"} {
			if strings.Contains(tt.in, keep) && !strings.Contains(string(out), keep) {
				t.Errorf("%s: lost %q\n%s", tt.name, keep, out)
			}
		}
		if !strings.Contains(string(out), importComment) {
			t.Errorf("%s: no import comment\n%s", tt.name, out)
		}
	}
	if _, err := Append([]byte(`{"key": "x"}`), entries); err == nil {
		t.Error("Append into an object, want an error")
	}
}

func TestImportMergesWithBackup(t *testing.T) {
	home := t.TempDir()
	src := writeSource(t, home, fixture, `{ // settings
		"keyboard.dispatch": "keyCode",
	}`)
	dst := filepath.Join(t.TempDir(), "keybindings.json")
	mine := "// my bindings\n[\n\t{ \"key\": \"cmd+1\", \"command\": \"workbench.action.quickOpen\" }\n]\n"
	if err := os.WriteFile(dst, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Import(src, dst, knownCommands, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 11 || len(r.Plan.Add) != 5 || len(r.Plan.Skipped) != 5 {
		t.Errorf("total %d, added %d, skipped %d", r.Total, len(r.Plan.Add), len(r.Plan.Skipped))
	}
	if !r.DispatchFound || r.Dispatch != keymap.DispatchKeyCode || r.DispatchError != nil {
		t.Errorf("dispatch = %q %v %v", r.Dispatch, r.DispatchFound, r.DispatchError)
	}
	backup, err := os.ReadFile(r.Backup)
	if err != nil || string(backup) != mine {
		t.Errorf("backup %s = %q, %v", r.Backup, backup, err)
	}
	k := keymap.New(nil, mustRead(t, dst))
	if len(k.Errors) > 0 {
		t.Errorf("merged file errors: %v", k.Errors)
	}
	data, _ := os.ReadFile(dst)
	if !strings.HasPrefix(string(data), "// my bindings") {
		t.Errorf("merged file lost the comment:\n%s", data)
	}
	again, err := Import(src, dst, knownCommands, testNow.Add(time.Second))
	if err != nil || len(again.Plan.Add) != 0 || again.Plan.Duplicates != 6 {
		t.Errorf("second import added %d, duplicates %d, err %v", len(again.Plan.Add), again.Plan.Duplicates, err)
	}
}

func mustRead(t *testing.T, path string) []keymap.Entry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := keymap.ParseEntries(data, path)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestImportErrors(t *testing.T) {
	home := t.TempDir()
	dst := filepath.Join(t.TempDir(), "keybindings.json")
	if _, err := Import(Sources(home)[0], dst, knownCommands, testNow); err == nil {
		t.Error("import without a source file, want an error")
	}
	src := writeSource(t, home, "{ not json", "")
	if _, err := Import(src, dst, knownCommands, testNow); err == nil {
		t.Error("import of a broken file, want an error")
	}
	src = writeSource(t, home, fixture, `{"keyboard.dispatch": "scanCode"}`)
	if err := os.WriteFile(dst, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(src, dst, knownCommands, testNow); err == nil {
		t.Error("import into a broken keybindings.json, want an error")
	}
	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}
	r, err := Import(src, dst, knownCommands, testNow)
	if err != nil || r.Backup != "" || r.DispatchError == nil {
		t.Errorf("import into a new file: backup %q, dispatch error %v, err %v", r.Backup, r.DispatchError, err)
	}
}

func TestFindPrefersStableThenOthers(t *testing.T) {
	home := t.TempDir()
	if _, ok := Find(home); ok {
		t.Fatal("found a source in an empty home")
	}
	codium := Sources(home)[2]
	if err := os.MkdirAll(codium.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codium.Keybindings(), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := Find(home); !ok || got.Product != "VSCodium" {
		t.Errorf("Find = %v %v, want VSCodium", got, ok)
	}
	writeSource(t, home, "[]", "")
	if got, _ := Find(home); got.Product != "Code" {
		t.Errorf("Find = %v, want Code", got)
	}
}

func TestShouldOfferOnce(t *testing.T) {
	home := t.TempDir()
	writeSource(t, home, fixture, "")
	dir := t.TempDir()
	state := filepath.Join(dir, "data", PromptStateFile)
	ours := filepath.Join(dir, "keybindings.json")
	if _, ok := ShouldOffer(home, state, ours); !ok {
		t.Fatal("no offer with a VS Code file and no keybindings.json")
	}
	if err := os.WriteFile(ours, []byte("// nothing yet\n[\n]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := ShouldOffer(home, state, ours); !ok {
		t.Fatal("no offer with an empty keybindings.json")
	}
	if err := os.WriteFile(ours, []byte(`[{"key": "cmd+1", "command": "x"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := ShouldOffer(home, state, ours); ok {
		t.Fatal("offer with a keybindings.json that has entries")
	}
	if err := os.Remove(ours); err != nil {
		t.Fatal(err)
	}
	if err := Remember(state, ChoiceNotNow); err != nil {
		t.Fatal(err)
	}
	if _, ok := ShouldOffer(home, state, ours); ok {
		t.Fatal("offer after the choice is saved")
	}
	if _, ok := ShouldOffer(t.TempDir(), filepath.Join(t.TempDir(), PromptStateFile), ours); ok {
		t.Fatal("offer without a VS Code file")
	}
}

func TestSummaryStates(t *testing.T) {
	s := &Summary{Loading: true}
	tt := ui.NewTester(s.View, testViewW, testViewH)
	if !tt.HasText("Importing VS Code keybindings...") {
		t.Fatal("no loading state")
	}
	retried := false
	s.Loading, s.Err, s.Retry = false, errors.New("read keybindings.json: denied"), func() { retried = true }
	tt.Frame()
	if !tt.HasText("Cannot import VS Code keybindings") || !tt.HasText("read keybindings.json: denied") {
		t.Fatal("no error state")
	}
	if err := tt.Click("Try Again"); err != nil || !retried {
		t.Fatalf("Try Again: %v, retried %v", err, retried)
	}
	s.Err = nil
	s.Report = Report{Source: Source{Product: "Code"}}
	tt.Frame()
	if !tt.HasText("The VS Code keybindings.json has no entries.") {
		t.Fatal("no empty state")
	}
	opened := false
	s.OpenKeybindings = func() { opened = true }
	s.Report = Report{
		Source:        Source{Product: "Code"},
		Total:         3,
		Plan:          Plan{Add: []keymap.Entry{{Key: "alt+k", Command: "cursorUp", When: "vim.active"}}, Inactive: []keymap.Entry{{Key: "alt+k", Command: "cursorUp", When: "vim.active"}}, Skipped: []Skipped{{Entry: keymap.Entry{Key: "f18", Command: "x"}, Reason: "unsupported key f18"}}, Duplicates: 1},
		Backup:        "/tmp/keybindings.json.backup",
		Dispatch:      keymap.DispatchKeyCode,
		DispatchFound: true,
	}
	tt.Frame()
	for _, text := range []string{
		"Imported 1 of 3 keybindings from Code. Skipped 1.",
		"1 were already in your keybindings.json.",
		"Backup of your old keybindings.json: /tmp/keybindings.json.backup",
		"Set keyboard.dispatch to \"keyCode\".",
		"Imported but inactive",
		"Not imported",
		"unsupported key f18",
	} {
		if !tt.HasText(text) {
			t.Errorf("summary does not show %q", text)
		}
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, "Open Keyboard Shortcuts (JSON)"); !ok {
		t.Fatal("no button to open keybindings.json")
	}
	if err := tt.Click("Open Keyboard Shortcuts (JSON)"); err != nil || !opened {
		t.Fatalf("open: %v, opened %v", err, opened)
	}
}
