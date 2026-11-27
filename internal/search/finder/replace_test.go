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
	f.dialog.OpenReplace(seed)
	f.settle()
}

func (f *fixture) has(role ui.Role, name string) bool {
	_, ok := f.tt.AccessByRole(role, name)
	return ok
}

func TestToggleReplaceShowsTheReplaceFieldOnTheTextTab(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.open(TabText, "needle")
	if f.has(ui.RoleTextField, replaceLabel) || f.has(ui.RoleButton, replaceAllLabel) {
		t.Fatal("the replace field shows before Toggle Replace")
	}
	f.click(toggleReplaceLabel)
	if !f.dialog.Snapshot().ShowReplace || !f.has(ui.RoleTextField, replaceLabel) || !f.has(ui.RoleButton, replaceAllLabel) || !f.has(ui.RoleDialog, titleReplace) {
		t.Fatal("Toggle Replace did not show the replace field, Replace All and the Replace in Files title")
	}
	for _, name := range []string{"Preserve Case", maskLabel, scopeSelectLabel} {
		if _, ok := f.tt.Access(name); !ok {
			t.Errorf("the Text tab has no %q", name)
		}
	}
	f.click(toggleReplaceLabel)
	if f.dialog.Snapshot().ShowReplace || f.has(ui.RoleTextField, replaceLabel) || !f.has(ui.RoleDialog, labelText) {
		t.Fatal("Toggle Replace did not hide the replace field")
	}
}

func TestReplaceFieldKeepsItsTextAcrossTabSwitches(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	f.dialog.st.With, f.dialog.st.PreserveCase = "pin", true
	f.settle()
	f.click("Files")
	if f.has(ui.RoleTextField, replaceLabel) || f.has(ui.RoleButton, toggleReplaceLabel) {
		t.Fatal("the Files tab shows replace controls")
	}
	f.click("Text")
	if !f.has(ui.RoleTextField, replaceLabel) || f.dialog.st.With != "pin" || !f.dialog.st.PreserveCase {
		t.Fatalf("replace field lost its state: with %q, preserve case %v", f.dialog.st.With, f.dialog.st.PreserveCase)
	}
	if st := f.dialog.Snapshot(); st.With != "pin" || !st.ShowReplace {
		t.Fatalf("snapshot %+v does not keep the replace field", st)
	}
}

func TestReplaceInFilesOpensTheTextTabWithTheFocusedReplaceField(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	if f.dialog.Tab() != TabText || !f.dialog.Snapshot().ShowReplace || f.dialog.Query() != "needle" {
		t.Fatalf("tab %v replace %v query %q", f.dialog.Tab(), f.dialog.Snapshot().ShowReplace, f.dialog.Query())
	}
	f.type_("pin")
	if f.dialog.st.With != "pin" || f.dialog.Query() != "needle" {
		t.Fatalf("typing went to the wrong field: with %q query %q", f.dialog.st.With, f.dialog.Query())
	}
}

func TestMatchRowsShowTheReplacementAndRowActions(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	f.dialog.st.With = "pin"
	f.settle()
	if !f.tt.HasText("pin") {
		t.Fatal("the match row shows no replacement")
	}
	for _, name := range []string{replaceLabel, replaceFileLabel} {
		if !f.has(ui.RoleButton, name) {
			t.Errorf("the selected match row has no %q button", name)
		}
	}
	f.click(toggleReplaceLabel)
	if f.has(ui.RoleButton, replaceFileLabel) {
		t.Fatal("row actions show without the replace field")
	}
}

func TestFileMaskAndDirectoryScope(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.open(TabText, "needle")
	if f.dialog.text.total != 4 || len(f.dialog.text.files) != 3 {
		t.Fatalf("matches %d in %d files; want 4 in 3 (the binary file is skipped)", f.dialog.text.total, len(f.dialog.text.files))
	}
	f.dialog.st.Mask = "*.go"
	f.settle()
	if f.dialog.text.total != 1 || f.dialog.text.files[0].Rel != "c.go" {
		t.Fatalf("mask *.go: %d matches", f.dialog.text.total)
	}
	f.dialog.st.Mask = ""
	f.dialog.st.Scope, f.dialog.st.Directory = ScopeDirectory, "b"
	f.settle()
	if f.dialog.text.total != 1 || f.dialog.text.files[0].Rel != "b/b.txt" {
		t.Fatalf("directory b: %d matches", f.dialog.text.total)
	}
	f.dialog.st.Directory = ""
	f.settle()
	if f.dialog.text.total != 0 {
		t.Fatal("an empty directory still searched the project")
	}
}

func TestReplaceAllAsksForConfirmationThenEditsFilesAndBuffers(t *testing.T) {
	f := newFixture(t, replaceProject)
	open := f.path("a.txt")
	f.host.buffers[open] = newBuffer(open, "needle one\nneedle two\n")
	f.openReplace("needle")
	f.dialog.st.With = "pin"
	f.settle()
	f.clickLast(ui.RoleButton, replaceAllLabel)
	if !f.dialog.confirm {
		t.Fatal("Replace All did not ask for a confirmation")
	}
	if data, _ := os.ReadFile(f.path("b/b.txt")); !strings.Contains(string(data), "needle") {
		t.Fatal("a file changed before the confirmation")
	}
	f.clickLast(ui.RoleButton, cancelLabel)
	if f.dialog.confirm || !f.dialog.IsOpen() || f.host.buffers[open].String() != "needle one\nneedle two\n" {
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
	f.dialog.st.With = "pin"
	f.settle()
	f.clickLast(ui.RoleButton, replaceLabel)
	if got, _ := os.ReadFile(f.path("a.txt")); string(got) != "pin one\nneedle two\n" {
		t.Fatalf("a.txt = %q; want only the first match replaced", got)
	}
	if f.dialog.text.total != 3 {
		t.Fatalf("%d matches left, want 3", f.dialog.text.total)
	}
}

func TestReplaceAllInFileLeavesOtherFilesAlone(t *testing.T) {
	f := newFixture(t, replaceProject)
	f.openReplace("needle")
	f.dialog.st.With = "pin"
	f.settle()
	f.clickLast(ui.RoleButton, replaceFileLabel)
	if got, _ := os.ReadFile(f.path("a.txt")); string(got) != "pin one\npin two\n" {
		t.Fatalf("a.txt = %q; want both matches replaced", got)
	}
	if got, _ := os.ReadFile(f.path("b/b.txt")); string(got) != "another needle\n" {
		t.Fatalf("b/b.txt = %q; want it unchanged", got)
	}
}

func TestReplaceRegexAndPreserveCase(t *testing.T) {
	f := newFixture(t, map[string]string{"x.txt": "Hello hello HELLO\n"})
	f.openReplace("hello")
	f.dialog.st.With, f.dialog.st.PreserveCase = "world", true
	f.settle()
	f.clickLast(ui.RoleButton, replaceAllLabel)
	f.clickLast(ui.RoleButton, replaceAllLabel)
	if got, _ := os.ReadFile(f.path("x.txt")); string(got) != "World world WORLD\n" {
		t.Fatalf("preserve case: %q", got)
	}
	f.openReplace("(w)orld")
	f.dialog.st.Regex, f.dialog.st.PreserveCase, f.dialog.st.MatchCase, f.dialog.st.With = true, false, false, "[$1]"
	f.settle()
	f.clickLast(ui.RoleButton, replaceAllLabel)
	f.clickLast(ui.RoleButton, replaceAllLabel)
	if got, _ := os.ReadFile(f.path("x.txt")); string(got) != "[W] [w] [W]\n" {
		t.Fatalf("regex groups: %q", got)
	}
}
