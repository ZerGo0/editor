package picker

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func Tabs(c *ui.Context, label string, selected *int, titles []string) ui.Element {
	p := token.Of(c)
	parts := ui.TabsBase(c, selected, len(titles))
	parts.List.Height(token.ControlHeight.Px()).AlignItems(ui.Stretch).Gap(token.GapTight.Px()).Shrink(0).Label(label).Children(func() {
		for i, title := range titles {
			tab := parts.Tab(i).FillHeight().Padding(0, token.PadControlX.Px()).Radius(token.RadiusControl.Px()).Center().Shrink(0).Label(title)
			color := p.Text.Secondary
			switch {
			case i == *selected:
				tab.Background(p.Control.ToggleActive)
				color = p.Text.Strong
			case tab.Hovered():
				tab.Background(p.Control.Hover)
				color = p.Text.Primary
			}
			tab.Children(func() {
				token.TextBody.WholeText(c, title).TextColor(color).SingleLine().AccessHidden(true)
			})
		}
	})
	return parts.List
}
