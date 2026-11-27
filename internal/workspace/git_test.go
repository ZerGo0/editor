package workspace

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
)

const (
	gitWait       = 5 * time.Second
	gitPoll       = 2 * time.Millisecond
	trackedFile   = "a.go"
	trackedOther  = "b.go"
	trackedSource = "package a\n\nfunc A() {}\n"
)

func newCommittedRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if !git.Available() {
		t.Skip("git is not installed")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testutil.WriteFiles(t, root, files)
	testutil.RunGit(t, root, "init", "--quiet")
	testutil.RunGit(t, root, "add", "-A")
	testutil.RunGit(t, root, "-c", "user.name=Ada", "-c", "user.email=ada@example.com", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "first")
	return root
}

func TestOpenFileShowsCodeAtItsSettledX(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		gitReady bool
	}{
		{"git ready", true},
		{"git loading", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := newCommittedRepo(t, map[string]string{trackedFile: trackedSource, trackedOther: trackedSource})
			w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
			w.attachGit()
			tt := ui.NewTester(w.View, testWideW, testViewH)
			var xs []float32
			frame := func() {
				w.runPendingUpdates()
				tt.Frame()
				if r, ok := tt.TextCaret(); ok {
					xs = append(xs, r.X)
				}
			}
			baseLoaded := func(name string) {
				deadline := time.Now().Add(gitWait)
				for w.git.Base(filepath.Join(root, name)) == nil {
					if time.Now().After(deadline) {
						t.Fatalf("timed out waiting for the Git base of %s", name)
					}
					frame()
					time.Sleep(gitPoll)
				}
				frame()
			}
			frame()
			if tc.gitReady {
				baseLoaded(trackedOther)
			}
			openTab(t, w, filepath.Join(root, trackedFile))
			xs = nil
			baseLoaded(trackedFile)
			if len(xs) == 0 {
				t.Fatal("no frame showed the code")
			}
			settled := xs[len(xs)-1]
			for i, x := range xs {
				if x != settled {
					t.Fatalf("frame %d shows the code at x %v, settled at %v: %v", i, x, settled, xs)
				}
			}
		})
	}
}
