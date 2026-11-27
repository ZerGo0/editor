package vscodeimport

import (
	"encoding/json"
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
	{ "key": "capslock", "command": "editor.action.commentLine" },
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
		"capslock":     "unsupported key",
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

const reportFixture = `[
	{ "key": "ctrl+t", "command": "workbench.action.terminal.new" },
	{ "key": "ctrl+y", "command": "workbench.action.terminal.new" },
	{ "key": "capslock", "command": "cursorUp" },
	{ "key": "ctrl+[Period]", "command": "cursorUp" },
	{ "key": "ctrl+k", "command": "cursorUp", "when": "a &&" },
	{ "key": "ctrl+j", "command": "workbench.action.quickOpen", "args": "?" },
	{ "key": "alt+k", "command": "cursorUp", "when": "vim.active" },
	{ "key": "", "command": "cursorUp" }
]`

func reportOf(t *testing.T) Report {
	t.Helper()
	entries, err := keymap.ParseEntries([]byte(reportFixture), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	return Report{Source: Source{Product: "Code"}, Total: len(entries), Plan: Convert(entries, nil, knownCommands)}
}

func TestSectionsListWhatWasNotImported(t *testing.T) {
	r := reportOf(t)
	if got, want := r.Headline(), "Imported 2 of 8 keybindings from Code. Skipped 6."; got != want {
		t.Errorf("headline = %q, want %q", got, want)
	}
	var titles []string
	rows := map[string][]string{}
	for _, s := range r.Sections() {
		titles = append(titles, s.Title)
		rows[s.Title] = s.Rows
	}
	wantTitles := []string{
		"Imported with unset context keys",
		"Commands this editor does not have",
		"Commands with arguments this editor does not support",
		"Keys this editor cannot read",
		"When clauses this editor cannot read",
		"Entries without a key or a command",
	}
	if strings.Join(titles, "|") != strings.Join(wantTitles, "|") {
		t.Fatalf("sections = %q, want %q", titles, wantTitles)
	}
	if got := rows["Commands this editor does not have"]; len(got) != 1 || got[0] != "workbench.action.terminal.new: 2" {
		t.Errorf("command rows = %q, want one row with a count of 2", got)
	}
	if got := rows["Keys this editor cannot read"]; len(got) != 1 || !strings.Contains(got[0], "capslock") {
		t.Errorf("key rows = %q, want the capslock entry", got)
	}
	if got := rows["Imported with unset context keys"]; len(got) != 1 || !strings.Contains(got[0], "vim.active") {
		t.Errorf("unset rows = %q, want the vim.active entry", got)
	}
}

func TestScanCodeKeysImport(t *testing.T) {
	r := reportOf(t)
	imported := false
	for _, e := range r.Plan.Add {
		imported = imported || e.Key == "ctrl+[Period]"
	}
	if !imported {
		t.Error("ctrl+[Period] was not imported")
	}
}

func TestSummaryShowsTheSections(t *testing.T) {
	opened := false
	s := &Summary{Report: Report{Source: Source{Product: "Code"}}, OpenKeybindings: func() { opened = true }}
	tt := ui.NewTester(s.View, testViewW, testViewH)
	if !tt.HasText("The VS Code keybindings.json has no entries.") {
		t.Fatal("no empty state")
	}
	s.Report = reportOf(t)
	s.Report.Backup = "/tmp/keybindings.json.backup"
	s.Report.Dispatch, s.Report.DispatchFound = keymap.DispatchKeyCode, true
	tt.Frame()
	for _, text := range []string{
		"Imported 2 of 8 keybindings from Code. Skipped 6.",
		"Backup of your old keybindings.json: /tmp/keybindings.json.backup",
		"Set keyboard.dispatch to \"keyCode\".",
		"Commands this editor does not have",
		"workbench.action.terminal.new: 2",
		"capslock cursorUp: unsupported key capslock",
	} {
		if !tt.HasText(text) {
			t.Errorf("summary does not show %q", text)
		}
	}
	if err := tt.Click("Open Keyboard Shortcuts (JSON)"); err != nil || !opened {
		t.Fatalf("open: %v, opened %v", err, opened)
	}
}
