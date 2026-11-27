package chrome

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const tipSeparator = " - "

type Activity struct {
	Icon     *ui.SVG
	Label    string
	Keys     string
	Badge    int
	BadgeTip string
}

func ActivityBar(c *ui.Context, label string, selected *int, items []Activity) ui.Element {
	p := token.Of(c)
	parts := ui.TabsBase(c, selected, len(items))
	forward := parts.List.Focused()
	parts.List.Height(token.ActivityBarHeight.Px()).Padding(0, token.PadRow.Px()).AlignItems(ui.Center).Shrink(0).
		Label(label).Children(func() {
		for i, it := range items {
			active := i == *selected
			tab := parts.Tab(i).Size(token.ActivityItem.Px(), token.ActivityBarHeight.Px()).Center().Shrink(0).Label(it.Label).
				FocusRingOffset(-token.FocusRingWidth.Px())
			action.Tooltip(c, tab, activityTip(it))
			if active && forward {
				tab.Focus()
			}
			color := p.Bar.ActivityInactive
			if active || tab.Hovered() {
				color = p.Bar.ActivityActive
			}
			tab.Children(func() {
				ui.Icon(c, it.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(color).AccessHidden(true)
				if active {
					ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(token.IndicatorWidth.Px()).Background(p.Bar.ActivityIndicator)
				}
				if it.Badge > 0 {
					badge.Count(c, it.Badge, badge.CountOptions{Accent: true, Small: true}).Absolute().Top(token.GapInline.Px()).Left(token.ActivityItem.Px() - token.BadgeSmall.Px())
				}
			})
		}
	})
	return parts.List
}

func activityTip(it Activity) string {
	tip := action.Hint(it.Label, it.Keys)
	if it.BadgeTip == "" {
		return tip
	}
	return tip + tipSeparator + it.BadgeTip
}
