package palette

import (
	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/egoist/mygo/ui"
)

const (
	panelWidth     = 680
	panelMaxHeight = 460
	panelRadius    = 10
	rowHeight      = 30
	rowRadius      = 5
	keysFontSize   = 12
	matchWeight    = 700
	shadowBlur     = 24
	shadowY        = 8
	shadowAlpha    = 0.25
	iconSize       = 16
	hintFontSize   = 11
	hintGap        = 14
	hintHeight     = 22
	defaultHint    = "Enter to open"
	moveHint       = "↑↓ to move"
	closeHint      = "Esc to close"
)

type Item struct {
	ID        string
	Label     string
	Detail    string
	Keys      string
	Positions []int
	Value     any
}

type Tab struct {
	Placeholder string
	Search      func(query string) []Item
	Hint        string
}

type Palette struct {
	Icon     func(c *ui.Context, item Item)
	open     bool
	title    string
	tab      Tab
	query    string
	searched string
	stale    bool
	items    []Item
	selected int
	list     ui.ListState
	accept   func(Item)
}

func Static(items []Item) func(string) []Item {
	return func(query string) []Item {
		rs := fuzzy.Filter(query, len(items), func(i int) string { return items[i].Label })
		out := make([]Item, len(rs))
		for i, r := range rs {
			out[i] = items[r.Index]
			out[i].Positions = r.Match.Positions
		}
		return out
	}
}

func (p *Palette) Show(title string, tab Tab) {
	p.Close()
	p.open, p.title, p.tab, p.query, p.stale = true, title, tab, "", true
}

func (p *Palette) Title() string {
	return p.title
}

func (p *Palette) IsOpen() bool {
	return p.open
}

func (p *Palette) Close() {
	p.open, p.tab, p.items = false, Tab{}, nil
}

func (p *Palette) Move(d int) {
	if !p.open {
		return
	}
	if p.stale || p.query != p.searched {
		p.search()
	}
	p.move(d)
}

func (p *Palette) Accept() {
	if !p.open || p.selected < 0 || p.selected >= len(p.items) || p.accept == nil {
		return
	}
	item := p.items[p.selected]
	p.Close()
	p.accept(item)
}

func (p *Palette) search() {
	p.searched, p.stale = p.query, false
	p.items, p.selected = p.tab.Search(p.query), -1
	p.move(1)
	p.list.ScrollTo(0, ui.Start)
}

func (p *Palette) move(d int) {
	if len(p.items) == 0 {
		p.selected = -1
		return
	}
	p.selected = (p.selected + d + len(p.items)) % len(p.items)
	p.list.ScrollIntoView(p.selected)
}

func (p *Palette) View(c *ui.Context, top float32, accept func(Item)) {
	if !p.open {
		return
	}
	if p.stale || p.query != p.searched {
		p.search()
	}
	t := c.Theme()
	p.accept = accept
	choose := func(i int) {
		item := p.items[i]
		p.Close()
		accept(item)
	}
	ui.Overlay(c, func() {
		back := ui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Center).Modal()
		back.Children(func() {
			panel := ui.Column(c).Margin(top, 0, 0, 0).Width(panelWidth).MaxHeight(panelMaxHeight).Padding(6).Gap(4).
				Background(t.Background).Radius(panelRadius).BorderWidth(1).BorderColor(t.Border).
				Shadow(0, shadowY, shadowBlur, 0, ui.RGBA(0, 0, 0, shadowAlpha)).Role(ui.RoleDialog).Label(p.title)
			panel.Children(func() {
				p.inputRow(c, choose)
				if p.open {
					p.rows(c, t, choose)
					p.hints(c, t)
				}
			})
			if p.open && (panel.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape)) {
				p.Close()
			}
		})
	})
}

func (p *Palette) inputRow(c *ui.Context, choose func(int)) {
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		input := ui.TextInput(c, &p.query).AutoFocus().Grow(1).MinWidth(0).Placeholder(p.tab.Placeholder).Label(p.title)
		switch {
		case input.Shortcut(0, ui.KeyDown):
			p.move(1)
		case input.Shortcut(0, ui.KeyUp):
			p.move(-1)
		case input.Submitted() && p.selected >= 0:
			choose(p.selected)
		}
	})
}

func (p *Palette) rows(c *ui.Context, t *ui.Theme, choose func(int)) {
	list := ui.List(c, &p.list, len(p.items), func(i int) {
		item := p.items[i]
		row := ui.Row(c).Height(rowHeight).Padding(0, 10).Gap(12).AlignItems(ui.Center).FillWidth().Radius(rowRadius).Cursor(ui.CursorPointer)
		if i == p.selected {
			row.Background(t.Selection)
		} else if row.Hovered() {
			row.Background(t.SurfaceHover)
		}
		selected := i == p.selected
		row.Children(func() {
			if p.Icon != nil {
				p.Icon(c, item)
			}
			ui.RichText(c, Highlight(item.Label, item.Positions, t, selected)...).SingleLine().Shrink(0).MaxWidth(panelWidth / 2)
			if item.Detail != "" {
				ui.Text(c, item.Detail).TextColor(t.TextMuted).SingleLine().Grow(1).Shrink(1).MinWidth(0)
			} else {
				ui.Row(c).Grow(1)
			}
			if item.Keys != "" {
				ui.Text(c, item.Keys).FontSize(keysFontSize).TextColor(t.TextMuted).SingleLine().Shrink(0)
			}
		})
		if row.Clicked() {
			choose(i)
		}
	}).Grow(1).Shrink(1).FillWidth()
	list.Children(func() {
		if len(p.items) > 0 {
			return
		}
		ui.Row(c).Padding(8, 10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "No matching results").TextColor(t.TextMuted)
		})
	})
}

func (p *Palette) hints(c *ui.Context, t *ui.Theme) {
	hint := p.tab.Hint
	if hint == "" {
		hint = defaultHint
	}
	ui.Row(c).FillWidth().Height(hintHeight).Padding(0, 10).Gap(hintGap).AlignItems(ui.Center).Shrink(0).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		for _, h := range []string{moveHint, hint, closeHint} {
			ui.Text(c, h).FontSize(hintFontSize).TextColor(t.TextMuted).SingleLine()
		}
	})
}

func Highlight(label string, positions []int, t *ui.Theme, selected bool) []ui.Span {
	if len(positions) == 0 {
		return []ui.Span{{Text: label}}
	}
	var spans []ui.Span
	runes := []rune(label)
	next, start := 0, 0
	flush := func(end int, matched bool) {
		if end <= start {
			return
		}
		s := ui.Span{Text: string(runes[start:end])}
		if matched {
			s.Weight, s.Color = matchWeight, t.Accent
			if selected {
				s.Color = t.Text
			}
		}
		spans = append(spans, s)
		start = end
	}
	for i := range runes {
		matched := next < len(positions) && positions[next] == i
		if matched {
			flush(i, false)
			for next < len(positions) && positions[next] == i {
				next++
			}
			start = i
			flush(i+1, true)
		}
	}
	flush(len(runes), false)
	return spans
}
