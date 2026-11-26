package app

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type commandSource struct {
	app *App
}

func (s commandSource) Lookup(id string) (action.CommandInfo, bool) {
	a := s.app
	cmd := a.commands.Get(id)
	if cmd == nil {
		return action.CommandInfo{}, false
	}
	info := action.CommandInfo{Title: cmd.Title}
	if chords := a.keys.Current.For(id); len(chords) > 0 {
		info.Keys = chords[0].Label()
	}
	if ws := a.drawing; ws != nil {
		info.Disabled = !ws.CommandEnabled(id)
	}
	return info, true
}

func (s commandSource) Run(id string) {
	a := s.app
	var win *mygo.Window
	if ws := a.drawing; ws != nil {
		win = ws.Window()
	}
	a.commands.Run(id, win)
}

func (a *App) projectView(ws *workspace.Workspace) func(c *ui.Context) {
	return func(c *ui.Context) {
		a.drawing = ws
		defer func() { a.drawing = nil }()
		ws.View(c)
	}
}
