package diffmode

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

const alignedOld = "package p\n\nfunc f() {\n\tx := 1\n\ty := 2\n\treturn x + y\n}\n\nfunc g() {\n\treturn 0  \n}\n"
const alignedNew = "package p\n\nfunc f() {\n    x := 1\n    y := 3\n    return x + y\n}\n\nfunc g() {\n\treturn 0\n}\n"

func parseOne(t *testing.T, patch string) editor.FilePatch {
	t.Helper()
	files, err := editor.ParsePatch(patch)
	if err != nil || len(files) != 1 {
		t.Fatalf("parse: %v, %d files", err, len(files))
	}
	return files[0]
}

func TestAlignPatchMakesNewFromAligned(t *testing.T) {
	patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -3,5 +3,5 @@\n func f() {\n \tx := 1\n-\ty := 2\n+    y := 3\n \treturn x + y\n }\n"
	p := parseOne(t, patch)
	aligned, old, err := AlignPatch(p, alignedOld, alignedNew)
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := p.Apply(alignedOld); err == nil && raw == alignedNew {
		t.Fatal("the raw patch already makes the new file")
	}
	got, err := aligned.Apply(old)
	if err != nil || got != alignedNew {
		t.Fatalf("apply = %q, %v", got, err)
	}
	d := &editor.DiffState{}
	if err := d.SetPatchFiles(aligned, old, alignedNew); err != nil {
		t.Fatal(err)
	}
	hunks := d.Hunks()
	if len(hunks) != 1 || hunks[0].OldStart != 4 || hunks[0].OldEnd != 5 || hunks[0].NewStart != 4 || hunks[0].NewEnd != 5 {
		t.Fatalf("hunks = %+v", hunks)
	}
	if !strings.Contains(old, "    x := 1") || !strings.Contains(old, "\ty := 2") {
		t.Fatalf("aligned old = %q", old)
	}
}

func TestAlignPatchWithoutHunksEqualsNew(t *testing.T) {
	aligned, old, err := AlignPatch(editor.FilePatch{}, "a\n  b\n", "a\nb\n")
	if err != nil || old != "a\nb\n" || len(aligned.Hunks) != 0 {
		t.Fatalf("old = %q, %+v, %v", old, aligned, err)
	}
	d := &editor.DiffState{}
	if err := d.SetPatchFiles(aligned, old, "a\nb\n"); err != nil {
		t.Fatal(err)
	}
	if len(d.Hunks()) != 0 {
		t.Fatalf("hunks = %+v", d.Hunks())
	}
}

func TestAlignPatchNumbersTheOldSideAfterBlankLines(t *testing.T) {
	oldText := "a\nb\n\nc\nd\ne\nf\ng\nh\ni\nj\nk\n"
	newText := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nK\n"
	patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -9,4 +8,4 @@\n h\n i\n j\n-k\n+K\n"
	aligned, old, err := AlignPatch(parseOne(t, patch), oldText, newText)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(old, "\n\n") {
		t.Fatalf("blank line kept in %q", old)
	}
	got, err := aligned.Apply(old)
	if err != nil || got != newText {
		t.Fatalf("apply = %q, %v", got, err)
	}
}

func TestAlignPatchRejectsMismatch(t *testing.T) {
	patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,2 +1,2 @@\n-one\n+uno\n two\n"
	if _, _, err := AlignPatch(parseOne(t, patch), "other\ntwo\n", "uno\ntwo\n"); err == nil {
		t.Fatal("a patch of other texts was accepted")
	}
	if _, _, err := AlignPatch(editor.FilePatch{Binary: true}, "", ""); err == nil {
		t.Fatal("a binary patch was accepted")
	}
}
