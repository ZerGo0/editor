package list

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type Item struct {
	Label         string
	Matches       []int
	Description   string
	Aside         string
	Icon          *ui.SVG
	Leading       func(c *ui.Context)
	Git           token.GitStatus
	Count         int
	Strike        bool
	Muted         bool
	Strong        bool
	Spans         []ui.Span
	Rich          func(s Style) []ui.Span
	Depth         int
	Branch        bool
	Open          bool
	Pills         []Pill
	LeadingWidth  float32
	TrailingWidth float32
	Trailing      func(c *ui.Context)
	Content       func(c *ui.Context)
	Tone          token.Tone
	Tooltip       string
	Actions       func(c *ui.Context)
	Menu          func(m *ui.Menu)
	OnClick       func()
	OnDoubleClick func()
	Drop          func(row ui.Element) bool
}

type Style struct {
	Text      ui.Color
	Secondary ui.Color
	Highlight ui.Color
}

type State struct {
	Selected bool
	Focused  bool
	Hovered  bool
	Depth    int
	Width    float32
	Guides   bool
	Branch   bool
	Open     bool
	arrow    func() ui.Element
	tree     bool
	flat     bool
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
	showDescription := s.showDescription(c, it)
	shown, dropped := s.fitPills(c, it, showDescription)
	if it.Tooltip != "" {
		tip := it.Tooltip
		if it.Description != "" && !showDescription {
			tip += "\n" + it.Description
		}
		if len(dropped) > 0 {
			tip += "\n" + droppedNames(dropped)
		}
		action.Tooltip(c, row, tip)
	}
	if (s.tree || s.flat) && s.Guides && s.Depth > 0 {
		guides(row, s.Depth, p.List.IndentGuide)
	}
	row.Children(func() {
		if s.tree || s.flat {
			twistie(c, p, s)
		}
		leading(c, p, it, active)
		label(c, p, it, active, showDescription)
		if len(shown) > 0 {
			ui.Row(c).Gap(token.GapInline.Px()).AlignItems(ui.Center).Shrink(0).Margin(0, 0, 0, token.GapInline.Px()).Children(func() {
				for _, t := range shown {
					badge.Tag(c, t.text, t.opts)
				}
			})
		}
		aside(c, p, it, active)
		if it.Trailing != nil {
			ui.Row(c).Gap(token.GapInline.Px()).AlignItems(ui.Center).Shrink(0).Children(func() { it.Trailing(c) })
		}
		if it.Actions != nil && (hovered || active) {
			ui.Row(c).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).Children(func() { it.Actions(c) })
		}
		switch {
		case it.Count > 0 && it.Tone != token.ToneNeutral:
			toneCount(c, p, it, active)
		case it.Count > 0:
			badge.Count(c, it.Count, badge.CountOptions{})
		}
		if it.Git != token.GitNone {
			badge.Decoration(c, it.Git)
		}
	})
	if s.flat {
		row.Role(ui.RoleTreeItem).Level(s.Depth + 1)
		if s.Branch {
			row.Expanded(s.Open)
		}
	}
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
	var arrow ui.Element
	switch {
	case s.arrow != nil:
		arrow = s.arrow()
	case s.flat:
		arrow = ui.Box(c)
	default:
		return
	}
	arrow.Size(token.TwistieWidth.Px(), token.RowHeight.Px()).Center().Shrink(0)
	if !s.Branch {
		return
	}
	target := float32(0)
	if s.Open {
		target = token.TwistieOpenAngle
	}
	arrow.Children(func() {
		ui.Icon(c, icons.ChevronRight).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon).Rotate(target)
	})
}

func leading(c *ui.Context, p *token.Palette, it Item, active bool) {
	switch {
	case it.Leading != nil:
		ui.Row(c).Shrink(0).AlignItems(ui.Center).Margin(0, token.GapRowIcon.Px(), 0, 0).Children(func() { it.Leading(c) })
	case it.Icon != nil && p.IsSkeleton():
		icons.Placeholder(c, token.IconSize.Px()).Margin(0, token.GapRowIcon.Px(), 0, 0)
	case it.Icon != nil:
		color := p.Icon
		if active {
			color = p.List.ActiveSelectionText
		}
		ui.Icon(c, it.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(color).Shrink(0).Margin(0, token.GapRowIcon.Px(), 0, 0)
	}
}

func label(c *ui.Context, p *token.Palette, it Item, active, showDescription bool) {
	if it.Content != nil {
		ui.Row(c).Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Children(func() { it.Content(c) })
		return
	}
	color, secondary, match := p.Text.Primary, p.Text.Secondary, p.List.Highlight
	switch {
	case active:
		color, secondary, match = p.List.ActiveSelectionText, p.List.ActiveSelectionSecondary, p.List.FocusHighlight
	case it.Tone != token.ToneNeutral:
		color = p.Tone(it.Tone)
	case it.Muted:
		color = p.Git.Ignored
	case it.Git != token.GitNone:
		color = p.GitColor(it.Git)
	}
	ui.Row(c).Grow(1).MinWidth(0).Gap(token.GapRowIcon.Px()).AlignItems(ui.Baseline).Children(func() {
		spans := it.Spans
		if it.Rich != nil {
			spans = it.Rich(Style{Text: color, Secondary: secondary, Highlight: match})
		}
		if spans == nil {
			spans = Spans(it.Label, it.Matches, color, match)
		}
		name := token.TextBody.Apply(ui.RichText(c, spans...)).SingleLine().Shrink(1).MinWidth(0)
		if it.Strong {
			name.FontWeight(token.WeightSemibold)
		}
		if it.Strike {
			name.Strikethrough()
		}
		if showDescription {
			token.TextLabel.Apply(ui.Text(c, it.Description)).TextColor(secondary).SingleLine().Shrink(descriptionShrink).MinWidth(0)
		}
	})
}

func aside(c *ui.Context, p *token.Palette, it Item, active bool) {
	if it.Aside == "" {
		return
	}
	color := p.Text.Secondary
	if active {
		color = p.List.ActiveSelectionSecondary
	}
	token.TextLabel.Apply(ui.Text(c, it.Aside)).TextColor(color).SingleLine().Shrink(1).MinWidth(0).MaxWidthPercent(token.AsideMaxPercent).
		Margin(0, 0, 0, token.GapControl.Px())
}

func guides(row ui.Element, depth int, color ui.Color) {
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		for i := range depth {
			x := r.X + token.PadRow.Px() + float32(i)*token.IndentStep.Px() + token.TwistieWidth.Px()/2
			p.Fill(ui.Rect{X: x, Y: r.Y, W: token.IndentGuideWidth.Px(), H: r.H}, color, 0)
		}
	})
}
