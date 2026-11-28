package list

import (
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/egoist/mygo/ui"
)

const SkeletonRows = 6

func Skeleton(c *ui.Context, label string, n int, item func(i int) Item) ui.Element {
	return feedback.Skeleton(c, label, func() { Rows(c, n, item) })
}

func Rows(c *ui.Context, n int, item func(i int) Item) {
	for i := range n {
		it := item(i)
		Row(c, State{Depth: it.Depth, Branch: it.Branch, Open: it.Open}, it)
	}
}
