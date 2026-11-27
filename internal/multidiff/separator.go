package multidiff

import (
	"fmt"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func separator(c *ui.Context, sep ui.Element, s *editor.DiffSeparator) {
	p := token.Of(c)
	sep.Padding(token.GapInline.Px())
	sep.Children(func() {
		bar := ui.Row(c).Grow(1).AlignSelf(ui.Stretch).AlignItems(ui.Center).Gap(token.GapInline.Px()).Padding(0, token.PadRow.Px()).
			Radius(token.RadiusControl.Px()).Background(p.Layer.Panel)
		bar.Children(func() {
			if s.Lines == 0 {
				token.TextMono.Apply(ui.Text(c, s.Header)).TextColor(p.Text.Secondary).SingleLine().Shrink(1).MinWidth(0)
				return
			}
			expand := func(fn func()) func() {
				return func() {
					if c.Modifiers()&ui.Shift != 0 {
						fn = s.ExpandAll
					}
					fn()
				}
			}
			chevron := func(icon *ui.SVG, label string, fn func()) {
				action.IconButton(c, icon, label, action.IconOptions{ID: label}).TabStop(false).OnClick(expand(fn))
			}
			switch {
			case s.First:
				chevron(icons.ChevronUp, "Expand Up", s.ExpandUp)
			case s.Last:
				chevron(icons.ChevronDown, "Expand Down", s.ExpandDown)
			case s.Lines <= s.Step:
				chevron(icons.ExpandAll, "Expand All", s.ExpandAll)
			default:
				chevron(icons.ChevronDown, "Expand Down", s.ExpandDown)
				chevron(icons.ChevronUp, "Expand Up", s.ExpandUp)
			}
			action.Button(c, fmt.Sprintf("%d Lines", s.Lines), action.ButtonOptions{Variant: action.Ghost, Small: true, ID: "expand-all-lines"}).Label(fmt.Sprintf("Expand %d Unmodified Lines", s.Lines)).TabStop(false).OnClick(s.ExpandAll)
		})
	})
}
