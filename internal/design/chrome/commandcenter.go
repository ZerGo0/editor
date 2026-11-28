package chrome

import (
	"sync"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

var commandCenter = struct {
	sync.Mutex
	bounds map[ui.Services]ui.Rect
}{bounds: map[ui.Services]ui.Rect{}}

func CommandCenterBounds(c *ui.Context) ui.Rect {
	commandCenter.Lock()
	defer commandCenter.Unlock()
	return commandCenter.bounds[c.Services()]
}

func CommandCenter(c *ui.Context, text, command string, pathMenu func(m *ui.Menu)) ui.Element {
	p := token.Of(c)
	info, _ := action.Lookup(command)
	b := ui.ButtonBase(c.Key("command-center:"+command)).Height(token.ControlHeightSmall.Px()).Grow(1).
		MinWidth(token.CommandCenterMinWidth.Px()).MaxWidth(token.CommandCenterWidth.Px()).Radius(token.RadiusPopover.Px()).
		Padding(0, token.PadControlX.Px()).Gap(token.GapRowIcon.Px()).Center().Label(info.Title).
		Border(token.BorderWidth.Px(), p.Bar.CommandCenterBorder)
	commandCenter.Lock()
	commandCenter.bounds[c.Services()] = b.Bounds()
	commandCenter.Unlock()
	action.Tooltip(c, b, action.Hint(info.Title, info.Keys))
	bg := p.Bar.CommandCenter
	if b.Hovered() || b.Pressed() {
		bg = p.Bar.CommandCenterHover
	}
	open := ui.Local(b, "path-menu", func() bool { return false })
	b.Background(bg).Children(func() {
		ui.Icon(c, icons.Search).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(p.Text.Secondary).Shrink(0)
		token.TextLabel.Apply(ui.Text(c, text)).TextColor(p.Text.Secondary).SingleLine().MinWidth(0).Shrink(1)
		if pathMenu != nil {
			anchor := ui.Box(c.Key("path-anchor")).Absolute().Left(0).Right(0).Bottom(0).Height(0).AccessHidden(true).TabStop(false)
			action.PopoverMenu(c, anchor, pathMenu, ui.PopoverMenuButtonOptions{Open: open})
		}
	})
	if pathMenu != nil {
		b.ContextMenu(pathMenu)
	}
	if b.Clicked() {
		if pathMenu != nil && c.Modifiers()&ui.Cmd != 0 {
			*open = true
		} else {
			action.Run(command)
		}
	}
	return b
}
