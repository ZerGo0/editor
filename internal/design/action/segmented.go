package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type SegmentedOptions struct {
	Disabled bool
	ID       string
}

func Segmented(c *ui.Context, label string, selected *int, labels []string, o SegmentedOptions) ui.Element {
	p := token.Of(c)
	seg := ui.SegmentedBase(c.Key(keyOf(o.ID, label)), selected, len(labels))
	seg.Track.Height(token.ControlHeight.Px()).Padding(token.GapTight.Px()).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).
		Radius(token.RadiusControl.Px()).Background(p.Layer.Input).Border(token.BorderWidth.Px(), p.Border.Input).Label(label).Disabled(o.Disabled)
	seg.Track.Children(func() {
		for i, text := range labels {
			s := seg.Segment(i).FillHeight().Padding(0, token.PadControlX.Px()).Radius(token.RadiusInline.Px()).Center().Label(text)
			color := p.Text.Secondary
			switch {
			case *selected == i:
				s.Background(p.Control.ToggleActive)
				color = p.Text.Strong
			case s.Hovered():
				s.Background(p.Control.Hover)
			}
			s.Children(func() {
				token.TextBody.Apply(ui.Text(c, text)).TextColor(color).SingleLine()
			})
		}
	})
	return seg.Track
}
