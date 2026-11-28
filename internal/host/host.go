package host

import (
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type Host interface {
	RootDir() string
	Window() *mygo.Window
	Update(fn func())
	OpenAt(path string, line, col int)
	OpenView(id, title string, view func(c *ui.Context))
	CloseView(id string)
	Notify(text string)
	ShowError(title string, err error)
	Ask(title, action, value string, submit func(string) error)
	SaveAllFiles() bool
	Buffer(path string) *editor.Buffer
	ActivePath() string
	ActiveEditor() *editor.State
	Language(path, text string) *editor.Language
	ConfigureEditor(st *editor.State, path string)
	ConfigureDiff(d *editor.DiffState, path string)
	UserDataDir() string
	ProjectState() *session.Memento
	GlobalState() *session.Memento
	RestoreViews(prefix string, restore func(id, title string) (func(c *ui.Context), bool))
}
