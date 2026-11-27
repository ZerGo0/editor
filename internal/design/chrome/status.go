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
	ID      string
	OnClick func()
}

type StatusPart struct {
	Icon *ui.SVG
	Text string
	Tone token.Tone
}

func StatusItem(c *ui.Context, text string, o StatusOptions) ui.Element {
	return StatusParts(c, text, []StatusPart{{Icon: o.Icon, Text: text, Tone: o.Tone}}, o)
}

func StatusParts(c *ui.Context, label string, parts []StatusPart, o StatusOptions) ui.Element {
	p := token.Of(c)
	tip, keys, disabled := o.Tooltip, "", false
	if o.Command != "" {
		info, _ := action.Lookup(o.Command)
		keys, disabled = info.Keys, info.Disabled
		if tip == "" {
			tip = info.Title
		}
	}
	if label == "" {
		label = tip
	}
	key := o.ID
	if key == "" {
		key = label
	}
	b := ui.ButtonBase(c.Key("status:"+key)).FillHeight().Padding(0, token.PadStatusItemX.Px()).
		Gap(token.GapInline.Px()).AlignItems(ui.Center).Shrink(0).Label(label).Disabled(disabled).
		FocusRingOffset(-token.FocusRingWidth.Px())
	if tip != "" {
		action.Tooltip(c, b, action.Hint(tip, keys))
	}
	if b.Hovered() || b.Pressed() {
		b.Background(p.Bar.StatusHover)
	}
	b.Children(func() {
		for _, part := range parts {
			color := p.Bar.StatusText
			if part.Tone != token.ToneNeutral {
				color = p.Tone(part.Tone)
			}
			if part.Icon != nil {
				ui.Icon(c, part.Icon).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(color).AccessHidden(true)
			}
			if part.Text != "" {
				token.TextLabel.ApplyLine(ui.Text(c, part.Text)).TextColor(p.Bar.StatusText).SingleLine().AccessHidden(true)
			}
		}
	})
	switch {
	case o.Menu != nil:
		action.PopoverMenu(c, b, o.Menu, ui.PopoverMenuButtonOptions{At: ui.AnchorTopStart, Self: ui.AnchorBottomStart})
	case !b.Clicked():
	case o.OnClick != nil:
		o.OnClick()
	case o.Command != "":
		action.Run(o.Command)
	}
	return b
}
