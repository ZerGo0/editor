package palette

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/quickinput"
	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/egoist/mygo/ui"
)

type Item struct {
	ID        string
	Label     string
	Detail    string
	Keys      string
	Positions []int
	Value     any
	Icon      *ui.SVG
	Leading   func(c *ui.Context)
	Group     string
	Query     string
}

type Mode struct {
	Prefix      string
	Label       string
	Placeholder string
	Empty       string
	Search      func(query string, done func([]Item))
	Accept      func(Item)
}

type Palette struct {
	Modes    []Mode
	open     bool
	single   *Mode
	state    quickinput.State
	searched string
	mode     *Mode
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

func (p *Palette) Open(query string) {
	p.open, p.single = true, nil
	p.state.Reset(query)
	p.refresh()
}

func (p *Palette) Show(m Mode) {
	p.open, p.single = true, &m
	p.state.Reset("")
	p.refresh()
}

func (p *Palette) IsOpen() bool {
	return p.open
}

func (p *Palette) Label() string {
	if m := p.current(); p.open && m != nil {
		return m.Label
	}
	return ""
}

func (p *Palette) Close() {
	p.open, p.single, p.mode, p.items = false, nil, nil, nil
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
	p.accept(p.items[p.state.Selected])
}

func (p *Palette) accept(it Item) {
	m := p.mode
	if it.Query != "" {
		p.state.Reset(it.Query)
		p.refresh()
		return
	}
	p.Close()
	if m != nil && m.Accept != nil {
		m.Accept(it)
	}
}

func (p *Palette) current() *Mode {
	if p.single != nil {
		return p.single
	}
	var best *Mode
	for i := range p.Modes {
		m := &p.Modes[i]
		if strings.HasPrefix(p.state.Query, m.Prefix) && (best == nil || len(m.Prefix) > len(best.Prefix)) {
			best = m
		}
	}
	return best
}

func (p *Palette) refresh() {
	p.searched = p.state.Query
	p.mode = p.current()
	p.seq++
	if p.mode == nil || p.mode.Search == nil {
		p.items = nil
		return
	}
	seq := p.seq
	query := strings.TrimLeft(strings.TrimPrefix(p.state.Query, p.mode.Prefix), " ")
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
		items[i] = quickinput.Item{Label: it.Label, Matches: it.Positions, Description: it.Detail, Icon: it.Icon, Leading: it.Leading, Keys: it.Keys, Group: it.Group}
	}
	res := quickinput.View(c, label, &p.state, items, o)
	if p.state.Query != p.searched {
		c.Invalidate()
	}
	switch {
	case res.Accepted >= 0 && res.Accepted < len(p.items):
		p.accept(p.items[res.Accepted])
	case res.Closed:
		p.Close()
	}
}
