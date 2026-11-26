package diffmode

import (
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	controlHeight   = 22
	controlRadius   = 5
	controlPadding  = 6
	controlGap      = 4
	controlFont     = 12
	chevronSize     = 10
	spinnerSize     = 14
	whitespaceLabel = "Ignore Whitespace"
	movesToggle     = "Show Moved Code"
)

func Controls(c *ui.Context, e *Engine) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("diff-whitespace")).Height(controlHeight).Padding(0, controlPadding).Gap(controlGap).
		Radius(controlRadius).AlignItems(ui.Center).Label(whitespaceLabel).Tooltip(whitespaceLabel)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, WhitespaceLabel(Session.Whitespace)).FontSize(controlFont).SingleLine()
		ui.Icon(c, icons.ChevronDown).Size(chevronSize, chevronSize).TextColor(t.TextMuted)
	})
	b.PopoverMenuButton(func(m *ui.Menu) {
		for _, choice := range WhitespaceChoices {
			if m.Item(choice.Label).Radio(Session.Whitespace == choice.Mode).Chosen() {
				Session.Whitespace = choice.Mode
			}
		}
	})
	ui.Toggle(c.Key("diff-show-moves"), &Session.ShowMoves, movesToggle)
	if e != nil && e.Busy() {
		ui.Spinner(c).Size(spinnerSize, spinnerSize).Label("Comparing")
	}
	if e != nil && e.Notice() != "" {
		ui.Text(c, e.Notice()).FontSize(controlFont).TextColor(t.Warning).SingleLine().Shrink(1).MinWidth(0)
	}
}
