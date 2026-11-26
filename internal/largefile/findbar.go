package largefile

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

const (
	findGap       = 8
	findPadY      = 6
	findPadX      = 12
	findPrefill   = 256
	statusSearch  = "Searching..."
	statusNone    = "No matches"
	statusChanged = "The text changed; search again"
)

type finder struct {
	open       bool
	matchCase  bool
	query      string
	status     string
	focusInput bool
	seq        uint64
	cancel     context.CancelFunc

	mu     sync.Mutex
	result *findResult
}

type findResult struct {
	seq     uint64
	version uint64
	start   int64
	end     int64
	found   bool
	err     error
}

func (s *State) openFind() {
	if a, b := s.selection(); a < b && b-a <= findPrefill {
		if text, err := s.Doc.Slice(a, b); err == nil && !strings.Contains(text, "\n") {
			s.find.query = text
		}
	}
	s.find.open = true
	s.find.focusInput = true
}

func (s *State) closeFind() {
	s.stopSearch()
	s.find.open = false
	s.find.status = ""
	s.focus.Focus()
}

func (s *State) stopSearch() {
	if s.find.cancel != nil {
		s.find.cancel()
		s.find.cancel = nil
	}
}

func (s *State) findBar(c *ui.Context) {
	t := c.Theme()
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyG):
		s.search(false)
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyG):
		s.search(true)
	}
	ui.Row(c).Gap(findGap).Padding(findPadY, findPadX).AlignItems(ui.Center).Shrink(0).FillWidth().
		Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		in := ui.TextInput(c, &s.find.query).Grow(1).Placeholder("Find").Label("Find")
		if s.find.focusInput {
			s.find.focusInput = false
			in.Focus()
		}
		switch {
		case in.Submitted():
			s.search(c.Modifiers()&ui.Shift != 0)
		case in.Shortcut(0, ui.KeyEscape):
			s.closeFind()
		}
		ui.Checkbox(c, &s.find.matchCase, "Match case")
		if ui.Button(c, "Previous").Clicked() {
			s.search(true)
		}
		if ui.Button(c, "Next").Clicked() {
			s.search(false)
		}
		if ui.Button(c, "Done").Clicked() {
			s.closeFind()
		}
	})
}

func (s *State) search(backward bool) {
	query := s.find.query
	if query == "" {
		s.openFind()
		return
	}
	s.stopSearch()
	ctx, cancel := context.WithCancel(context.Background())
	s.find.cancel = cancel
	s.find.seq++
	s.find.status = statusSearch
	a, b := s.selection()
	from := b
	if backward {
		from = a
	}
	req := findResult{seq: s.find.seq, version: s.Doc.Version()}
	doc, services, matchCase := s.Doc, s.services, s.find.matchCase
	go func() {
		r := req
		r.start, r.end, r.found, r.err = doc.Find(ctx, from, query, matchCase, backward)
		if r.err == nil && !r.found {
			wrap := int64(0)
			if backward {
				wrap = doc.Len()
			}
			r.start, r.end, r.found, r.err = doc.Find(ctx, wrap, query, matchCase, backward)
		}
		s.find.mu.Lock()
		s.find.result = &r
		s.find.mu.Unlock()
		services.Invalidate()
	}()
}

func (s *State) applyFound() {
	s.find.mu.Lock()
	r := s.find.result
	s.find.result = nil
	s.find.mu.Unlock()
	if r == nil || r.seq != s.find.seq {
		return
	}
	s.find.cancel = nil
	switch {
	case errors.Is(r.err, context.Canceled):
		s.find.status = ""
	case r.err != nil:
		s.find.status = r.err.Error()
	case !r.found:
		s.find.status = statusNone
	case r.version != s.Doc.Version():
		s.find.status = statusChanged
	default:
		s.find.status = ""
		s.Anchor, s.Caret = r.start, r.end
		s.hasGoal = false
		s.reveal()
	}
}
