package filetree

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTreeShiftDownSelectsSeveralAndTrashesThem(t *testing.T) {
	tree, tt, got, root := newTestTree(t, "a.txt", "b.txt", "c.txt")
	if err := tt.Click("a.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Shift, ui.KeyDown)
	tt.Frame()
	want := []string{filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")}
	if paths := tree.SelectedPaths(); !slices.Equal(paths, want) {
		t.Fatalf("selected %v; want %v", paths, want)
	}
	tt.Key(ui.Cmd, ui.KeyBackspace)
	if !slices.Equal(got.trashed, want) {
		t.Fatalf("trashed %v; want %v", got.trashed, want)
	}
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Key(ui.Cmd, ui.KeyV)
	if !slices.Equal(got.pasted, append(want, root)) {
		t.Fatalf("pasted %v", got.pasted)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if paths := tree.SelectedPaths(); len(paths) != 1 {
		t.Fatalf("Escape kept %v selected", paths)
	}
}

func TestTreeCmdASelectsAll(t *testing.T) {
	tree, tt, _, _ := newTestTree(t, "a.txt", "b.txt", "c.txt")
	if err := tt.Click("a.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	if paths := tree.SelectedPaths(); len(paths) != 3 {
		t.Fatalf("Cmd+A selected %v", paths)
	}
}

func TestTreeSelectedPathsSkipNestedEntries(t *testing.T) {
	got := outermost([]string{"/r/src", "/r/src/a.go", "/r/b.go"})
	if want := []string{"/r/src", "/r/b.go"}; !slices.Equal(got, want) {
		t.Fatalf("outermost = %v; want %v", got, want)
	}
}

func TestTreeOpenKeys(t *testing.T) {
	_, tt, got, root := newTestTree(t, "a.txt", "src/b.go")
	if err := tt.Click("a.txt"); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(root, "a.txt")
	tt.Key(ui.Cmd, ui.KeyDown)
	if n := len(got.opened); n == 0 || got.opened[n-1] != a || got.preview[n-1] {
		t.Fatalf("Cmd+Down opened %v preview %v", got.opened, got.preview)
	}
}

func TestTreeFindFiltersRows(t *testing.T) {
	tree, tt, _, _ := newTestTree(t, "alpha.txt", "beta.txt", "src/gamma.go")
	if err := tt.Click("alpha.txt"); err != nil {
		t.Fatal(err)
	}
	tree.StartFind()
	tt.Frame()
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleTextField, findLabel); !ok {
		t.Fatal("Cmd+F shows no filter field")
	}
	tt.Type("bet")
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleTreeItem, "alpha.txt"); ok {
		t.Fatal("the filter keeps alpha.txt")
	}
	if _, ok := tt.AccessByRole(ui.RoleTreeItem, "beta.txt"); !ok {
		t.Fatal("the filter hides beta.txt")
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if p := tree.Selected(); filepath.Base(p) != "beta.txt" {
		t.Fatalf("Down from the filter chose %q; want beta.txt", p)
	}
	tree.StartFind()
	tt.Frame()
	tt.Type("zzz")
	tt.Frame()
	if _, ok := tt.Access(noMatchesTitle); !ok {
		t.Fatal("no empty state for a filter without matches")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tree.Finding() {
		t.Fatal("Escape keeps the filter open")
	}
	if _, ok := tt.AccessByRole(ui.RoleTreeItem, "alpha.txt"); !ok {
		t.Fatal("closing the filter does not show all rows")
	}
}

func TestTreePageDownMovesTheSelection(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("MyGo moves the selection with Page Down itself outside macOS")
	}
	files := make([]string, 0, 40)
	for i := range 40 {
		files = append(files, string(rune('a'+i/26))+string(rune('a'+i%26))+".txt")
	}
	tree, tt, _, _ := newTestTree(t, files...)
	if err := tt.Click("aa.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyPageDown)
	tt.Frame()
	paths := tree.SelectedPaths()
	if tree.row <= 1 || len(paths) != 1 || paths[0] != tree.Selected() {
		t.Fatalf("Page Down chose row %d, selection %v", tree.row, paths)
	}
}
