package action

import "github.com/egoist/mygo/ui"

func MenuButton(c *ui.Context, label string, o ButtonOptions, menu func(m *ui.Menu)) ui.Element {
	return Button(c, label, o).PopoverMenuButton(menu, ui.PopoverMenuButtonOptions{At: ui.AnchorBottomEnd, Self: ui.AnchorTopEnd})
}
