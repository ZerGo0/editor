package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/ZerGo0/editor/internal/workspace/notices"
)

const readOnlyMode = 0o444

func actionLabels(n notices.Notice) []string {
	var out []string
	for _, a := range n.Actions {
		out = append(out, a.Label)
	}
	return out
}

func lastNotice(t *testing.T, w *Workspace) notices.Notice {
	t.Helper()
	list := w.notes.List()
	if len(list) == 0 {
		t.Fatal("no notice")
	}
	return list[0]
}

func TestReadOnlyFileOpensLockedAndSaveKeepsIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write read-only files")
	}
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.txt": "one\n"})
	p := filepath.Join(root, "a.txt")
	if err := os.Chmod(p, readOnlyMode); err != nil {
		t.Fatal(err)
	}
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	tab, e := openTab(t, w, p)
	if !e.doc.Locked || !e.doc.ReadOnly || !tab.Editor.ReadOnly {
		t.Fatalf("locked %v read-only %v editor %v", e.doc.Locked, e.doc.ReadOnly, tab.Editor.ReadOnly)
	}
	e.doc.SetLocked(false)
	e.doc.Buffer.Set("changed\n")
	if w.save([]*document.Document{e.doc}) {
		t.Fatal("save replaced a read-only file")
	}
	if got := string(testutil.ReadFile(t, p)); got != "one\n" {
		t.Fatalf("file = %q", got)
	}
	n := lastNotice(t, w)
	if n.Tone != token.ToneError || n.Title != "Cannot save a.txt" || !slices.Equal(actionLabels(n), []string{saveAsLabel, revealInFinderLabel}) {
		t.Fatalf("notice = %+v", n)
	}
	if !e.doc.ReadOnly || !tab.Editor.ReadOnly {
		t.Fatal("a failed save did not lock the file again")
	}
}

func TestSaveFailedActions(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	d := &document.Document{Path: filepath.Join(t.TempDir(), "a.txt")}
	for _, tc := range []struct {
		err  error
		want []string
	}{
		{&document.NotUTF8Error{Path: d.Path}, []string{revealInFinderLabel}},
		{&os.PathError{Op: "save", Path: d.Path, Err: os.ErrPermission}, []string{saveAsLabel, revealInFinderLabel}},
		{errors.New("disk full"), []string{saveAsLabel, retryLabel}},
	} {
		w.saveFailed(d, tc.err)
		if got := actionLabels(lastNotice(t, w)); !slices.Equal(got, tc.want) {
			t.Errorf("%v: actions %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestDeletedFileIsMarkedAndClearedBySave(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.txt": "one\n"})
	p := filepath.Join(root, "a.txt")
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	_, e := openTab(t, w, p)
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	w.checkFiles(root)
	waitFor(t, w, func() bool { return w.deleted(p) })
	if got := deletedOnDisk([]*document.Document{e.doc}); len(got) != 1 {
		t.Fatalf("deletedOnDisk = %v", got)
	}
	if !w.save([]*document.Document{e.doc}) || w.deleted(p) {
		t.Fatal("save did not create the file again")
	}
	if got := string(testutil.ReadFile(t, p)); got != "one\n" {
		t.Fatalf("file = %q", got)
	}
}

func waitFor(t *testing.T, w *Workspace, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(peekWait)
	for w.runPendingUpdates(); !done(); w.runPendingUpdates() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(peekPoll)
	}
}

func TestToastsHideAndDoNotDisturb(t *testing.T) {
	b := newTransientBed(t)
	b.w.Warn("Heads up", "detail")
	b.settle()
	if !b.w.toastsVisible() || !b.hasText("Heads up") {
		t.Fatal("the warning toast is not shown")
	}
	b.w.HideToasts()
	b.settle()
	if b.w.toastsVisible() || b.hasText("Heads up") {
		t.Fatal("HideToasts left the toast")
	}
	b.w.ToggleDoNotDisturb()
	b.w.Warn("Quiet", "")
	b.settle()
	if b.hasText("Quiet") {
		t.Fatal("Do Not Disturb showed a warning toast")
	}
	b.w.showError("Loud", errors.New("boom"))
	b.settle()
	if !b.hasText("Loud") {
		t.Fatal("Do Not Disturb hid an error toast")
	}
}
