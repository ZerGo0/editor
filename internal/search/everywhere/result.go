package everywhere

import (
	"github.com/ZerGo0/editor/internal/search/symbols"
)

type Kind int

const (
	KindFile Kind = iota
	KindSymbol
	KindAction
)

type Result struct {
	Kind            Kind
	Label           string
	Detail          string
	Keys            string
	Positions       []int
	DetailPositions []int
	Path            string
	Line            int
	Col             int
	HasPos          bool
	Symbol          symbols.Kind
	Container       string
	ActionID        string
	Open            bool
	Recent          bool
	score           int
}
