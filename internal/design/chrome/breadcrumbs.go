package chrome

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type Crumb struct {
	Label   string
	Icon    *ui.SVG
	Leading func(c *ui.Context)
	Menu    func(m *ui.Menu)
}

func Breadcrumbs(c *ui.Context, label string, crumbs []Crumb) ui.Element {
	p := token.Of(c)
	return ui.Row(c.Key("breadcrumbs:"+label)).Height(token.RowHeight.Px()).FillWidth().MinWidth(0).Clip().Padding(0, token.PadRow.Px()).
		AlignItems(ui.Center).Gap(token.GapTight.Px()).Role(ui.RoleNavigation).Label(label).FocusGroup(ui.Horizontal).
		Children(func() {
			for i, cr := range crumbs {
				if i > 0 {
					ui.Icon(c, icons.ChevronRight).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).
						TextColor(p.Text.Secondary).Shrink(0).AccessHidden(true)
				}
				crumb(c, p, i, cr, i == len(crumbs)-1)
			}
		})
}

func crumb(c *ui.Context, p *token.Palette, i int, cr Crumb, last bool) {
	b := ui.ButtonBase(c.Key("crumb:"+strconv.Itoa(i))).Height(token.RowHeight.Px()).Padding(0, token.PadInputEnd.Px()).
		Gap(token.GapInline.Px()).AlignItems(ui.Center).Radius(token.RadiusControl.Px()).MinWidth(0).Shrink(1).Label(cr.Label)
	color := p.Text.Secondary
	if last || b.Hovered() {
		color = p.Text.Primary
	}
	if b.Pressed() {
		b.Background(p.Control.Pressed)
	}
	b.Children(func() {
		if cr.Leading != nil {
			cr.Leading(c)
		} else if cr.Icon != nil {
			ui.Icon(c, cr.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon).Shrink(0).AccessHidden(true)
		}
		token.TextBody.Apply(ui.Text(c, cr.Label)).TextColor(color).SingleLine().MinWidth(0).Shrink(1).AccessHidden(true)
	})
	if cr.Menu != nil {
		b.PopoverMenuButton(cr.Menu, ui.PopoverMenuButtonOptions{At: ui.AnchorBottomStart, Self: ui.AnchorTopStart})
	}
}
