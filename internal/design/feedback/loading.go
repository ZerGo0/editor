package feedback

import (
	"math"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	skeletonRows      = 4
	skeletonPulse     = 1400 * time.Millisecond
	skeletonStep      = 66 * time.Millisecond
	skeletonRowPhase  = 0.12
	skeletonAlphaLow  = 0.05
	skeletonAlphaHigh = 0.13
	captionTrim       = ".… "
)

var skeletonShares = []float32{0.72, 0.48, 0.86, 0.6}

func Loading(c *ui.Context, text string) ui.Element {
	p := token.Of(c)
	rowH := token.RowHeight.Px()
	box := ui.Box(c).FillWidth().Height(skeletonRows * rowH).MinHeight(rowH).Role(ui.RoleProgress).Busy(true).Label(text)
	if !pastDelay(c, box) {
		return box
	}
	base := p.Text.Primary
	pad, dot, gap, bar := token.PadRow.Px(), token.IconSizeSmall.Px(), token.GapRowIcon.Px(), token.Space8.Px()
	return box.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.After(skeletonStep)
		now := float64(pt.Now().UnixMilli()%skeletonPulse.Milliseconds()) / float64(skeletonPulse.Milliseconds())
		for i := range skeletonRows {
			y := r.Y + float32(i)*rowH
			if y+rowH > r.Y+r.H {
				return
			}
			level := float32(0.5 + 0.5*math.Sin(2*math.Pi*(now-float64(i)*skeletonRowPhase)))
			color := base.Alpha(skeletonAlphaLow + (skeletonAlphaHigh-skeletonAlphaLow)*level)
			x, cy := r.X+pad, y+rowH/2
			pt.Fill(ui.Rect{X: x, Y: cy - dot/2, W: dot, H: dot}, color, token.RadiusInline.Px())
			bx := x + dot + gap
			pt.Fill(ui.Rect{X: bx, Y: cy - bar/2, W: (r.X + r.W - pad - bx) * skeletonShares[i%len(skeletonShares)], H: bar}, color, bar/2)
		}
	})
}

func LoadingPane(c *ui.Context, text string) ui.Element {
	p := token.Of(c)
	col := ui.Column(c).AlignItems(ui.Center).Gap(token.GapControl.Px()).Role(ui.RoleProgress).Busy(true).Label(text)
	if !pastDelay(c, col) {
		return col
	}
	return col.Children(func() {
		ui.Spinner(c).Size(token.SpinnerSizeLarge.Px(), token.SpinnerSizeLarge.Px()).TextColor(p.Text.Secondary).AccessHidden(true)
		token.TextLabel.Apply(ui.Text(c, strings.TrimRight(text, captionTrim))).TextColor(p.Text.Secondary).AccessHidden(true)
	})
}

func pastDelay(c *ui.Context, e ui.Element) bool {
	since := ui.Local(e, "loading-since", c.Now)
	if wait := token.SpinnerDelay - c.Now().Sub(*since); wait > 0 {
		c.After(wait)
		return false
	}
	return true
}
