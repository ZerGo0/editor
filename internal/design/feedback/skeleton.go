package feedback

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	skeletonKey         = "skeleton"
	placeholderInk      = "x"
	placeholderCapStart = "\uE000"
	placeholderCapEnd   = "\uE001"
)

var placeholderWords = []int{7, 4, 9, 3, 6, 11, 5, 8, 2, 6, 10, 4}

func Skeleton(c *ui.Context, label string, body func()) ui.Element {
	p := token.Of(c)
	wrap := ui.Column(c).FillWidth().MinHeight(0).Clip()
	if p.IsSkeleton() {
		return wrap.Children(body)
	}
	sk := p.Skeleton()
	wrap.Role(ui.RoleProgress).Busy(true).Label(label)
	shown := pastDelay(c, wrap)
	wrap.Children(func() {
		inner := ui.Themed(c.Key(skeletonKey), sk.Theme()).Background(ui.Transparent).FillWidth().Grow(1).MinHeight(0).Font(token.FontSkeleton).
			IgnoreInput(true).AccessHidden(true)
		if !shown {
			inner.Invisible()
		}
		inner.Children(body)
	})
	return wrap
}

func Placeholder(seed, words int) string { return placeholder(seed, words, placeholderInk) }

func placeholder(seed, words int, ink string) string {
	n := 0
	for i := range words {
		n += placeholderWords[(seed*words+i)%len(placeholderWords)]
	}
	return placeholderCapStart + strings.Repeat(ink, n) + placeholderCapEnd
}
