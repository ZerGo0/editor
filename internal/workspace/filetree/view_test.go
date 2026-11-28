package filetree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	testViewW     = 320
	testViewH     = 400
	invalidSuffix = "!"
)

var errInvalidName = errors.New("That name is not allowed.")

type calls struct {
	opened  []string
	preview []bool
	created []string
	renamed []string
	pasted  []string
	trashed []string
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
		Trash: func(paths []string) { got.trashed = append(got.trashed, paths...) },
		Paste: func(paths []string, dir string, _ bool) {
			got.pasted = append(got.pasted, paths...)
			got.pasted = append(got.pasted, dir)
		},
		Move:       func([]string, string) {},
		Error:      func(string, error) {},
		OpenFolder: func() {},
		Close:      func() {},
		Validate: func(_, name string) error {
			if strings.HasSuffix(name, invalidSuffix) {
				return errInvalidName
			}
			return nil
		},
	}
	tt := ui.NewTester(func(c *ui.Context) { tree.View(c, h) }, testViewW, testViewH)
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
	if !tree.IsIgnored(filepath.Join(root, "dist")) || !tree.IsIgnored(filepath.Join(root, "dist", "cache")) || !tree.IsIgnored(filepath.Join(root, "debug.log")) {
		t.Fatal("ignored entries are not marked")
	}
	if tree.IsIgnored(filepath.Join(root, "a.txt")) || tree.IsIgnored(root) {
		t.Fatal("a.txt or the root is marked ignored")
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

func TestTreeDoubleClickOpensPermanently(t *testing.T) {
	_, tt, got, _ := newTestTree(t, "a.txt")
	row, ok := tt.AccessByRole(ui.RoleTreeItem, "a.txt")
	if !ok {
		t.Fatal("no row for a.txt")
	}
	x, y := row.Bounds.X+row.Bounds.W/2, row.Bounds.Y+row.Bounds.H/2
	tt.ClickAt(x, y)
	tt.ClickAt(x, y)
	if len(got.preview) == 0 || got.preview[len(got.preview)-1] {
		t.Fatalf("double click opened with preview %v", got.preview)
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
	tree.StartCreate(root, false, false)
	tt.Frame()
	tt.Type("notes.md")
	tt.Key(0, ui.KeyEnter)
	if want := filepath.Join(root, "notes.md"); len(got.created) != 1 || got.created[0] != want {
		t.Fatalf("created %v, want %s", got.created, want)
	}
	tree.StartCreate(root, true, false)
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

func TestTreeCompactsSingleChildFolders(t *testing.T) {
	tree, tt, _, root := newTestTree(t, "src/app/main/x.go", "src/b.go")
	tree.state.Open.Add(filepath.Join(root, "src"))
	tree.state.Generation++
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleTreeItem, "app/main"); !ok {
		t.Fatal("the single-child folder chain is not one row")
	}
}

func TestTreeGitDecorations(t *testing.T) {
	tree, _, _, root := newTestTree(t, "a/b.txt", "c.txt")
	tree.SetGit([]git.FileStatus{{Path: "a/b.txt", Index: '.', WorkTree: 'M'}, {Path: "c.txt", Index: '?', WorkTree: '?'}})
	tree.isDir[filepath.Join(root, "a")] = true
	if s := tree.gitStatus(filepath.Join(root, "a", "b.txt")); s != token.GitModified {
		t.Fatalf("file status = %v", s)
	}
	if s := tree.gitStatus(filepath.Join(root, "a")); s != token.GitModified {
		t.Fatalf("folder status = %v", s)
	}
	if s := tree.gitStatus(filepath.Join(root, "c.txt")); s != token.GitUntracked {
		t.Fatalf("untracked status = %v", s)
	}
}

func TestTreeTwistieOnlyForFolders(t *testing.T) {
	_, tt, _, _ := newTestTree(t, "a.txt", "src/main.go", "docs/guide/intro.md")
	tt.Frame()
	for name, branch := range map[string]bool{"a.txt": false, "src": true, "docs/guide": true} {
		row, ok := tt.AccessByRole(ui.RoleTreeItem, name)
		if !ok {
			t.Fatalf("row %q is not shown", name)
		}
		if row.Expandable != branch {
			t.Errorf("row %q expandable = %v, want %v", name, row.Expandable, branch)
		}
	}
}

func TestTreeValidatesNameWhileTyping(t *testing.T) {
	tree, tt, got, root := newTestTree(t, "a.txt")
	tree.StartCreate(root, false, false)
	tt.Frame()
	tt.Type("b" + invalidSuffix)
	tt.Frame()
	if !tt.HasText(errInvalidName.Error()) {
		t.Fatalf("no inline error in %q", tt.Texts())
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if tt.HasText(errInvalidName.Error()) {
		t.Fatal("the error stays after the name became valid")
	}
	tt.Key(0, ui.KeyEnter)
	if want := filepath.Join(root, "b"); len(got.created) != 1 || got.created[0] != want {
		t.Fatalf("created %v, want %s", got.created, want)
	}
}

func TestTreeMissingRootShowsRecovery(t *testing.T) {
	tree, tt, _, root := newTestTree(t)
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	tree.InvalidateAll()
	tt.Frame()
	tt.Frame()
	name := filepath.Base(root)
	for _, s := range []string{"The folder " + name + " is no longer available", openFolderLabel, closeWindowLabel} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
}
