package token

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	DurationFast   = 100 * time.Millisecond
	DurationNormal = 150 * time.Millisecond
	DurationSlow   = 250 * time.Millisecond
	TooltipDelay   = 500 * time.Millisecond
	LongPress      = 400 * time.Millisecond
)

const (
	TwistieOpenAngle float32 = 90
	PopupOffsetY     float32 = -4
	PopupScale       float32 = 0.98
	noTransition             = -1
)

var (
	Quick  = ui.Animation{Duration: DurationFast, Ease: ui.EaseOut}
	Expand = ui.Animation{Duration: DurationNormal, Ease: ui.EaseOut}
)

func Popup(c *ui.Context) ui.ElementTransition {
	if c.Preferences().ReduceMotion {
		return ui.ElementTransition{Duration: noTransition}
	}
	motion := &ui.Motion{Y: PopupOffsetY, Scale: PopupScale, Flip: true}
	return ui.ElementTransition{Duration: DurationNormal, ExitDuration: DurationFast, Enter: motion, Exit: motion}
}
