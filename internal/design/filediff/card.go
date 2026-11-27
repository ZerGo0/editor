package filediff

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

var (
	cardInset  = token.GapControl.Px()
	CardGap    = token.GapGroup.Px()
	cardRadius = token.RadiusCard.Px()
)

func cardSlot(c *ui.Context, gap float32) ui.Element {
	return ui.Column(c).FillWidth().Shrink(0).Padding(0, cardInset, gap, cardInset)
}

func Body(c *ui.Context, children func()) {
	p := token.Of(c)
	bw := token.BorderWidth.Px()
	cardSlot(c.Key("file-body"), CardGap).Children(func() {
		ui.Column(c).FillWidth().BorderWidth(0, bw, bw, bw).BorderColor(p.Border.Default).Radius(0, 0, cardRadius, cardRadius).Children(children)
	})
}
