package design

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const settingMenuLabel = "More Actions for "

type SettingOptions struct {
	Category string
	Modified bool
	Note     string
	Menu     func(m *ui.Menu)
}

func SettingRow(c *ui.Context, title, description string, o SettingOptions, control func()) ui.Element {
	p := token.Of(c)
	row := ui.Row(c.Key("setting:" + o.Category + title)).FillWidth().AlignItems(ui.Stretch).Role(ui.RoleGroup).Label(title).
		Radius(token.RadiusControl.Px())
	within := row.FocusWithin()
	entered := ui.Local(row, "focus-within", func() bool { return false })
	if within && !*entered {
		row.ScrollIntoView()
	}
	*entered = within
	show := row.Hovered() || within
	if show {
		row.Background(p.List.Hover)
	}
	row.Children(func() {
		bar := ui.Box(c).Width(token.ModifiedBarWidth.Px()).Shrink(0)
		if o.Modified {
			bar.Background(p.Accent.Base)
		}
		ui.Column(c).Grow(1).MinWidth(0).Padding(token.GapStack.Px(), token.PadRow.Px(), token.GapStack.Px(), token.PadPanel.Px()-token.ModifiedBarWidth.Px()).Gap(token.GapInline.Px()).Children(func() {
			ui.Row(c).FillWidth().Gap(token.GapRowIcon.Px()).AlignItems(ui.Center).Children(func() {
				spans := []ui.Span{{Text: title, Color: p.Text.Primary, Weight: token.TextStrong.Weight()}}
				if o.Category != "" {
					spans = append([]ui.Span{{Text: o.Category + ": ", Color: p.Text.Secondary, Weight: token.TextStrong.Weight()}}, spans...)
				}
				token.TextStrong.Apply(ui.RichText(c, spans...)).SingleLine().Shrink(1).MinWidth(0)
				if o.Note != "" {
					Text(c, o.Note, token.TextCaption).SingleLine().Shrink(1).MinWidth(0)
				}
				Spacer(c)
				if o.Menu != nil {
					gear := action.MenuIconButton(c, icons.Settings, settingMenuLabel+title, action.IconOptions{Tooltip: "More Actions..."}, o.Menu)
					if !show {
						gear.Opacity(token.OpacityHidden.Value())
					}
				}
			})
			if description != "" {
				Text(c, description, token.TextParagraph).Wrap()
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(control)
		})
	})
	return row
}
