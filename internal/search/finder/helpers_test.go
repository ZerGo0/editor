package finder

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/host"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/files"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/ZerGo0/editor/internal/syntax"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testWidth     = 1000
	testHeight    = 900
	settleTries   = 400
	settleWait    = 5 * time.Millisecond
	queueCapacity = 4096
)

type fakeHost struct {
	host.MemoryState
	root    string
	buffers map[string]*editor.Buffer
	active  string
	queue   chan func()
	notes   []string
	errs    []error
}

func (h *fakeHost) RootDir() string                                { return h.root }
func (h *fakeHost) Window() *mygo.Window                           { return nil }
func (h *fakeHost) Update(fn func())                               { h.queue <- fn }
func (h *fakeHost) OpenAt(string, int, int)                        {}
func (h *fakeHost) OpenView(string, string, func(*ui.Context))     {}
func (h *fakeHost) CloseView(string)                               {}
func (h *fakeHost) Notify(text string)                             { h.notes = append(h.notes, text) }
func (h *fakeHost) ShowError(_ string, err error)                  { h.errs = append(h.errs, err) }
func (h *fakeHost) Ask(string, string, string, func(string) error) {}
func (h *fakeHost) SaveAllFiles() bool                             { return true }
func (h *fakeHost) Buffer(path string) *editor.Buffer              { return h.buffers[path] }
func (h *fakeHost) ActivePath() string                             { return h.active }
func (h *fakeHost) ActiveEditor() *editor.State                    { return nil }
func (h *fakeHost) Language(path, text string) *editor.Language    { return syntax.For(path, text) }
func (h *fakeHost) ConfigureEditor(*editor.State, string)          {}
func (h *fakeHost) ConfigureDiff(*editor.DiffState, string)        {}
func (h *fakeHost) UserDataDir() string                            { return "" }

func (h *fakeHost) drain() bool {
	ran := false
	for {
		select {
		case fn := <-h.queue:
			fn()
			ran = true
		default:
			return ran
		}
	}
}

type fixture struct {
	t        *testing.T
	host     *fakeHost
	env      Env
	dialog   *Dialog
	recent   *RecentFiles
	find     *FindWindow
	tt       *ui.Tester
	opened   []Location
	sides    []bool
	ran      []string
	found    []FindResults
	shown    []string
	forgot   []string
	recents  []string
	commands []palette.Item
	used     []palette.Item
	written  []string
	onFrame  func()
}

func newFixture(t *testing.T, project map[string]string) *fixture {
	t.Helper()
	root := t.TempDir()
	testutil.WriteFiles(t, root, project)
	idx := &files.Index{Root: root}
	if err := idx.Refresh(context.Background(), func(string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, host: &fakeHost{root: root, buffers: map[string]*editor.Buffer{}, queue: make(chan func(), queueCapacity)}}
	f.commands = []palette.Item{{ID: "save", Label: "File: Save", Keys: "⌘S"}, {ID: "close", Label: "View: Close Editor"}, {ID: "build", Label: "Edit: Build"}}
	engine := everywhere.New(&everywhere.Env{
		Root:    root,
		Files:   idx,
		Symbols: symbols.NewIndex(),
		Actions: func() []palette.Item { return append(append([]palette.Item(nil), f.used...), f.commands...) },
		Recent:  func() []string { return append([]string(nil), f.recents...) },
	})
	f.env = Env{
		Host:   f.host,
		Engine: engine,
		Files:  idx,
		Open: func(loc Location, o OpenOptions) {
			f.opened = append(f.opened, loc)
			f.sides = append(f.sides, o.Side)
		},
		Commands: func() (recent, other []palette.Item) {
			recent = f.used
			for _, c := range f.commands {
				used := false
				for _, u := range f.used {
					used = used || u.ID == c.ID
				}
				if !used {
					other = append(other, c)
				}
			}
			return recent, other
		},
		RunAction:       func(id string) { f.ran = append(f.ran, id) },
		ForgetRecent:    func(path string) { f.forgot = append(f.forgot, path) },
		ShowFindResults: func(res FindResults) { f.found = append(f.found, res) },
		ToolWindows: func() []ToolWindow {
			return []ToolWindow{
				{Title: "Explorer", Keys: "⇧⌘E", Show: func() { f.shown = append(f.shown, "explorer") }},
				{Title: "Problems", Show: func() { f.shown = append(f.shown, "problems") }},
			}
		},
		BeforeWrite: func(path string) { f.written = append(f.written, path) },
	}
	f.dialog, f.recent, f.find = NewDialog(f.env), NewRecentFiles(f.env), NewFindWindow(f.env)
	f.tt = ui.NewTester(func(c *ui.Context) {
		f.dialog.View(c)
		f.recent.View(c)
		f.find.View(c)
		if f.onFrame != nil {
			f.onFrame()
		}
	}, testWidth, testHeight)
	return f
}

func (f *fixture) path(rel string) string {
	return filepath.Join(f.host.root, filepath.FromSlash(rel))
}

func (f *fixture) settle() {
	f.t.Helper()
	for range settleTries {
		ran := f.host.drain()
		f.tt.Frame()
		if !ran && len(f.host.queue) == 0 && !f.dialog.busy() && !f.dialog.preview.pending {
			return
		}
		time.Sleep(settleWait)
	}
	f.t.Fatal("the search did not settle")
}

func (f *fixture) type_(text string) {
	f.t.Helper()
	f.tt.Type(text)
	f.settle()
}

func (f *fixture) key(mods ui.Modifiers, key ui.Key) {
	f.t.Helper()
	f.tt.Key(mods, key)
	f.settle()
}

func (f *fixture) click(name string) {
	f.t.Helper()
	f.tt.Click(name)
	f.settle()
}

func (f *fixture) labels() []string {
	var out []string
	for _, e := range f.dialog.entries {
		out = append(out, e.label)
	}
	return out
}

func (f *fixture) groups() []string {
	var out []string
	prev := ""
	for _, e := range f.dialog.entries {
		if e.group != "" && e.group != prev {
			out = append(out, e.group)
		}
		prev = e.group
	}
	return out
}

func (f *fixture) open(tab Tab, seed string) {
	f.t.Helper()
	f.dialog.Open(tab, seed)
	f.settle()
}

func newBuffer(path, text string) *editor.Buffer {
	buf := editor.NewBuffer(text)
	buf.SetLanguage(syntax.For(path, text))
	return buf
}

func (f *fixture) writeProjectFile(rel, data string) {
	f.t.Helper()
	testutil.WriteFile(f.t, f.path(rel), data)
}

func (f *fixture) reindex() {
	f.t.Helper()
	if err := f.env.Files.Refresh(context.Background(), func(string) bool { return false }); err != nil {
		f.t.Fatal(err)
	}
}

func newEngineWithSymbols(f *fixture, path string) *everywhere.Search {
	syms := symbols.NewIndex()
	syms.Update(path, "Go", "package server\n\ntype Handler struct{}\n")
	return everywhere.New(&everywhere.Env{Root: f.host.root, Files: f.env.Files, Symbols: syms})
}

func (f *fixture) setQuery(q string) {
	f.t.Helper()
	f.dialog.field.Focus()
	f.tt.Frame()
	f.tt.Key(ui.Cmd, ui.KeyA)
	f.type_(q)
}
