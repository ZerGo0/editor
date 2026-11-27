package action

import "github.com/egoist/mygo/ui"

type menuOpen struct{}

func PopoverMenu(c *ui.Context, anchor ui.Element, menu func(m *ui.Menu), o ui.PopoverMenuButtonOptions) ui.Element {
	if o.Open == nil {
		o.Open = ui.Local(anchor, menuOpen{}, func() bool { return false })
	}
	if *o.Open && !c.WindowActive() {
		*o.Open = false
	}
	return anchor.KeepFocus().PopoverMenuButton(menu, o)
}
