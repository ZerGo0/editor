package chrome

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func CommandCenter(c *ui.Context, text, command string) ui.Element {
	p := token.Of(c)
	info, _ := action.Lookup(command)
	b := ui.ButtonBase(c.Key("command-center:"+command)).Height(token.ControlHeightSmall.Px()).Grow(1).
		MinWidth(token.CommandCenterMinWidth.Px()).MaxWidth(token.CommandCenterWidth.Px()).Radius(token.RadiusPopover.Px()).
		Padding(0, token.PadControlX.Px()).Gap(token.GapRowIcon.Px()).Center().Label(info.Title).
		Border(token.BorderWidth.Px(), p.Bar.CommandCenterBorder)
	action.Tooltip(c, b, action.Hint(info.Title, info.Keys))
	bg := p.Bar.CommandCenter
	if b.Hovered() || b.Pressed() {
		bg = p.Bar.CommandCenterHover
	}
	b.Background(bg).Children(func() {
		ui.Icon(c, icons.Search).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(p.Text.Secondary).Shrink(0)
		token.TextLabel.Apply(ui.Text(c, text)).TextColor(p.Text.Secondary).SingleLine().MinWidth(0).Shrink(1)
	})
	if b.Clicked() {
		action.Run(command)
	}
	return b
}

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
		ui.Box(c.Key("history-anchor")).Absolute().Left(0).Right(0).Bottom(0).Height(0).AccessHidden(true).TabStop(false).
			PopoverMenuButton(history, ui.PopoverMenuButtonOptions{Open: open})
	})
	return wrap
}
