package viewer

import (
	"time"

	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo/ui"
)

const previewDebounce = 250 * time.Millisecond

type Preview struct {
	s        *State
	pane     previewPane
	text     func() string
	started  bool
	seen     string
	dirty    bool
	polledAt time.Time
}

type previewPane interface {
	pane
	setText(text string)
}

func NewPreview(h host.Host, path string, text func() string) *Preview {
	kind := Detect(path, nil)
	if !kind.Previewable() || text == nil {
		return nil
	}
	s := newState(h, path, kind, Options{})
	var pane previewPane
	if kind == SVG {
		pane = newImageSurface(s, SVG, svgPreviewLabel)
	} else {
		pane = newMarkdownSurface(s)
	}
	s.pane = pane
	return &Preview{s: s, pane: pane, text: text}
}

func (p *Preview) View(c *ui.Context) ui.Element {
	p.poll(c)
	return p.s.View(c)
}

func (p *Preview) Handle() *ui.Handle { return p.s.Handle() }

func (p *Preview) Close() { p.s.Close() }

func (p *Preview) poll(c *ui.Context) {
	now := c.Now()
	if !p.started {
		p.started = true
		p.seen, p.polledAt = p.text(), now
		p.pane.setText(p.seen)
		return
	}
	if wait := previewDebounce - now.Sub(p.polledAt); wait > 0 {
		c.After(wait)
		return
	}
	p.polledAt = now
	switch current := p.text(); {
	case current != p.seen:
		p.seen, p.dirty = current, true
		c.After(previewDebounce)
	case p.dirty:
		p.dirty = false
		p.pane.setText(p.seen)
	}
}
