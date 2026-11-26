package action

import (
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type ItemOptions struct {
	Icon     *ui.SVG
	Mods     ui.Modifiers
	Key      ui.Key
	Detail   string
	Checked  bool
	Disabled bool
	Danger   bool
	ID       string
}

var menuIcons = struct {
	sync.Mutex
	cache map[*ui.SVG]*mygo.MenuIcon
}{cache: map[*ui.SVG]*mygo.MenuIcon{}}

func Item(m *ui.Menu, label string, o ItemOptions) *ui.MenuItem {
	it := m.Item(label).Disabled(o.Disabled)
	if o.ID != "" {
		it.ID(o.ID)
	}
	if o.Checked {
		it.Checked(true)
	}
	if o.Key != 0 {
		it.Shortcut(o.Mods, o.Key)
	}
	if o.Detail != "" {
		it.Trailing(o.Detail)
	}
	if o.Danger {
		it.Danger()
	}
	if icon := menuIcon(o.Icon); icon != nil {
		it.Icon(icon)
	}
	return it
}

func menuIcon(svg *ui.SVG) *mygo.MenuIcon {
	if svg == nil {
		return nil
	}
	menuIcons.Lock()
	defer menuIcons.Unlock()
	if icon, ok := menuIcons.cache[svg]; ok {
		return icon
	}
	icon, err := mygo.NewMenuIconSVG(svg, true)
	if err != nil {
		icon = nil
	}
	menuIcons.cache[svg] = icon
	return icon
}
