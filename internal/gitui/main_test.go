package gitui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/host"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const drainTimeout = 5 * time.Second

func TestMain(m *testing.M) {
	cleanup, err := testutil.IsolateGit("gitui-home")
	if err != nil {
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func tempDir(t testing.TB) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTempRepo(t testing.TB) string {
	t.Helper()
	dir := tempDir(t)
	testutil.RunGit(t, dir, "init", "--quiet")
	return dir
}

func commitAll(t testing.TB, dir, msg string) {
	t.Helper()
	testutil.RunGit(t, dir, "add", "-A")
	testutil.RunGit(t, dir, "commit", "--quiet", "-m", msg)
}

type fakeHost struct {
	host.MemoryState
	root    string
	updates chan func()
	mu      sync.Mutex
	notes   []string
	errs    []string
	views   map[string]string
	asked   []string
	answer  string
	buffers map[string]*editor.Buffer
	active  string
	editor  *editor.State
	opened  []string
	lines   []int
	viewID  string
	scmOn   bool
}

func newFakeHost(root string) *fakeHost {
	return &fakeHost{root: root, updates: make(chan func(), 256), views: map[string]string{}, buffers: map[string]*editor.Buffer{}}
}

func (h *fakeHost) RootDir() string      { return h.root }
func (h *fakeHost) Window() *mygo.Window { return nil }
func (h *fakeHost) Update(fn func())     { h.updates <- fn }
func (h *fakeHost) OpenAt(path string, line, _ int) {
	h.opened, h.lines = append(h.opened, path), append(h.lines, line)
}
func (h *fakeHost) ActiveViewID() string           { return h.viewID }
func (h *fakeHost) SourceControlFocused() bool     { return h.scmOn }
func (h *fakeHost) CloseView(id string)            { delete(h.views, id) }
func (h *fakeHost) SaveAllFiles() bool             { return true }
func (h *fakeHost) ActivePath() string             { return h.active }
func (h *fakeHost) ActiveEditor() *editor.State    { return h.editor }
func (h *fakeHost) UserDataDir() string            { return h.root }
func (h *fakeHost) Buffer(p string) *editor.Buffer { return h.buffers[p] }

func (h *fakeHost) OpenView(id, title string, _ func(c *ui.Context)) { h.views[id] = title }

func (h *fakeHost) Language(string, string) *editor.Language { return nil }
func (h *fakeHost) ConfigureEditor(*editor.State, string)    {}
func (h *fakeHost) ConfigureDiff(*editor.DiffState, string)  {}

func (h *fakeHost) Notify(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.notes = append(h.notes, text)
}

func (h *fakeHost) ShowError(title string, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errs = append(h.errs, title+": "+err.Error())
}

func (h *fakeHost) Ask(title, _, _ string, submit func(string) error) {
	h.asked = append(h.asked, title)
	if h.answer != "" {
		_ = submit(h.answer)
	}
}

func (h *fakeHost) drain(t testing.TB, done func() bool) {
	t.Helper()
	deadline := time.After(drainTimeout)
	for !done() {
		select {
		case fn := <-h.updates:
			fn()
		case <-deadline:
			t.Fatalf("condition not met in time; errors: %v", h.errs)
		}
	}
}

func (h *fakeHost) settle() {
	for {
		select {
		case fn := <-h.updates:
			fn()
		case <-time.After(50 * time.Millisecond):
			return
		}
	}
}

func (f *fakeRepo) DiffWorkTree(context.Context, ...string) ([]git.FileDiff, error) { return nil, nil }

func (f *fakeRepo) DiffStaged(context.Context, ...string) ([]git.FileDiff, error) { return nil, nil }

type fakeRepo struct {
	repo
	dir      string
	mu       sync.Mutex
	status   *git.Status
	branches []git.Branch
	commits  []git.Commit
	files    []git.ChangedFile
	tags     []git.Tag
	calls    []string
	commitOK git.CommitOptions
	shows    map[string]string
	remotes  []git.Remote
	versions [3]string
	conflict []git.ConflictFile
	blame    []git.BlameLine
	pushed   git.PushOptions
}

func (r *fakeRepo) record(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, s)
}

func (r *fakeRepo) called(s string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if c == s {
			return true
		}
	}
	return false
}

