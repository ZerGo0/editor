package diffmode

import "github.com/ZerGo0/editor/internal/git"

type Options struct {
	Whitespace git.WhitespaceMode
	ShowMoves  bool
}

var Session = Options{ShowMoves: true}

type WhitespaceChoice struct {
	Mode  git.WhitespaceMode
	Label string
}

var WhitespaceChoices = []WhitespaceChoice{
	{git.WhitespaceNone, "Do not ignore"},
	{git.WhitespaceTrim, "Trim whitespaces"},
	{git.WhitespaceIgnore, "Ignore whitespaces"},
	{git.WhitespaceIgnoreBlank, "Ignore whitespaces and empty lines"},
}

func WhitespaceLabel(m git.WhitespaceMode) string {
	for _, c := range WhitespaceChoices {
		if c.Mode == m {
			return c.Label
		}
	}
	return WhitespaceChoices[0].Label
}
