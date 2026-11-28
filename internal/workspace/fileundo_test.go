package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
)

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func TestUndoAndRedoARename(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.txt": "a\n"})
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	a, b := filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")
	if err := w.renameEntry(a, "b.txt"); err != nil {
		t.Fatal(err)
	}
	ch := w.tree.PopUndo()
	if ch == nil || ch.Label != renameLabel {
		t.Fatalf("the rename recorded %+v", ch)
	}
	w.tree.Settled()
	if _, err := applyFileChange(ch, true); err != nil || !exists(a) || exists(b) {
		t.Fatalf("undo: err %v, a.txt %v, b.txt %v", err, exists(a), exists(b))
	}
	if _, err := applyFileChange(ch, false); err != nil || exists(a) || !exists(b) {
		t.Fatalf("redo: err %v, a.txt %v, b.txt %v", err, exists(a), exists(b))
	}
}

func TestCreateRecordsAnUndoableChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	if err := w.createEntry(root, "dir", true); err != nil {
		t.Fatal(err)
	}
	ch := w.tree.PopUndo()
	if ch == nil || ch.Label != newFolderLabel || len(ch.Created) != 1 || ch.Created[0].Path != filepath.Join(root, "dir") {
		t.Fatalf("New Folder recorded %+v", ch)
	}
}

func TestRestoreFromTheTrash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFiles(t, dir, map[string]string{"trash/a.txt": "a\n", "taken.txt": "x\n"})
	trashed := filepath.Join(dir, "trash", "a.txt")
	if err := restoreEntry(filetree.Trashed{Path: filepath.Join(dir, "taken.txt"), Trash: trashed}); err == nil {
		t.Fatal("restore overwrote an existing file")
	}
	if err := restoreEntry(filetree.Trashed{Path: filepath.Join(dir, "a.txt")}); err != errNotInTrash {
		t.Fatalf("restore without a Trash path: %v", err)
	}
	to := filepath.Join(dir, "back", "a.txt")
	if err := restoreEntry(filetree.Trashed{Path: to, Trash: trashed}); err != nil || !exists(to) || exists(trashed) {
		t.Fatalf("restore: err %v", err)
	}
}
