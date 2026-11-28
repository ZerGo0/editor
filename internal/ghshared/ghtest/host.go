package ghtest

import (
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	Timeout      = 5 * time.Second
	drainQuiet   = 20 * time.Millisecond
	waitInterval = 5 * time.Millisecond
	updateBuffer = 1024
)

type AskCall struct {
	Title, Action, Value string
	Submit               func(string) error
}

type FakeHost struct {
	host.MemoryState
	Root     string
	UserData string
	Path     string
	State    *editor.State
	Updates  chan func()
	Views    map[string]func(*ui.Context)
	Titles   map[string]string
	Opened   []string
	Closed   []string
	Notices  []string
	Errs     []string
	Asks     []AskCall
}

func NewFakeHost(root, userData string) *FakeHost {
	return &FakeHost{Root: root, UserData: userData, Updates: make(chan func(), updateBuffer), Views: map[string]func(*ui.Context){}, Titles: map[string]string{}}
}

func (h *FakeHost) RootDir() string                          { return h.Root }
func (h *FakeHost) Window() *mygo.Window                     { return nil }
func (h *FakeHost) Update(fn func())                         { h.Updates <- fn }
func (h *FakeHost) OpenAt(string, int, int)                  {}
func (h *FakeHost) CloseView(id string)                      { h.Closed = append(h.Closed, id) }
func (h *FakeHost) Notify(text string)                       { h.Notices = append(h.Notices, text) }
func (h *FakeHost) SaveAllFiles() bool                       { return true }
func (h *FakeHost) Buffer(string) *editor.Buffer             { return nil }
func (h *FakeHost) ActivePath() string                       { return h.Path }
func (h *FakeHost) ActiveEditor() *editor.State              { return h.State }
func (h *FakeHost) UserDataDir() string                      { return h.UserData }
func (h *FakeHost) Language(string, string) *editor.Language { return nil }
func (h *FakeHost) ConfigureEditor(*editor.State, string)    {}
func (h *FakeHost) ConfigureDiff(*editor.DiffState, string)  {}

func (h *FakeHost) OpenView(id, title string, view func(*ui.Context)) {
	h.Opened = append(h.Opened, id)
	h.Views[id] = view
	h.Titles[id] = title
}

func (h *FakeHost) ShowError(title string, err error) {
	h.Errs = append(h.Errs, title+": "+err.Error())
}

func (h *FakeHost) Ask(title, action, value string, submit func(string) error) {
	h.Asks = append(h.Asks, AskCall{title, action, value, submit})
}

func (h *FakeHost) Pump(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.After(Timeout)
	for !done() {
		select {
		case fn := <-h.Updates:
			fn()
		case <-deadline:
			t.Fatal("timed out waiting for the background work")
		}
	}
}

func (h *FakeHost) Drain() {
	for {
		select {
		case fn := <-h.Updates:
			fn()
		case <-time.After(drainQuiet):
			return
		}
	}
}

func (h *FakeHost) WaitFor(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(Timeout)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for a condition")
		}
		select {
		case fn := <-h.Updates:
			fn()
		case <-time.After(waitInterval):
		}
	}
}

func (h *FakeHost) LastError() string {
	if len(h.Errs) == 0 {
		return ""
	}
	return h.Errs[len(h.Errs)-1]
}

func (h *FakeHost) LastNotice() string {
	if len(h.Notices) == 0 {
		return ""
	}
	return h.Notices[len(h.Notices)-1]
}

func Start[C any](t *testing.T, h *FakeHost, s *ghshared.Session[C]) {
	t.Helper()
	t.Cleanup(s.Close)
	h.Pump(t, func() bool { return s.Phase != ghshared.PhaseLoading })
}
