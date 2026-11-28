package list

import (
	"math"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	TitleShare        = 0.6
	descriptionShrink = 8
	pillSeparator     = ", "
	widthTolerance    = 0.5
)

type Pill struct {
	Text    string
	Short   string
	Icon    *ui.SVG
	Tone    token.Tone
	Tooltip string
}

type pillTag struct {
	text string
	opts badge.TagOptions
}

func ObserveWidth(c *ui.Context, list ui.Element) float32 {
	used := list.Bounds().W
	list.Draw(func(_ *ui.Painter, r ui.Rect) {
		if math.Abs(float64(r.W-used)) > widthTolerance {
			c.Invalidate()
		}
	})
	return used
}

func (p Pill) tag(compact bool) pillTag {
	text := p.Text
	if compact {
		text = p.Short
	}
	return pillTag{text: text, opts: badge.TagOptions{Tone: p.Tone, Icon: p.Icon, Tooltip: p.Tooltip, MaxWidth: token.PillNameMaxWidth.Px()}}
}

func tags(pills []Pill, compact bool) []pillTag {
	out := make([]pillTag, len(pills))
	for i, p := range pills {
		out[i] = p.tag(compact)
	}
	return out
}

func tagsWidth(c *ui.Context, tags []pillTag) float32 {
	var w float32
	for _, t := range tags {
		w += badge.TagWidth(c, t.text, t.opts) + token.GapInline.Px()
	}
	return w
}

func (s State) fixedWidth(it Item) float32 {
	w := 2*token.PadRow.Px() + it.LeadingWidth + it.TrailingWidth
	if s.tree || s.flat {
		w += float32(s.Depth)*token.IndentStep.Px() + token.TwistieWidth.Px()
	}
	if it.Git != token.GitNone {
		w += token.DecorationWidth.Px()
	}
	if it.Leading != nil || it.Icon != nil {
		w += token.GapRowIcon.Px()
	}
	if it.Icon != nil {
		w += token.IconSize.Px()
	}
	return w
}

func titleWidth(c *ui.Context, it Item) float32 {
	weight := token.TextBody.Weight()
	if it.Strong {
		weight = token.WeightSemibold
	}
	w, _ := c.MeasureText(0, ui.Span{Text: it.Label, Size: token.TextBody.Px(), Weight: weight})
	return w
}

func descriptionWidth(c *ui.Context, it Item) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: it.Description, Size: token.TextLabel.Px(), Weight: token.TextLabel.Weight()})
	return w + token.GapRowIcon.Px()
}

func (s State) showDescription(c *ui.Context, it Item) bool {
	if it.Description == "" {
		return false
	}
	if s.Width <= 0 {
		return true
	}
	return titleWidth(c, it)+descriptionWidth(c, it) <= s.Width-s.fixedWidth(it)
}

func (s State) fitPills(c *ui.Context, it Item, description bool) (shown []pillTag, dropped []Pill) {
	if len(it.Pills) == 0 {
		return nil, nil
	}
	full := tags(it.Pills, false)
	if s.Width <= 0 {
		return full, nil
	}
	need := min(titleWidth(c, it), TitleShare*s.Width)
	room := s.Width - s.fixedWidth(it) - need
	if description {
		room -= descriptionWidth(c, it)
	}
	if tagsWidth(c, full) <= room {
		return full, nil
	}
	var iconic []Pill
	for _, p := range it.Pills {
		if p.Icon != nil {
			iconic = append(iconic, p)
		}
	}
	compact := tags(iconic, true)
	for len(compact) > 0 && tagsWidth(c, compact) > room {
		compact = compact[:len(compact)-1]
	}
	if len(compact) == 0 {
		return nil, it.Pills
	}
	return compact, droppedPills(it.Pills, iconic[:len(compact)])
}

func droppedPills(all, shown []Pill) []Pill {
	var out []Pill
	for _, p := range all {
		if !slices.ContainsFunc(shown, func(s Pill) bool { return s == p }) {
			out = append(out, p)
		}
	}
	return out
}

func droppedNames(pills []Pill) string {
	names := make([]string, len(pills))
	for i, p := range pills {
		names[i] = p.Tooltip
		if names[i] == "" {
			names[i] = p.Text
		}
	}
	return strings.Join(names, pillSeparator)
}
