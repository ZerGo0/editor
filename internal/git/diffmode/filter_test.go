package diffmode

import (
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

func filterFixture(t *testing.T) editor.FilePatch {
	t.Helper()
	return parseOne(t, "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,5 +1,6 @@\n head\n-\tfoo\n-bar  \n-\n+    foo\n+baz\n+\n+\n tail\n")
}

func TestFilterPatchIgnoresWhitespaceOnlyLines(t *testing.T) {
	got := FilterPatch(filterFixture(t), git.WhitespaceIgnore)
	want := []string{" head", "     foo", "-bar  ", "+baz", " ", "+", " tail"}
	if len(got.Hunks) != 1 || !slices.Equal(got.Hunks[0].Lines, want) {
		t.Fatalf("lines = %q", got.Hunks[0].Lines)
	}
	if h := got.Hunks[0]; h.OldLines != 5 || h.NewLines != 6 {
		t.Fatalf("counts = %d, %d", h.OldLines, h.NewLines)
	}
}

func TestFilterPatchTrimKeepsIndentChanges(t *testing.T) {
	got := FilterPatch(filterFixture(t), git.WhitespaceTrim)
	if lines := got.Hunks[0].Lines; slices.Contains(lines, " bar") || !slices.Contains(lines, "-\tfoo") {
		t.Fatalf("lines = %q", lines)
	}
}

func TestFilterPatchBlankModeDropsBlankChanges(t *testing.T) {
	got := FilterPatch(filterFixture(t), git.WhitespaceIgnoreBlank)
	want := []string{" head", "     foo", "-bar  ", "+baz", " ", " ", " tail"}
	if !slices.Equal(got.Hunks[0].Lines, want) {
		t.Fatalf("lines = %q", got.Hunks[0].Lines)
	}
}

func TestFilterPatchDropsHunkWithoutChanges(t *testing.T) {
	p := parseOne(t, "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,3 +1,3 @@\n a\n-\tb\n+  b\n c\n")
	if got := FilterPatch(p, git.WhitespaceIgnore); len(got.Hunks) != 0 {
		t.Fatalf("hunks = %+v", got.Hunks)
	}
	if got := FilterPatch(p, git.WhitespaceNone); len(got.Hunks) != 1 {
		t.Fatalf("none mode hunks = %+v", got.Hunks)
	}
}
