package workspace

import (
	"github.com/ZerGo0/editor/internal/host"
	"github.com/ZerGo0/editor/internal/layout"
	searchview "github.com/ZerGo0/editor/internal/search/view"
	"github.com/ZerGo0/editor/internal/syntax"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
)

var _ host.Host = (*Workspace)(nil)

func (w *Workspace) RootDir() string {
	return w.Root
}

func (w *Workspace) Window() *mygo.Window {
	return w.win
}

func (w *Workspace) Update(fn func()) {
	w.update(fn)
}

func (w *Workspace) OpenAt(path string, line, col int) {
	pos := editor.Pos{Line: line, Col: col}
	w.openLocation(searchview.Location{Path: path, Start: pos, End: pos, HasRange: line >= 0}, searchview.OpenOptions{Preview: true, Focus: true})
}

func (w *Workspace) CloseView(id string) {
	p := virtualPrefix + id
	for _, g := range w.layout.Groups() {
		for _, t := range g.Tabs {
			if t.Path == p {
				w.removeTabs(g, []*layout.Tab{t})
				return
			}
		}
	}
}

func (w *Workspace) Notify(text string) {
	w.setNotice(text)
}

func (w *Workspace) ShowError(title string, err error) {
	w.showError(title, err)
}

func (w *Workspace) Ask(title, action, value string, submit func(string) error) {
	w.ask(title, action, value, submit)
}

func (w *Workspace) SaveAllFiles() bool {
	return w.save(w.modified())
}

func (w *Workspace) Buffer(path string) *editor.Buffer {
	if d := w.doc(path); d != nil {
		return d.Buffer
	}
	return nil
}

func (w *Workspace) ActiveEditor() *editor.State {
	if t := w.active.ActiveTab(); t != nil {
		return t.Editor
	}
	return nil
}

func (w *Workspace) Language(path, text string) *editor.Language {
	return syntax.For(path, text)
}

func (w *Workspace) UserDataDir() string {
	return w.env.UserData
}
