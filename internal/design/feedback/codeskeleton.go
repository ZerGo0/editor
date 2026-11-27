package feedback

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	codeLines              = 14
	codeFontSize   float32 = 13
	codeLineHeight float32 = 1.5
	codeIndent             = "    "
	codeWordsMin           = 2
	codeWordsSpan          = 4
)

var codeIndents = []int{0, 1, 1, 2, 2, 1, 0, 0, 1, 2, 3, 2, 1, 0}

func CodeSkeleton(c *ui.Context, label string) ui.Element {
	return Skeleton(c, label, func() {
		ui.Column(c).FillWidth().Padding(token.Space2.Px(), token.PadView.Px()).Children(func() {
			for i := range codeLines {
				line := strings.Repeat(codeIndent, codeIndents[i%len(codeIndents)]) + Placeholder(i, codeWordsMin+i%codeWordsSpan)
				ui.Text(c.Key(i), line).FontSize(codeFontSize).Height(codeFontSize * codeLineHeight).SingleLine()
			}
		})
	})
}