func (r *fakeRepo) WorkDir() string { return r.dir }
func (r *fakeRepo) MetaDir() string { return filepath.Join(r.dir, ".git") }

func (r *fakeRepo) Status(context.Context) (*git.Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status, nil
}

func (r *fakeRepo) State(context.Context) (git.RepoState, error) { return git.RepoState{}, nil }

func (r *fakeRepo) Branches(context.Context) ([]git.Branch, error) { return r.branches, nil }

func (r *fakeRepo) Remotes(context.Context) ([]git.Remote, error) { return r.remotes, nil }

func (r *fakeRepo) Tags(context.Context) ([]git.Tag, error) { return r.tags, nil }

func (r *fakeRepo) Log(_ context.Context, o git.LogOptions) ([]git.Commit, error) {
	if o.Skip >= len(r.commits) {
		return nil, nil
	}
	return r.commits[o.Skip:], nil
}

func (r *fakeRepo) CommitFiles(context.Context, string) ([]git.ChangedFile, error) {
	return r.files, nil
}

func (r *fakeRepo) BranchesContaining(context.Context, string) ([]string, error) {
	return []string{"main"}, nil
}

func (r *fakeRepo) StageFiles(_ context.Context, paths ...string) error {
	r.record("stage " + strings.Join(paths, ","))
	return nil
}

func (r *fakeRepo) Commit(_ context.Context, o git.CommitOptions) (string, error) {
	r.mu.Lock()
	r.commitOK = o
	r.mu.Unlock()
	r.record("commit")
	return "abcdef1234567890", nil
}

func (r *fakeRepo) CreateBranch(_ context.Context, name, start string, checkout bool) error {
	r.record(fmt.Sprintf("branch %s from %s checkout %t", name, start, checkout))
	return nil
}

func (r *fakeRepo) Checkout(_ context.Context, ref string, _ bool) error {
	r.record("checkout " + ref)
	return nil
}

func newFakeGit(t *testing.T, r *fakeRepo) (*Git, *fakeHost) {
	t.Helper()
	if r.dir == "" {
		r.dir = tempDir(t)
	}
	return newFakeGitOn(t, r, newFakeHost(r.dir))
}

func newFakeGitOn(t *testing.T, r *fakeRepo, h *fakeHost) (*Git, *fakeHost) {
	t.Helper()
	d := deps{
		available: func() bool { return true },
		open:      func(context.Context, string, string) (repo, error) { return r, nil },
		initRepo:  func(context.Context, string) error { r.record("init"); return nil },
		watch:     func(context.Context, string, func()) {},
		ask:       func(*mygo.Window, question) bool { return true },
	}
	g := newGit(h, d)
	t.Cleanup(g.Close)
	h.drain(t, func() bool { return g.ready() && g.snap.status != nil })
	return g, h
}

func (r *fakeRepo) Show(_ context.Context, rev, path string) ([]byte, error) {
	text, ok := r.shows[rev+":"+path]
	if !ok {
		return nil, git.ErrNotFound
	}
	return []byte(text), nil
}

func (r *fakeRepo) MergeVersions(context.Context, string) ([]byte, []byte, []byte, error) {
	return []byte(r.versions[0]), []byte(r.versions[1]), []byte(r.versions[2]), nil
}

func (r *fakeRepo) Conflicts(context.Context) ([]git.ConflictFile, error) { return r.conflict, nil }

func (r *fakeRepo) MarkResolved(_ context.Context, paths ...string) error {
	r.record("resolved " + strings.Join(paths, ","))
	return nil
}

func (r *fakeRepo) Blame(context.Context, string, string, []byte) ([]git.BlameLine, error) {
	return r.blame, nil
}

func (r *fakeRepo) OutgoingCommits(context.Context) ([]git.Commit, error) { return r.commits, nil }

func (r *fakeRepo) IncomingCommits(context.Context) ([]git.Commit, error) {
	return nil, git.ErrNoUpstream
}

func (r *fakeRepo) Push(_ context.Context, o git.PushOptions) error {
	r.mu.Lock()
	r.pushed = o
	r.mu.Unlock()
	r.record("push")
	return nil
}
