package picker

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type Tab struct {
	Title   string
	Command string
}

func Tabs(c *ui.Context, label string, selected *int, tabs []Tab) ui.Element {
	p := token.Of(c)
	parts := ui.TabsBase(c, selected, len(tabs))
	parts.List.Height(token.ControlHeight.Px()).AlignItems(ui.Stretch).Gap(token.GapTight.Px()).Shrink(0).Label(label).Children(func() {
		for i, t := range tabs {
			tab := parts.Tab(i).FillHeight().Padding(0, token.PadControlX.Px()).Radius(token.RadiusControl.Px()).Center().Shrink(0).Label(t.Title)
			if t.Command != "" {
				info, _ := action.Lookup(t.Command)
				action.Tooltip(c, tab, action.Hint(info.Title, info.Keys))
			}
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
				token.TextBody.WholeText(c, t.Title).TextColor(color).SingleLine().AccessHidden(true)
			})
		}
	})
	return parts.List
}
