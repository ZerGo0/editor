package quickinput

import (
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	none         = -1
	defaultEmpty = "No matching results"
)

type Item struct {
	Label       string
	Matches     []int
	Description string
	Icon        *ui.SVG
	Leading     func(c *ui.Context)
	Keys        string
	Group       string
}

type State struct {
	Query    string
	Selected int
	list     ui.ListState
	scroll   bool
}

type Options struct {
	Placeholder string
	Busy        bool
	Empty       string
}

type Result struct {
	Accepted int
	Closed   bool
}

func View(c *ui.Context, label string, s *State, items []Item, o Options) Result {
	res := Result{Accepted: none}
	s.Selected = clamp(s.Selected, len(items))
	p := token.Of(c)
	ui.Overlay(c, func() {
		back := ui.Column(c.Key("quick-input:" + label)).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Center).Modal()
		back.Children(func() {
			panel := ui.Column(c).Margin(token.TitleBarHeight.Px()+token.OffsetQuickOpen.Px(), 0, 0, 0).
				Width(token.QuickInputWidth.Px()).Padding(token.PadPopover.Px()).Gap(token.PadPopover.Px()).
				Background(p.Layer.QuickInput).Radius(token.RadiusPopover.Px()).Border(token.BorderWidth.Px(), p.Border.Widget).
				ZIndex(token.LayerQuickInput.Z()).Role(ui.RoleDialog).Label(label)
			token.ElevationDialog.Apply(panel, p)
			panel.Children(func() {
				in := field.TextInput(c, label, &s.Query, field.InputOptions{Placeholder: o.Placeholder, AutoFocus: true})
				switch {
				case in.Shortcut(0, ui.KeyDown):
					s.move(1, len(items))
				case in.Shortcut(0, ui.KeyUp):
					s.move(-1, len(items))
				case in.Submitted() && s.Selected != none:
					res.Accepted = s.Selected
				}
				if in.Changed() {
					s.Selected = clamp(0, len(items))
				}
				if o.Busy {
					feedback.ProgressBar(c, label)
				}
				rows(c, p, s, items, o, &res)
			})
			if panel.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				res.Closed = true
			}
		})
	})
	return res
}

func rows(c *ui.Context, p *token.Palette, s *State, items []Item, o Options, res *Result) {
	maxHeight := token.RowHeight.Px() * token.QuickInputRows
	l := ui.List(c, &s.list, len(items), func(i int) {
		it := items[i]
		selected := i == s.Selected
		row := ui.Row(c).Height(token.RowHeight.Px()).FillWidth().Padding(0, token.PadRow.Px()).Gap(token.GapRowIcon.Px()).
			AlignItems(ui.Center).Radius(token.RadiusControl.Px()).Label(it.Label)
		text, secondary, match := p.Text.Primary, p.Text.Secondary, p.List.Highlight
		switch {
		case selected:
			row.Background(p.List.PickSelection)
			text, secondary, match = p.List.PickSelectionText, p.List.PickSelectionText, p.List.PickHighlight
		case row.Hovered():
			row.Background(p.List.Hover)
		}
		if it.Group != "" && (i == 0 || items[i-1].Group != it.Group) && i > 0 {
			row.BorderWidth(token.BorderWidth.Px(), 0, 0, 0).BorderColor(p.Border.Default)
		}
		row.Children(func() {
			switch {
			case it.Leading != nil:
				it.Leading(c)
			case it.Icon != nil:
				ui.Icon(c, it.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(text).Shrink(0).AccessHidden(true)
			}
			ui.RichText(c, list.Spans(it.Label, it.Matches, text, match)...).FontSize(token.TextBody.Px()).SingleLine().
				Shrink(0).MaxWidth(token.QuickInputWidth.Px() / 2)
			token.TextLabel.Apply(ui.Text(c, it.Description)).TextColor(secondary).SingleLine().Grow(1).Shrink(1).MinWidth(0)
			if it.Group != "" && (i == 0 || items[i-1].Group != it.Group) {
				token.TextLabel.Apply(ui.Text(c, it.Group)).TextColor(secondary).SingleLine().Shrink(0)
			}
			if it.Keys != "" {
				badge.Keys(c, it.Keys)
			}
		})
		if it.Keys != "" {
			row.Description(it.Keys)
		}
		if row.Clicked() {
			res.Accepted = i
		}
	}).FillWidth().MaxHeight(maxHeight).Shrink(1).Background(ui.Transparent)
	if s.scroll && s.Selected != none {
		s.list.ScrollIntoView(s.Selected)
		s.scroll = false
	}
	if len(items) == 0 {
		empty := o.Empty
		if empty == "" {
			empty = defaultEmpty
		}
		l.Children(func() {
			token.TextBody.Apply(ui.Text(c, empty)).TextColor(p.Text.Secondary).Padding(token.PadInputY.Px(), token.PadRow.Px())
		})
	}
}

func (s *State) move(delta, n int) {
	if n == 0 {
		s.Selected = none
		return
	}
	s.Selected = (s.Selected + delta + n) % n
	s.scroll = true
}

func clamp(i, n int) int {
	switch {
	case n == 0:
		return none
	case i < 0:
		return 0
	case i >= n:
		return n - 1
	}
	return i
}
