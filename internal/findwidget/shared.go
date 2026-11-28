package findwidget

import (
	"math"

	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui/editor"
)

type Shared struct {
	MatchCase bool    `json:"matchCase,omitempty"`
	WholeWord bool    `json:"wholeWord,omitempty"`
	Regex     bool    `json:"regex,omitempty"`
	Width     int     `json:"width,omitempty"`
	History   History `json:"history,omitzero"`
}

func options(f *editor.FindBar, sh *Shared) (caseOn, wordOn, regexOn *bool) {
	if sh == nil {
		return f.CaseSensitive(), f.WholeWord(), f.Regexp()
	}
	return &sh.MatchCase, &sh.WholeWord, &sh.Regex
}

func (sh *Shared) apply(f *editor.FindBar) {
	if sh != nil {
		*f.CaseSensitive(), *f.WholeWord(), *f.Regexp() = sh.MatchCase, sh.WholeWord, sh.Regex
	}
}

func (sh *Shared) remember(query string) {
	if sh != nil {
		sh.History.Add(query)
	}
}

func (sh *Shared) history() *History {
	if sh == nil {
		return nil
	}
	return &sh.History
}

func (sh *Shared) inputWidth() float32 {
	if sh == nil || sh.Width <= 0 {
		return token.FindInputWidth.Px()
	}
	return max(float32(sh.Width), token.FindInputWidth.Px())
}

func (sh *Shared) resize(current float32, r pane.SashResult) {
	switch {
	case r.Reset:
		sh.Width = 0
	case r.Delta != 0:
		sh.Width = int(max(whole(current-r.Delta), token.FindInputWidth.Px()))
	}
}

func whole(v float32) float32 { return float32(math.Round(float64(v))) }
