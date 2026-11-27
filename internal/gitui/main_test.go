package gitui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const drainTimeout = 5 * time.Second

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "gitui-home")
	if err != nil {
		os.Exit(1)
	}
	env := map[string]string{
		"HOME":                home,
		"XDG_CONFIG_HOME":     home,
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Ada Lovelace",
		"GIT_AUTHOR_EMAIL":    "ada@example.com",
		"GIT_COMMITTER_NAME":  "Ada Lovelace",
		"GIT_COMMITTER_EMAIL": "ada@example.com",
		"GIT_CONFIG_COUNT":    "3",
		"GIT_CONFIG_KEY_0":    "init.defaultBranch",
		"GIT_CONFIG_VALUE_0":  "main",
		"GIT_CONFIG_KEY_1":    "protocol.file.allow",
		"GIT_CONFIG_VALUE_1":  "always",
		"GIT_CONFIG_KEY_2":    "commit.gpgsign",
		"GIT_CONFIG_VALUE_2":  "false",
	}
	for k, v := range env {
		if err := os.Setenv(k, v); err != nil {
			os.Exit(1)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

func rawGit(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
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
	rawGit(t, dir, "init", "--quiet")
	return dir
}

func writeFile(t testing.TB, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t testing.TB, dir, msg string) {
	t.Helper()
	rawGit(t, dir, "add", "-A")
	rawGit(t, dir, "commit", "--quiet", "-m", msg)
}

type fakeHost struct {
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
}

func newFakeHost(root string) *fakeHost {
	return &fakeHost{root: root, updates: make(chan func(), 256), views: map[string]string{}, buffers: map[string]*editor.Buffer{}}
}

func (h *fakeHost) RootDir() string                { return h.root }
func (h *fakeHost) Window() *mygo.Window           { return nil }
func (h *fakeHost) Update(fn func())               { h.updates <- fn }
func (h *fakeHost) OpenAt(string, int, int)        {}
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

func (r *fakeRepo) Checkout(_ context.Context, ref string, _ bool) error {
	r.record("checkout " + ref)
	return nil
}

func newFakeGit(t *testing.T, r *fakeRepo) (*Git, *fakeHost) {
	t.Helper()
	if r.dir == "" {
		r.dir = tempDir(t)
	}
	h := newFakeHost(r.dir)
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
