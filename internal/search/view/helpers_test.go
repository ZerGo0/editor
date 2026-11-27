package searchview

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/files"
	"github.com/ZerGo0/editor/internal/syntax"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type opened struct {
	loc  Location
	opts OpenOptions
}

type fakeHost struct {
	root    string
	mu      sync.Mutex
	queue   []func()
	buffers map[string]*editor.Buffer
	active  string
	notices []string
	errs    []error
}

func (h *fakeHost) ConfigureEditor(*editor.State, string)          {}
func (h *fakeHost) ConfigureDiff(*editor.DiffState, string)        {}
func (h *fakeHost) RootDir() string                                { return h.root }
func (h *fakeHost) Window() *mygo.Window                           { return nil }
func (h *fakeHost) OpenAt(string, int, int)                        {}
func (h *fakeHost) OpenView(string, string, func(*ui.Context))     {}
func (h *fakeHost) CloseView(string)                               {}
func (h *fakeHost) Notify(text string)                             { h.notices = append(h.notices, text) }
func (h *fakeHost) ShowError(_ string, err error)                  { h.errs = append(h.errs, err) }
func (h *fakeHost) Ask(string, string, string, func(string) error) {}
func (h *fakeHost) SaveAllFiles() bool                             { return true }
func (h *fakeHost) Buffer(path string) *editor.Buffer              { return h.buffers[path] }
func (h *fakeHost) ActivePath() string                             { return h.active }
func (h *fakeHost) ActiveEditor() *editor.State                    { return nil }
func (h *fakeHost) Language(path, text string) *editor.Language    { return syntax.For(path, text) }
func (h *fakeHost) UserDataDir() string                            { return h.root }

func (h *fakeHost) Update(fn func()) {
	h.mu.Lock()
	h.queue = append(h.queue, fn)
	h.mu.Unlock()
}

func (h *fakeHost) drain() int {
	h.mu.Lock()
	q := h.queue
	h.queue = nil
	h.mu.Unlock()
	for _, fn := range q {
		fn()
	}
	return len(q)
}

type fixture struct {
	idx      *files.Index
	t        *testing.T
	host     *fakeHost
	view     *View
	tt       *ui.Tester
	opens    []opened
	runs     []string
	confirms []string
	approve  bool
	before   []string
}

func newFixture(t *testing.T, tree map[string]string) *fixture {
	t.Helper()
	root := t.TempDir()
	for name, text := range tree {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx := &files.Index{Root: root}
	if err := idx.Refresh(context.Background(), func(string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	f := &fixture{idx: idx, t: t, host: &fakeHost{root: root, buffers: map[string]*editor.Buffer{}}, approve: true}
	prov := everywhere.New(&everywhere.Env{
		Root:    root,
		Files:   idx,
		Actions: nil,
	})
	f.view = New(Env{
		Host:      f.host,
		Providers: prov,
		Files:     idx,
		OpenFiles: func() []string {
			var out []string
			for p := range f.host.buffers {
				out = append(out, p)
			}
			return out
		},
		Open:        func(loc Location, o OpenOptions) { f.opens = append(f.opens, opened{loc, o}) },
		RunAction:   func(id string) { f.runs = append(f.runs, id) },
		BeforeWrite: func(p string) { f.before = append(f.before, p) },
	})
	f.view.confirm = func(message, _, _ string, done func(bool)) {
		f.confirms = append(f.confirms, message)
		done(f.approve)
	}
	f.tt = ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).FillWidth().Children(func() { f.view.TitleActions(c) })
			f.view.View(c)
		})
	}, 360, 640)
	return f
}

func (f *fixture) path(rel string) string {
	return filepath.Join(f.host.root, filepath.FromSlash(rel))
}

func (f *fixture) settle() {
	f.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	f.tt.Frame()
	for {
		n := f.host.drain()
		f.tt.Frame()
		f.tt.Advance(textDelay)
		if n == 0 && !f.view.busy() && f.view.quickDue.IsZero() && f.view.textDue.IsZero() && !f.view.quickArm && !f.view.textArm {
			return
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("view did not settle: text=%+v", f.view.text.searching)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (f *fixture) search(scope Scope, query string) {
	f.t.Helper()
	f.view.Focus(scope, query)
	f.settle()
}

func (f *fixture) labels() []string {
	var out []string
	for _, r := range f.view.rows {
		out = append(out, r.label)
	}
	return out
}
