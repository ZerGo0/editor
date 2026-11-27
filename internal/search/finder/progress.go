package finder

import (
	"time"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type delayedBusy struct {
	since time.Time
	on    bool
}

func (g *delayedBusy) visible(c *ui.Context, busy bool) bool {
	if !busy {
		g.on = false
		return false
	}
	if !g.on {
		g.on, g.since = true, c.Now()
	}
	left := token.SpinnerDelay - c.Now().Sub(g.since)
	if left <= 0 {
		return true
	}
	c.After(left)
	return false
}
