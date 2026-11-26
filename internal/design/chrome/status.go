package chrome

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type StatusOptions struct {
	Icon    *ui.SVG
	Tooltip string
	Tone    token.Tone
	Command string
	Menu    func(m *ui.Menu)
}

func StatusItem(c *ui.Context, text string, o StatusOptions) ui.Element {
	p := token.Of(c)
	tip, keys, disabled := o.Tooltip, "", false
	if o.Command != "" {
		info, _ := action.Lookup(o.Command)
		keys, disabled = info.Keys, info.Disabled
		if tip == "" {
			tip = info.Title
		}
	}
	label := text
	if label == "" {
		label = tip
	}
	b := ui.ButtonBase(c.Key("status:"+label)).Height(token.StatusBarHeight.Px()).Padding(0, token.PadStatusItemX.Px()).
		Gap(token.GapInline.Px()).AlignItems(ui.Center).Shrink(0).Label(label).Disabled(disabled).
		FocusRingOffset(-token.FocusRingWidth.Px())
	if tip != "" {
		b.Tooltip(action.Hint(tip, keys))
	}
	if b.Hovered() || b.Pressed() {
		b.Background(p.Bar.StatusHover)
	}
	color := p.Bar.StatusText
	if o.Tone != token.ToneNeutral {
		color = p.Tone(o.Tone)
	}
	b.Children(func() {
		if o.Icon != nil {
			ui.Icon(c, o.Icon).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(color).AccessHidden(true)
		}
		if text != "" {
			token.TextLabel.Apply(ui.Text(c, text)).TextColor(p.Bar.StatusText).SingleLine().AccessHidden(true)
		}
	})
	switch {
	case o.Menu != nil:
		b.PopoverMenuButton(o.Menu, ui.PopoverMenuButtonOptions{At: ui.AnchorTopStart, Self: ui.AnchorBottomStart})
	case o.Command != "" && b.Clicked():
		action.Run(o.Command)
	}
	return b
}
