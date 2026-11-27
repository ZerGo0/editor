package multidiff

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	wrapRepeats = 300
	wrapFiles   = 3
	wrapSettle  = 6
)

func longPatchFor(t testing.TB, path string) *editor.FilePatch {
	t.Helper()
	long := strings.Repeat("word ", wrapRepeats)
	text := fmt.Sprintf("diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ -1,2 +1,2 @@\n-old %s\n+new %s\n same\n same\n", path, path, path, path, long, long)
	fps, err := editor.ParsePatch(text)
	if err != nil || len(fps) != 1 {
		t.Fatalf("patch: %v %d", err, len(fps))
	}
	return &fps[0]
}

func TestStackedDiffRowsGrowWhenWrappingTurnsOn(t *testing.T) {
	t.Parallel()
	for name, layout := range map[string]editor.DiffLayout{"split": editor.DiffSplit, "unified": editor.DiffUnified} {
		entries := testEntries(t, wrapFiles)
		for i := range entries {
			entries[i].Patch = longPatchFor(t, entries[i].Path)
		}
		wrap := false
		v := New()
		v.SetEntries(entries)
		o := Options{Label: "Changes", Set: testSet, Ctx: context.Background(), Post: func(f func()) { f() }, Viewed: NewMemory(),
			Configure: func(d *editor.DiffState, _ string) { d.Wrap, d.Layout = wrap, layout }}
		tt := ui.NewTester(func(c *ui.Context) { v.Render(c, o) }, testWidth, testHeight)
		for range wrapSettle {
			tt.Frame()
		}
		second := entries[1].Path
		before, ok := tt.AccessByRole(ui.RoleGroup, second)
		if !ok {
			t.Fatalf("%s: no header for %s: %v", name, second, tt.Texts())
		}
		wrap = true
		for range wrapSettle {
			tt.Frame()
		}
		after, ok := tt.AccessByRole(ui.RoleGroup, second)
		if !ok {
			t.Fatalf("%s: header of %s gone after wrapping", name, second)
		}
		if after.Bounds.Y <= before.Bounds.Y {
			t.Errorf("%s: header of the second file stayed at y %.1f, was %.1f: the first row did not grow", name, after.Bounds.Y, before.Bounds.Y)
		}
		list, _ := tt.AccessByRole(ui.RoleList, "Diffs of Changes")
		tt.Scroll(list.Bounds.X+list.Bounds.W/2, list.Bounds.Y+list.Bounds.H/2, 0, -100)
		for range wrapSettle {
			tt.Frame()
		}
		top := v.Top()
		header, ok := tt.AccessByRole(ui.RoleGroup, top)
		if !ok || header.Bounds.Y > list.Bounds.Y+list.Bounds.H {
			t.Errorf("%s: header of %s did not stay in view while scrolling wrapped rows", name, top)
		}
	}
}
