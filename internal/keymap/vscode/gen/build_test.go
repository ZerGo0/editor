package main

import (
	"strings"
	"testing"
)

const (
	macDump = `// Default Keybindings of Visual Studio Code 1.2.3 for macOS
[
	{ "key": "cmd+g", "command": "workbench.action.gotoLine" },
	{ "key": "ctrl+g", "command": "workbench.action.gotoLine" },
	{ "key": "cmd+k z", "command": "workbench.action.toggleZenMode" },
	// a comment
	{ "key": "end", "command": "cursorEnd", "when": "textInputFocus", "args": { "sticky": false } },
	{ "key": "cmd+k cmd+\\", "command": "workbench.action.splitEditorDown" },
	{ "key": "cmd+k cmd+\\", "command": "workbench.action.splitEditorOrthogonal" },
	{ "key": "cmd+k cmd+\\", "command": "workbench.action.splitEditorRight" },
	{ "key": "alt+cmd+s", "command": "saveAll" },
]`
	winDump = `// Default Keybindings of Visual Studio Code 1.2.3 for Windows
[
	{ "key": "ctrl+h", "command": "editor.action.startFindReplaceAction", "when": "editorFocus || editorIsOpen" },
]`
)

func testSupported(t *testing.T) map[string]bool {
	t.Helper()
	supported, err := parseCommands("workbench.action.splitEditorDown\nworkbench.action.splitEditorRight\nworkbench.action.files.saveAll\n")
	if err != nil {
		t.Fatal(err)
	}
	return supported
}

func TestBuildFiltersToSupportedCommands(t *testing.T) {
	tbl, err := build([][]byte{[]byte(macDump), []byte(winDump)}, testSupported(t))
	if err != nil {
		t.Fatal(err)
	}
	if tbl.version != "1.2.3" {
		t.Errorf("version = %q", tbl.version)
	}
	mac := tbl.platforms[0].entries
	var got []string
	for _, e := range mac {
		got = append(got, e.Key+" "+e.Command+" "+e.Args)
	}
	want := []string{
		"ctrl+g workbench.action.gotoLine ",
		"end cursorEnd {\"sticky\":false}",
		"cmd+k cmd+\\ workbench.action.splitEditorOrthogonal ",
		"alt+cmd+s saveAll ",
	}
	if strings.Join(got, "|") != strings.Join(want[1:], "|") && strings.Join(got, "|") != strings.Join(append([]string{"cmd+g workbench.action.gotoLine "}, want...), "|") {
		t.Errorf("macOS entries = %q", got)
	}
	if w := tbl.platforms[1].entries; len(w) != 1 || w[0].When != "editorFocus || editorIsOpen" {
		t.Errorf("Windows entries = %+v", w)
	}
}

func TestBuildErrors(t *testing.T) {
	supported := testSupported(t)
	for name, dumps := range map[string][]string{
		"no version":       {"[]", winDump},
		"version mismatch": {macDump, strings.Replace(winDump, "1.2.3", "9.9.9", 1)},
		"missing command":  {macDump, "// Default Keybindings of Visual Studio Code 1.2.3 for Windows\n[{\"key\": \"a\"}]"},
		"not json":         {macDump, "// Default Keybindings of Visual Studio Code 1.2.3 for Windows\n[{"},
		"unresolved clash": {macDump, "// Default Keybindings of Visual Studio Code 1.2.3 for Windows\n[{\"key\": \"ctrl+1\", \"command\": \"workbench.action.splitEditorDown\"}, {\"key\": \"ctrl+1\", \"command\": \"workbench.action.splitEditorRight\"}]"},
		"wrong dump count": {macDump},
	} {
		raw := make([][]byte, len(dumps))
		for i, d := range dumps {
			raw[i] = []byte(d)
		}
		if _, err := build(raw, supported); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestParseCommandsRejectsDuplicates(t *testing.T) {
	if _, err := parseCommands("a\na\n"); err == nil {
		t.Error("want error for a duplicate command")
	}
}

func TestRenderIsGoSource(t *testing.T) {
	tbl, err := build([][]byte{[]byte(macDump), []byte(winDump)}, testSupported(t))
	if err != nil {
		t.Fatal(err)
	}
	src, err := render(tbl)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Version      = \"1.2.3\"", "var MacOS = []Entry{", "var Windows = []Entry{", `Args: "{\"sticky\":false}"`} {
		if !strings.Contains(string(src), want) {
			t.Errorf("render output lacks %q:\n%s", want, src)
		}
	}
}
