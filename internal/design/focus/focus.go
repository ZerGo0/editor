package focus

import "github.com/egoist/mygo/ui"

func Return(c *ui.Context, key string) {
	ui.Overlay(c, func() {
		ui.Box(c.Key("focus-return:" + key)).Absolute().Width(0).Height(0).TabStop(false).AccessHidden(true)
	})
}
