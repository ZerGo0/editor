package filetree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	testViewW = 320
	testViewH = 400
)

type calls struct {
	opened  []string
	preview []bool
	created []string
	renamed []string
	pasted  []string
}

func newTestTree(t *testing.T, files ...string) (*Tree, *ui.Tester, *calls, string) {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree := New(root, nil)
	got := &calls{}
	h := Handlers{
		Open: func(p string, preview, _ bool) {
			got.opened = append(got.opened, p)
			got.preview = append(got.preview, preview)
		},
		Menu: func(*ui.Menu, string) {},
		Create: func(dir, name string, _ bool) error {
			got.created = append(got.created, filepath.Join(dir, name))
			return nil
		},
		Rename: func(from, name string) error {
			got.renamed = append(got.renamed, from+"->"+name)
			return nil
		},
		Trash: func(string) {},
		Paste: func(paths []string, dir string, _ bool) {
			got.pasted = append(got.pasted, paths...)
			got.pasted = append(got.pasted, dir)
		},
		Move:   func([]string, string) {},
		Notify: func(string) {},
	}
	tt := ui.NewTester(func(c *ui.Context) { tree.View(c, false, h) }, testViewW, testViewH)
	return tree, tt, got, root
}

func TestTreeShowsIgnoredFiles(t *testing.T) {
	tree, tt, _, root := newTestTree(t, "a.txt", "dist/out.js", "debug.log")
	tree.SetIgnored(ParseIgnored([]byte("dist/\x00debug.log\x00")))
	tt.Frame()
	for _, name := range []string{"a.txt", "dist", "debug.log"} {
		if _, ok := tt.AccessByRole(ui.RoleTreeItem, name); !ok {
			t.Fatalf("row %q is not shown", name)
		}
	}
	if !tree.IsIgnored(filepath.Join(root, "dist")) || !tree.IsIgnored(filepath.Join(root, "debug.log")) {
		t.Fatal("ignored entries are not marked")
	}
	if tree.IsIgnored(filepath.Join(root, "a.txt")) {
		t.Fatal("a.txt is marked ignored")
	}
}

func TestTreeClickPreviewsAndEnterPins(t *testing.T) {
	_, tt, got, root := newTestTree(t, "a.txt")
	if err := tt.Click("a.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEnter)
	want := filepath.Join(root, "a.txt")
	if len(got.opened) != 2 || got.opened[0] != want || !got.preview[0] || got.preview[1] {
		t.Fatalf("opened %v preview %v", got.opened, got.preview)
	}
}

func TestTreeRenameInline(t *testing.T) {
	_, tt, got, root := newTestTree(t, "a.txt")
	if err := tt.Click("a.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyF2)
	if _, ok := tt.AccessByRole(ui.RoleTextField, "Rename"); !ok {
		t.Fatal("F2 shows no rename field")
	}
	tt.Type("b")
	tt.Key(0, ui.KeyEnter)
	if want := filepath.Join(root, "a.txt") + "->b.txt"; len(got.renamed) != 1 || got.renamed[0] != want {
		t.Fatalf("renamed %v, want %s", got.renamed, want)
	}
	if _, ok := tt.AccessByRole(ui.RoleTextField, "Rename"); ok {
		t.Fatal("rename field stays after Enter")
	}
}

func TestTreeCreateInlineAndEscape(t *testing.T) {
	tree, tt, got, root := newTestTree(t, "a.txt")
	tree.StartCreate(root, false)
	tt.Frame()
	tt.Type("notes.md")
	tt.Key(0, ui.KeyEnter)
	if want := filepath.Join(root, "notes.md"); len(got.created) != 1 || got.created[0] != want {
		t.Fatalf("created %v, want %s", got.created, want)
	}
	tree.StartCreate(root, true)
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleTextField, "New Folder"); !ok {
		t.Fatal("no New Folder field")
	}
	tt.Type("x")
	tt.Key(0, ui.KeyEscape)
	if tree.Editing() || len(got.created) != 1 {
		t.Fatalf("Escape did not cancel: editing %v, created %v", tree.Editing(), got.created)
	}
}

func TestTreeCopyPaste(t *testing.T) {
	_, tt, got, root := newTestTree(t, "a.txt")
	if err := tt.Click("a.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Key(ui.Cmd, ui.KeyV)
	a := filepath.Join(root, "a.txt")
	if len(got.pasted) != 2 || got.pasted[0] != a || got.pasted[1] != root {
		t.Fatalf("pasted %v", got.pasted)
	}
}
