package badge

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	maxCount     = 999
	overflowMark = "+"
)

type CountOptions struct {
	Accent bool
	Small  bool
	Label  string
}

func CountText(n int) string {
	if n > maxCount {
		return strconv.Itoa(maxCount) + overflowMark
	}
	return strconv.Itoa(n)
}

func Count(c *ui.Context, n int, o CountOptions) ui.Element {
	p := token.Of(c)
	bg, fg := p.Badge.Background, p.Badge.Text
	if o.Accent {
		bg, fg = p.Badge.Accent, p.Badge.AccentText
	}
	height := token.BadgeHeight.Px()
	if o.Small {
		height = token.BadgeSmall.Px()
	}
	el := ui.Text(c, CountText(n)).FontSize(token.TextBadge.Px()).FontWeight(token.TextBadge.Weight()).TextColor(fg).
		Background(bg).Radius(token.RadiusPill.Px()).MinWidth(height).Height(height).Padding(0, token.PadBadgeX.Px()).TextAlign(ui.Center).
		SingleLine().Shrink(0)
	if o.Label != "" {
		el.Label(o.Label)
	}
	return el
}

func Decoration(c *ui.Context, status token.GitStatus) ui.Element {
	p := token.Of(c)
	return ui.Text(c, status.Letter()).FontSize(token.TextLabel.Px()).FontWeight(token.WeightSemibold).TextColor(p.GitColor(status)).
		Width(token.DecorationWidth.Px()).TextAlign(ui.Center).SingleLine().Shrink(0)
}
