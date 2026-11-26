package list

import (
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type Item struct {
	Label         string
	Matches       []int
	Description   string
	Icon          *ui.SVG
	Leading       func(c *ui.Context)
	Git           token.GitStatus
	Count         int
	Strike        bool
	Muted         bool
	Tooltip       string
	Actions       func(c *ui.Context)
	Menu          func(m *ui.Menu)
	OnClick       func()
	OnDoubleClick func()
	Drop          func(row ui.Element) bool
}

type State struct {
	Selected bool
	Focused  bool
	Hovered  bool
	Depth    int
	Guides   bool
	Branch   bool
	Open     bool
	arrow    func() ui.Element
	tree     bool
}

func Row(c *ui.Context, s State, it Item) ui.Element {
	p := token.Of(c)
	row := ui.Row(c).Height(token.RowHeight.Px()).FillWidth().AlignItems(ui.Center).Padding(0, token.PadRow.Px())
	hovered := s.Hovered
	if s.tree {
		hovered = row.Hovered()
	}
	active := s.Selected && s.Focused
	dropping := it.Drop != nil && it.Drop(row)
	switch {
	case dropping:
		row.Background(p.List.Drop)
	case active:
		row.Background(p.List.ActiveSelection).Border(token.BorderWidth.Px(), p.List.FocusOutline)
	case s.Selected:
		row.Background(p.List.InactiveSelection)
	case hovered:
		row.Background(p.List.Hover)
	}
	if it.Tooltip != "" {
		row.Tooltip(it.Tooltip)
	}
	if s.tree && s.Guides && s.Depth > 0 {
		guides(row, s.Depth, p.List.IndentGuide)
	}
	row.Children(func() {
		if s.tree {
			twistie(c, p, s)
		}
		leading(c, p, it, active)
		label(c, p, it, active)
		if it.Actions != nil && (hovered || active) {
			ui.Row(c).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).Children(func() { it.Actions(c) })
		}
		if it.Count > 0 {
			badge.Count(c, it.Count, badge.CountOptions{})
		}
		if it.Git != token.GitNone {
			badge.Decoration(c, it.Git)
		}
	})
	if it.Menu != nil {
		row.ContextMenu(it.Menu)
	}
	switch {
	case it.OnDoubleClick != nil && row.RowDoubleClicked():
		it.OnDoubleClick()
	case it.OnClick != nil && row.RowClicked():
		it.OnClick()
	}
	return row
}

func twistie(c *ui.Context, p *token.Palette, s State) {
	ui.Box(c).Width(float32(s.Depth) * token.IndentStep.Px()).Shrink(0)
	if s.arrow == nil {
		return
	}
	arrow := s.arrow().Size(token.TwistieWidth.Px(), token.RowHeight.Px()).Center().Shrink(0)
	if !s.Branch {
		return
	}
	target := float32(0)
	if s.Open {
		target = token.TwistieOpenAngle
	}
	angle := arrow.AnimateTo("twistie", target, token.Expand)
	arrow.Children(func() {
		ui.Icon(c, icons.ChevronRight).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon).Rotate(angle)
	})
}

func leading(c *ui.Context, p *token.Palette, it Item, active bool) {
	switch {
	case it.Leading != nil:
		ui.Row(c).Shrink(0).AlignItems(ui.Center).Margin(0, token.GapRowIcon.Px(), 0, 0).Children(func() { it.Leading(c) })
	case it.Icon != nil:
		color := p.Icon
		if active {
			color = p.List.ActiveSelectionText
		}
		ui.Icon(c, it.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(color).Shrink(0).Margin(0, token.GapRowIcon.Px(), 0, 0)
	}
}

func label(c *ui.Context, p *token.Palette, it Item, active bool) {
	color, secondary, match := p.Text.Primary, p.Text.Secondary, p.List.Highlight
	switch {
	case active:
		color, secondary, match = p.List.ActiveSelectionText, p.List.ActiveSelectionSecondary, p.List.FocusHighlight
	case it.Muted:
		color = p.Git.Ignored
	case it.Git != token.GitNone:
		color = p.GitColor(it.Git)
	}
	ui.Row(c).Grow(1).MinWidth(0).Gap(token.GapRowIcon.Px()).AlignItems(ui.Baseline).Children(func() {
		name := ui.RichText(c, Spans(it.Label, it.Matches, color, match)...).FontSize(token.TextBody.Px()).SingleLine().Shrink(0).MinWidth(0)
		if it.Strike {
			name.Strikethrough()
		}
		if it.Description != "" {
			ui.Text(c, it.Description).FontSize(token.TextLabel.Px()).TextColor(secondary).SingleLine().Shrink(1).MinWidth(0)
		}
	})
}

func guides(row ui.Element, depth int, color ui.Color) {
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		for i := range depth {
			x := r.X + token.PadRow.Px() + float32(i)*token.IndentStep.Px() + token.TwistieWidth.Px()/2
			p.Fill(ui.Rect{X: x, Y: r.Y, W: token.IndentGuideWidth.Px(), H: r.H}, color, 0)
		}
	})
}
