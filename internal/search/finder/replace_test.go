package finder

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

var replaceProject = map[string]string{
	"a.txt":     "needle one\nneedle two\n",
	"b/b.txt":   "another needle\n",
	"c.go":      "package c\n// needle\n",
	"notes.bin": "binary\x00needle",
}

func (f *fixture) clickLast(role ui.Role, name string) {
	f.t.Helper()
	var last ui.AccessInfo
	found := false
	for _, a := range f.tt.AccessAll() {
		if a.Role == role && a.Name == name {
			last, found = a, true
		}
	}
	if !found {
		f.t.Fatalf("no %v named %q", role, name)
	}
	f.tt.ClickAt(last.Bounds.X+last.Bounds.W/2, last.Bounds.Y+last.Bounds.H/2)
	f.settle()
}

func (f *fixture) openReplace(seed string) {
	f.t.Helper()
	f.replace.Open(seed)
	f.settle()
}

func TestReplacePopupShowsQueryReplaceRowAndMatches(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	if _, ok := f.tt.AccessByRole(ui.RoleDialog, titleReplace); !ok {
		t.Fatal("the popup has no Replace in Files dialog")
	}
	for _, name := range []string{searchLabel, replaceLabel, toggleReplaceLabel, maskLabel, "Match Case", "Match Whole Word", "Use Regular Expression", "Preserve Case"} {
		if _, ok := f.tt.Access(name); !ok {
			t.Errorf("the popup has no %q", name)
		}
	}
	if f.replace.text.total != 4 || len(f.replace.text.files) != 3 {
		t.Fatalf("matches %d in %d files; want 4 in 3 (the binary file is skipped)", f.replace.text.total, len(f.replace.text.files))
	}
	if row, ok := f.replace.current(); !ok || row.match != 0 || row.file.Rel != "a.txt" {
		t.Fatalf("selected %+v; want the first match of a.txt", row)
	}
	if f.replace.preview.st == nil || f.replace.preview.st.SelectedText() != "needle" {
		t.Fatal("the preview pane does not show the selected match")
	}
}

func TestReplacePopupFileMaskAndDirectoryScope(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	f.replace.st.Mask = "*.go"
	f.settle()
	if f.replace.text.total != 1 || f.replace.text.files[0].Rel != "c.go" {
		t.Fatalf("mask *.go: %d matches", f.replace.text.total)
	}
	f.replace.st.Mask = ""
	f.replace.st.Scope, f.replace.st.Directory = ScopeDirectory, "b"
	f.settle()
	if f.replace.text.total != 1 || f.replace.text.files[0].Rel != "b/b.txt" {
		t.Fatalf("directory b: %d matches", f.replace.text.total)
	}
	f.replace.st.Directory = ""
	f.settle()
	if f.replace.text.total != 0 {
		t.Fatal("an empty directory still searched the project")
	}
}

func TestReplaceAllAsksForConfirmationThenEditsFilesAndBuffers(t *testing.T) {
	f := newFixture(t, replaceProject)
	open := f.path("a.txt")
	f.host.buffers[open] = newBuffer(open, "needle one\nneedle two\n")
	f.openReplace("needle")
	f.replace.st.With = "pin"
	f.settle()
	f.clickLast(ui.RoleButton, replaceAllLabel)
	if !f.replace.confirm {
		t.Fatal("Replace All did not ask for a confirmation")
	}
	if data, _ := os.ReadFile(f.path("b/b.txt")); !strings.Contains(string(data), "needle") {
		t.Fatal("a file changed before the confirmation")
	}
	f.clickLast(ui.RoleButton, cancelLabel)
	if f.replace.confirm || f.host.buffers[open].String() != "needle one\nneedle two\n" {
		t.Fatal("Cancel did not leave everything as it was")
	}
	f.clickLast(ui.RoleButton, replaceAllLabel)
	f.clickLast(ui.RoleButton, replaceAllLabel)
	if got := f.host.buffers[open].String(); got != "pin one\npin two\n" {
		t.Fatalf("open buffer %q", got)
	}
	for rel, want := range map[string]string{"b/b.txt": "another pin\n", "c.go": "package c\n// pin\n", "notes.bin": "binary\x00needle"} {
		if got, _ := os.ReadFile(f.path(rel)); string(got) != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if len(f.host.notes) != 1 || !strings.Contains(f.host.notes[0], "Replaced 4 occurrences in 3 files") {
		t.Fatalf("notes %v", f.host.notes)
	}
	if !slices.Contains(f.written, f.path("c.go")) || slices.Contains(f.written, open) {
		t.Fatalf("before-write hook calls %v; want disk files only", f.written)
	}
}

func TestReplaceSelectedMatchOnly(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	f.replace.st.With = "pin"
	f.settle()
	f.clickLast(ui.RoleButton, replaceLabel)
	if got, _ := os.ReadFile(f.path("a.txt")); string(got) != "pin one\nneedle two\n" {
		t.Fatalf("a.txt = %q; want only the first match replaced", got)
	}
	if f.replace.text.total != 3 {
		t.Fatalf("%d matches left, want 3", f.replace.text.total)
	}
}

func TestReplaceRegexAndPreserveCase(t *testing.T) {
	f := newFixture(t, map[string]string{"x.txt": "Hello hello HELLO\n"})
	f.openReplace("hello")
	f.replace.st.With, f.replace.st.PreserveCase = "world", true
	f.settle()
	f.clickLast(ui.RoleButton, replaceAllLabel)
	f.clickLast(ui.RoleButton, replaceAllLabel)
	if got, _ := os.ReadFile(f.path("x.txt")); string(got) != "World world WORLD\n" {
		t.Fatalf("preserve case: %q", got)
	}
	f.openReplace("(w)orld")
	f.replace.st.Regex, f.replace.st.PreserveCase, f.replace.st.MatchCase, f.replace.st.With = true, false, false, "[$1]"
	f.settle()
	f.clickLast(ui.RoleButton, replaceAllLabel)
	f.clickLast(ui.RoleButton, replaceAllLabel)
	if got, _ := os.ReadFile(f.path("x.txt")); string(got) != "[W] [w] [W]\n" {
		t.Fatalf("regex groups: %q", got)
	}
}

func TestReplacePopupOpensResultsInFindWindowAndEditor(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	f.clickLast(ui.RoleButton, findWindowLabel)
	if f.replace.IsOpen() || len(f.found) != 1 || f.found[0].Total != 4 {
		t.Fatalf("open %v found %+v", f.replace.IsOpen(), f.found)
	}
	f.openReplace("needle")
	f.tt.Key(0, ui.KeyEscape)
	if f.replace.IsOpen() {
		t.Fatal("Escape did not close the popup")
	}
}

func TestReplacePopupRowToggleHidesReplaceFields(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	f.click(toggleReplaceLabel)
	if f.replace.st.ShowReplace {
		t.Fatal("Toggle Replace kept the replace row")
	}
	if _, ok := f.tt.AccessByRole(ui.RoleDialog, titleFind); !ok {
		t.Fatal("the popup is not named Find in Files without the replace row")
	}
	if _, ok := f.tt.AccessByRole(ui.RoleButton, replaceAllLabel); ok {
		t.Fatal("Replace All shows without the replace row")
	}
}
