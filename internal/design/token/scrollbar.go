package token

import "github.com/egoist/mygo/ui"

func ScrollbarStyle(p *Palette) ui.ScrollbarStyle {
	return ui.ScrollbarStyle{}.Resolve(p.Theme())
}
