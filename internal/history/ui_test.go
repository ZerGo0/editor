package history

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/host"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testWidth   = 900
	testHeight  = 500
	waitTimeout = 5 * time.Second
	waitStep    = 2 * time.Millisecond
	oldText     = "old text\n"
	currentText = "current text\n"
)

type fakeHost struct {
	host.MemoryState
	root    string
	active  string
	mu      sync.Mutex
	queue   []func()
	queued  int
	views   map[string]func(*ui.Context)
	buffers map[string]*editor.Buffer
	notices []string
	errs    []string
	submit  func(string) error
	asked   string
}

func newFakeHost(root string) *fakeHost {
	return &fakeHost{root: root, views: map[string]func(*ui.Context){}, buffers: map[string]*editor.Buffer{}}
}

func (f *fakeHost) RootDir() string                          { return f.root }
func (f *fakeHost) Window() *mygo.Window                     { return nil }
func (f *fakeHost) OpenAt(string, int, int)                  {}
func (f *fakeHost) CloseView(string)                         {}
func (f *fakeHost) SaveAllFiles() bool                       { return true }
func (f *fakeHost) UserDataDir() string                      { return "" }
func (f *fakeHost) ActivePath() string                       { return f.active }
func (f *fakeHost) ActiveEditor() *editor.State              { return nil }
func (f *fakeHost) Language(string, string) *editor.Language { return nil }
func (f *fakeHost) ConfigureEditor(*editor.State, string)    {}
func (f *fakeHost) ConfigureDiff(*editor.DiffState, string)  {}

func (f *fakeHost) Update(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, fn)
	f.queued++
}

func (f *fakeHost) updates() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.queued
}

func (f *fakeHost) OpenView(id, _ string, view func(*ui.Context)) { f.views[id] = view }

func (f *fakeHost) Notify(text string) { f.notices = append(f.notices, text) }

func (f *fakeHost) ShowError(title string, err error) {
	f.errs = append(f.errs, title+": "+err.Error())
}

func (f *fakeHost) Ask(title, _, _ string, submit func(string) error) {
	f.asked, f.submit = title, submit
}

func (f *fakeHost) Buffer(path string) *editor.Buffer { return f.buffers[path] }

func (f *fakeHost) drain() {
	f.mu.Lock()
	q := f.queue
	f.queue = nil
	f.mu.Unlock()
	for _, fn := range q {
		fn()
	}
}

type env struct {
	t     *testing.T
	host  *fakeHost
	store *Store
	ui    *UI
	path  string
	root  string
	tt    *ui.Tester
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	store, _ := openTest(t, Limits{})
	store.now = time.Now
	h := newFakeHost(root)
	e := &env{t: t, host: h, store: store, ui: NewUI(h, store), path: filepath.Join(root, "a.txt"), root: root}
	e.ui.confirm = func(*mygo.Window, string, time.Time) bool { return true }
	e.ui.copyText = func(string) {}
	h.active = e.path
	return e
}

func (e *env) show() {
	e.t.Helper()
	e.ui.ShowHistory(e.path)
	view := e.host.views[ViewPrefix+e.path]
	if view == nil {
		e.t.Fatal("no view opened")
	}
	e.tt = ui.NewTester(view, testWidth, testHeight)
}

func (e *env) waitFor(what string, cond func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		e.host.drain()
		if e.tt != nil {
			e.tt.Frame()
		}
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("timed out waiting for %s; texts %q", what, e.tt.Texts())
		}
		time.Sleep(waitStep)
	}
}

func (e *env) waitLabel(label string) {
	e.t.Helper()
	e.waitFor("version labelled "+label, func() bool {
		vs, _ := e.store.Versions(e.root, e.path)
		for _, v := range vs {
			if v.Label == label {
				return true
			}
		}
		return false
	})
}

func (e *env) waitReady() {
	e.t.Helper()
	e.waitFor("revert enabled", func() bool {
		a, ok := e.tt.AccessByRole(ui.RoleButton, revertLabel)
		return ok && !a.Disabled
	})
}

