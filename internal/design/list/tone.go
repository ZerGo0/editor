package list

import (
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func toneCount(c *ui.Context, p *token.Palette, it Item, active bool) {
	color := p.Tone(it.Tone)
	if active {
		color = p.List.ActiveSelectionText
	}
	token.TextLabel.Apply(ui.Text(c, badge.CountText(it.Count))).FontWeight(token.WeightSemibold).TextColor(color).
		MinWidth(token.DecorationWidth.Px()).TextAlign(ui.Center).SingleLine().Shrink(0)
}
