package redline

import (
	"math"
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
)

type namedSpace struct {
	name  string
	space token.Space
}

var spacing = []namedSpace{
	{"token.GapTight", token.GapTight},
	{"token.GapInline", token.GapInline},
	{"token.GapRowIcon", token.GapRowIcon},
	{"token.GapControl", token.GapControl},
	{"token.PadRow", token.PadRow},
	{"token.PadPanel", token.PadPanel},
	{"token.GapGroup", token.GapGroup},
	{"token.GapSection", token.GapSection},
	{"token.PadView", token.PadView},
	{"token.PadEmpty", token.PadEmpty},
}

const maxTokenNames = 3

func spaceTokens(v float64) string {
	var names []string
	for _, s := range spacing {
		if math.Abs(float64(s.space.Px())-v) < eps && len(names) < maxTokenNames {
			names = append(names, s.name)
		}
	}
	if len(names) == 0 {
		return num(v) + " px (no spacing token has this value)"
	}
	return strings.Join(names, " or ")
}