func (e *env) seed(texts ...string) {
	e.t.Helper()
	for _, s := range texts {
		if err := e.store.Record(e.root, e.path, []byte(s), ""); err != nil {
			e.t.Fatal(err)
		}
	}
}

func TestEmptyState(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFile(e.t, e.path, currentText)
	e.show()
	e.waitFor("empty text", e.showsEmptyHistory)
}

func (e *env) showsEmptyHistory() bool {
	return e.tt.HasText("No history yet") && e.tt.HasText(emptyMessage)
}

func TestLoadingShowsSpinner(t *testing.T) {
	e := newEnv(t)
	e.show()
	if !e.tt.HasText("Loading history...") {
		t.Errorf("no loading state: %q", e.tt.Texts())
	}
}

func TestListsVersionsAndEnablesActions(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText, "middle\n")
	testutil.WriteFile(e.t, e.path, currentText)
	e.show()
	e.waitReady()
	for _, name := range []string{revertLabel, "Copy", "Put Label..."} {
		if _, ok := e.tt.AccessByRole(ui.RoleButton, name); !ok {
			t.Errorf("no %q button", name)
		}
	}
	if !e.tt.HasText("just now  7 B") && !e.tt.HasText("just now  9 B") {
		t.Errorf("version rows missing: %q", e.tt.Texts())
	}
}

func TestSelectingOlderVersionLoadsIt(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText, "middle\n")
	testutil.WriteFile(e.t, e.path, currentText)
	e.show()
	e.waitReady()
	if err := e.tt.Click("just now  9 B"); err != nil {
		t.Fatal(err)
	}
	e.waitFor("older version loaded", func() bool { return e.ui.pages[e.path].oldText == oldText })
}

func TestRevertWritesFileAtomically(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText)
	testutil.WriteFile(e.t, e.path, currentText)
	e.show()
	e.waitReady()
	if err := e.tt.Click(revertLabel); err != nil {
		t.Fatal(err)
	}
	e.waitFor("file reverted", func() bool {
		b, _ := os.ReadFile(e.path)
		return string(b) == oldText
	})
	e.waitLabel(beforeRevertLabel)
	e.waitLabel(revertedLabel)
}

func TestRevertWritesOpenBufferSoUndoWorks(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText)
	buf := editor.NewBuffer(currentText)
	e.host.buffers[e.path] = buf
	e.show()
	e.waitReady()
	if err := e.tt.Click(revertLabel); err != nil {
		t.Fatal(err)
	}
	e.waitFor("buffer reverted", func() bool { return buf.String() == oldText })
	if !buf.Undo() || buf.String() != currentText {
		t.Errorf("undo gave %q", buf.String())
	}
	e.waitLabel(beforeRevertLabel)
}

func TestRevertCancelledChangesNothing(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText)
	testutil.WriteFile(e.t, e.path, currentText)
	asked := make(chan struct{}, 1)
	e.ui.confirm = func(*mygo.Window, string, time.Time) bool { asked <- struct{}{}; return false }
	e.show()
	e.waitReady()
	if err := e.tt.Click(revertLabel); err != nil {
		t.Fatal(err)
	}
	select {
	case <-asked:
	case <-time.After(waitTimeout):
		t.Fatal("no confirmation")
	}
	e.host.drain()
	if b, _ := os.ReadFile(e.path); string(b) != currentText {
		t.Errorf("file changed to %q", b)
	}
}

func TestCopyUsesVersionText(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText)
	testutil.WriteFile(e.t, e.path, currentText)
	var copied string
	e.ui.copyText = func(s string) { copied = s }
	e.show()
	e.waitReady()
	if err := e.tt.Click("Copy"); err != nil {
		t.Fatal(err)
	}
	if copied != oldText {
		t.Errorf("copied %q", copied)
	}
}

func TestPutLabelRecordsCurrentText(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFile(e.t, e.path, currentText)
	if !e.ui.Run(ActionPutLabel) || e.host.submit == nil {
		t.Fatal("no prompt")
	}
	if err := e.host.submit(""); err == nil {
		t.Error("empty label accepted")
	}
	if err := e.host.submit("Before refactor"); err != nil {
		t.Fatal(err)
	}
	e.waitFor("label stored", func() bool {
		vs, _ := e.store.Versions(e.root, e.path)
		return len(vs) == 1 && vs[0].Label == "Before refactor"
	})
}

