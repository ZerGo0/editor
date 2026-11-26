package chrome

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type ViewTab struct {
	Title string
	Keys  string
	Badge int
}

func ViewTabs(c *ui.Context, label string, selected *int, tabs []ViewTab) ui.Element {
	p := token.Of(c)
	parts := ui.TabsBase(c, selected, len(tabs))
	parts.List.FillHeight().AlignItems(ui.Stretch).Gap(token.GapTight.Px()).Shrink(0).Label(label).Children(func() {
		for i, t := range tabs {
			tab := parts.Tab(i).FillHeight().Padding(0, token.PadControlX.Px()).Gap(token.GapRowIcon.Px()).AlignItems(ui.Center).Shrink(0).
				Label(t.Title).Tooltip(action.Hint(t.Title, t.Keys)).FocusRingOffset(-token.FocusRingWidth.Px())
			color := p.Text.Secondary
			if i == *selected || tab.Hovered() {
				color = p.Text.Primary
			}
			tab.Children(func() {
				token.TextCaption.Apply(ui.Text(c, token.TextTitle.Format(t.Title))).TextColor(color).SingleLine().AccessHidden(true)
				if t.Badge > 0 {
					badge.Count(c, t.Badge, badge.CountOptions{Small: true})
				}
				if i == *selected {
					ui.Box(c).Absolute().Left(token.PadControlX.Px()).Right(token.PadControlX.Px()).Bottom(0).
						Height(token.IndicatorWidth.Px()).Background(p.Accent.Base)
				}
			})
		}
	})
	return parts.List
}
