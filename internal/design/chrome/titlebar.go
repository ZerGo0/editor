package chrome

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/egoist/mygo/ui"
)

func NavButton(c *ui.Context, icon *ui.SVG, command string, history func(m *ui.Menu)) ui.Element {
	info, _ := action.Lookup(command)
	wrap := ui.Box(c.Key("nav:" + command)).Shrink(0)
	wrap.Children(func() {
		b := action.IconButton(c, icon, info.Title, action.IconOptions{Keys: info.Keys, Disabled: info.Disabled, ID: command})
		if history == nil {
			if b.Clicked() {
				action.Run(command)
			}
			return
		}
		open := ui.Local(wrap, "history", func() bool { return false })
		b.ContextMenu(history)
		switch {
		case b.LongPressed():
			*open = true
		case b.Clicked():
			action.Run(command)
		}
		anchor := ui.Box(c.Key("history-anchor")).Absolute().Left(0).Right(0).Bottom(0).Height(0).AccessHidden(true).TabStop(false)
		action.PopoverMenu(c, anchor, history, ui.PopoverMenuButtonOptions{Open: open})
	})
	return wrap
}
