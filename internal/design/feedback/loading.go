package feedback

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	captionTrim     = ".… "
	loadingSinceKey = "loading-since"
)

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
	since := *ui.Local(e, loadingSinceKey, c.Now)
	if c.Now().Sub(since) >= token.SpinnerDelay {
		return true
	}
	svc := c.Services()
	e.Draw(func(pt *ui.Painter, _ ui.Rect) {
		if wait := token.SpinnerDelay - pt.Now().Sub(since); wait > 0 {
			pt.After(wait)
			return
		}
		svc.Invalidate()
	})
	return false
}
