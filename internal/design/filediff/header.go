package filediff

import (
	"path"
	"strconv"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	CountHidden = -1
	arrowText   = "\u2192"
)

type HeaderOptions struct {
	Path     string
	OldPath  string
	Status   token.GitStatus
	Adds     int
	Dels     int
	Open     bool
	OnToggle func()
	Viewed   bool
	OnViewed func(on bool)
	Actions  func(c *ui.Context)
}

func Height() float32 { return token.HeaderHeight.Px() + 2*token.BorderWidth.Px() }

func Header(c *ui.Context, o HeaderOptions) ui.Element {
	p := token.Of(c)
	dir, name := path.Split(o.Path)
	bw := token.BorderWidth.Px()
	gap, radius := float32(0), []float32{cardRadius, cardRadius, 0, 0}
	if !o.Open {
		gap, radius = CardGap, []float32{cardRadius}
	}
	slot := cardSlot(c.Key("file-header:"+o.Path), gap)
	slot.Children(func() {
		ui.Column(c).FillWidth().Background(p.Layer.SectionHeader).Role(ui.RoleGroup).Label(o.Path).
			BorderWidth(bw).BorderColor(p.Border.Default).Radius(radius...).Children(func() {
			ui.Row(c).FillWidth().Height(token.HeaderHeight.Px()).Shrink(0).AlignItems(ui.Center).Padding(0, token.PadRow.Px()).Gap(token.GapRowIcon.Px()).Children(func() {
				label := "Collapse " + o.Path
				if !o.Open {
					label = "Expand " + o.Path
				}
				if action.ExpandButton(c, label, o.Open, action.IconOptions{ID: "toggle"}).Clicked() && o.OnToggle != nil {
					o.OnToggle()
				}
				badge.Decoration(c, o.Status)
				ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(0).Tooltip(o.Path).Children(func() {
					if o.OldPath != "" && o.OldPath != o.Path {
						token.TextLabel.Apply(ui.Text(c, o.OldPath+" "+arrowText+" ")).TextColor(p.Text.Secondary).SingleLine().Shrink(1).MinWidth(0).TruncateStart()
					}
					if dir != "" {
						token.TextLabel.Apply(ui.Text(c, dir)).TextColor(p.Text.Secondary).SingleLine().Shrink(1).MinWidth(0).TruncateStart()
					}
					token.TextStrong.Apply(ui.Text(c, name)).TextColor(p.Text.Primary).SingleLine().Shrink(0)
				})
				Counts(c, o.Adds, o.Dels)
				if o.OnViewed != nil {
					viewed := o.Viewed
					if action.Checkbox(c, "Viewed", &viewed, action.ChoiceOptions{ID: "viewed:" + o.Path}).Changed() {
						o.OnViewed(viewed)
					}
				}
				if o.Actions != nil {
					action.Toolbar(c, o.Path).Children(func() { o.Actions(c) })
				}
			})
		})
	})
	return slot
}

func Counts(c *ui.Context, adds, dels int) {
	if adds == CountHidden || dels == CountHidden {
		return
	}
	p := token.Of(c)
	token.TextLabel.Apply(ui.Text(c, "+"+strconv.Itoa(adds))).TextColor(p.Tone(token.ToneSuccess)).SingleLine().Shrink(0)
	token.TextLabel.Apply(ui.Text(c, "-"+strconv.Itoa(dels))).TextColor(p.Tone(token.ToneError)).SingleLine().Shrink(0)
}
