package palette

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/quickinput"
	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/egoist/mygo/ui"
)

type Action struct {
	Icon  *ui.SVG
	Label string
	Run   func()
}

type Item struct {
	ID        string
	Label     string
	Detail    string
	Keys      string
	Positions []int
	Value     any
	Icon      *ui.SVG
	Leading   func(c *ui.Context)
	Action    *Action
	Group     string
}

type Mode struct {
	Label       string
	Placeholder string
	Empty       string
	Search      func(query string, done func([]Item))
	Accept      func(Item)
	AcceptAlt   func(Item)
}

type Palette struct {
	open     bool
	mode     *Mode
	state    quickinput.State
	searched string
	seq      int
	items    []Item
}

func Static(items []Item) func(string, func([]Item)) {
	return func(query string, done func([]Item)) { done(Filter(items, query)) }
}

func Filter(items []Item, query string) []Item {
	rs := fuzzy.Filter(query, len(items), func(i int) string { return items[i].Label })
	out := make([]Item, len(rs))
	for i, r := range rs {
		out[i] = items[r.Index]
		out[i].Positions = r.Match.Positions
	}
	return out
}

func (p *Palette) Show(m Mode) {
	p.open, p.mode = true, &m
	p.state.Reset("")
	p.refresh()
}

func (p *Palette) IsOpen() bool {
	return p.open
}

func (p *Palette) Label() string {
	if p.open && p.mode != nil {
		return p.mode.Label
	}
	return ""
}

func (p *Palette) Close() {
	p.open, p.mode, p.items = false, nil, nil
	p.seq++
}

func (p *Palette) Move(delta int) {
	if p.open {
		p.state.Move(delta, len(p.items))
	}
}

func (p *Palette) Accept() {
	if !p.open || p.state.Selected < 0 || p.state.Selected >= len(p.items) {
		return
	}
	p.accept(p.items[p.state.Selected], false)
}

func (p *Palette) accept(it Item, alt bool) {
	m := p.mode
	p.Close()
	switch {
	case m == nil:
	case alt && m.AcceptAlt != nil:
		m.AcceptAlt(it)
	case m.Accept != nil:
		m.Accept(it)
	}
}

func (p *Palette) refresh() {
	p.searched = p.state.Query
	p.seq++
	if p.mode == nil || p.mode.Search == nil {
		p.items = nil
		return
	}
	seq := p.seq
	query := strings.TrimLeft(p.state.Query, " ")
	p.mode.Search(query, func(items []Item) {
		if seq == p.seq && p.open {
			p.items = items
		}
	})
}

func (p *Palette) View(c *ui.Context) {
	if !p.open {
		return
	}
	if p.state.Query != p.searched {
		p.refresh()
	}
	m := p.mode
	label, o := "", quickinput.Options{}
	if m != nil {
		label, o = m.Label, quickinput.Options{Placeholder: m.Placeholder, Empty: m.Empty}
	}
	items := make([]quickinput.Item, len(p.items))
	for i, it := range p.items {
		items[i] = quickinput.Item{Label: it.Label, Matches: it.Positions, Description: it.Detail, Icon: it.Icon, Leading: it.Leading, Keys: it.Keys}
		items[i].Group = it.Group
		if it.Action != nil {
			items[i].Action = &quickinput.Action{Icon: it.Action.Icon, Label: it.Action.Label}
		}
	}
	res := quickinput.View(c, label, &p.state, items, o)
	if p.state.Query != p.searched {
		c.Invalidate()
	}
	switch {
	case res.Pressed >= 0 && res.Pressed < len(p.items):
		if a := p.items[res.Pressed].Action; a != nil && a.Run != nil {
			a.Run()
		}
		p.refresh()
	case res.Accepted >= 0 && res.Accepted < len(p.items):
		p.accept(p.items[res.Accepted], res.Alt)
	case res.Closed:
		p.Close()
	}
}