func TestPutLabelUsesOpenBuffer(t *testing.T) {
	e := newEnv(t)
	e.host.buffers[e.path] = editor.NewBuffer("unsaved\n")
	e.ui.PutLabel(e.path)
	if err := e.host.submit("Draft"); err != nil {
		t.Fatal(err)
	}
	e.waitFor("label stored", func() bool {
		vs, _ := e.store.Versions(e.root, e.path)
		return len(vs) == 1 && vs[0].Size == int64(len("unsaved\n"))
	})
}

func TestRecordGoesThroughStore(t *testing.T) {
	e := newEnv(t)
	content := []byte("saved")
	e.ui.Record(e.path, content, "")
	content[0] = 'X'
	e.waitFor("recorded", func() bool {
		vs, _ := e.store.Versions(e.root, e.path)
		if len(vs) != 1 {
			return false
		}
		got, _ := e.store.Read(vs[0].Hash)
		return string(got) == "saved"
	})
}

func TestRecordReportsLowDiskOnce(t *testing.T) {
	e := newEnv(t)
	e.store.free = func(string) (uint64, error) { return 0, nil }
	e.ui.Record(e.path, []byte("a"), "")
	e.ui.Record(e.path, []byte("b"), "")
	e.waitFor("both records reported", func() bool { return e.host.updates() == 2 })
	e.host.drain()
	if len(e.host.notices) != 1 {
		t.Errorf("notices %q", e.host.notices)
	}
}

func TestRecordIgnoresTooLarge(t *testing.T) {
	e := newEnv(t)
	e.ui.reportAuto(ErrTooLarge)
	if len(e.host.notices) != 0 {
		t.Errorf("notices %q", e.host.notices)
	}
}

func TestErrorStateOffersRetry(t *testing.T) {
	e := newEnv(t)
	x, _ := e.store.index(e.root)
	if err := os.Mkdir(e.store.indexPath(x.key), 0o755); err != nil {
		t.Fatal(err)
	}
	delete(e.store.indexes, x.key)
	e.show()
	e.waitFor("error", func() bool { return e.tt.HasText("Cannot load history") })
	if err := os.Remove(e.store.indexPath(x.key)); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	e.waitFor("empty after retry", e.showsEmptyHistory)
}

func TestMissingFileShowsDeleted(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText)
	e.show()
	e.waitReady()
	if !e.tt.HasText("File deleted") {
		t.Errorf("texts %q", e.tt.Texts())
	}
}

func TestRunActions(t *testing.T) {
	e := newEnv(t)
	if e.ui.Run("nope") {
		t.Error("unknown action ran")
	}
	e.host.active = ""
	if !e.ui.Run(ActionShow) || len(e.host.notices) != 1 {
		t.Errorf("notices %q", e.host.notices)
	}
	e.host.active = e.path
	if !e.ui.Run(ActionShow) || e.host.views[ViewPrefix+e.path] == nil {
		t.Error("history not opened")
	}
	for _, a := range Actions {
		if a.ID == "" || a.Title == "" || a.Category == "" {
			t.Errorf("incomplete action %+v", a)
		}
	}
}

func TestRenamedMovesHistoryAndPage(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText)
	to := filepath.Join(e.root, "b.txt")
	e.ui.ShowHistory(e.path)
	e.ui.Renamed(e.path, to)
	e.waitFor("renamed", func() bool {
		vs, _ := e.store.Versions(e.root, to)
		return len(vs) == 1
	})
	if e.ui.pages[to] == nil || e.ui.pages[e.path] != nil {
		t.Error("page not moved")
	}
}

func TestNoUnnamedButtons(t *testing.T) {
	e := newEnv(t)
	e.seed(oldText)
	testutil.WriteFile(e.t, e.path, currentText)
	e.show()
	e.waitReady()
	for _, a := range e.tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "" {
			t.Errorf("button without name at %v", a.Bounds)
		}
	}
}
