package host

import (
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
)

type MemoryState struct {
	Project   *session.Memento
	Global    *session.Memento
	Restorers map[string]func(id, title string) (func(c *ui.Context), bool)
}

func (m *MemoryState) ProjectState() *session.Memento {
	if m.Project == nil {
		m.Project = session.NewMemento(nil)
	}
	return m.Project
}

func (m *MemoryState) GlobalState() *session.Memento {
	if m.Global == nil {
		m.Global = session.NewMemento(nil)
	}
	return m.Global
}

func (m *MemoryState) RestoreViews(prefix string, restore func(id, title string) (func(c *ui.Context), bool)) {
	if m.Restorers == nil {
		m.Restorers = map[string]func(id, title string) (func(c *ui.Context), bool){}
	}
	m.Restorers[prefix] = restore
}
