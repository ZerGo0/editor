package ghshared

import (
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const detailIDParts = 4

type DetailRef struct {
	Owner, Name string
	Number      int
}

func ParseDetailID(prefix, id string) (DetailRef, bool) {
	parts := strings.Split(id, "/")
	if len(parts) != detailIDParts || parts[0] != prefix {
		return DetailRef{}, false
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil || n <= 0 {
		return DetailRef{}, false
	}
	return DetailRef{Owner: parts[1], Name: parts[2], Number: n}, true
}

func (s *Session[C]) RestoreDetails(prefix string, repo func() github.Remote, otherRepo error, show func(c *ui.Context, number int)) {
	s.Host.RestoreViews(prefix+"/", func(id, _ string) (func(c *ui.Context), bool) {
		ref, ok := ParseDetailID(prefix, id)
		if !ok {
			return nil, false
		}
		return func(c *ui.Context) { s.restoredDetail(c, repo(), ref, otherRepo, show) }, true
	})
}

func (s *Session[C]) restoredDetail(c *ui.Context, repo github.Remote, ref DetailRef, otherRepo error, show func(c *ui.Context, number int)) {
	switch {
	case s.Closed:
	case s.Phase == PhaseLoading:
		detail.Page(c, func() { DetailLoading(c, ref.Number) })
	case s.Phase != PhaseReady:
		detail.Page(c, func() { StateView(c, s.StateMessage(repo.Host), s.Connect).Grow(1) })
	case !strings.EqualFold(repo.Owner, ref.Owner) || !strings.EqualFold(repo.Name, ref.Name):
		detail.Page(c, func() { DetailLoadFailed(c, ref.Number, otherRepo, nil) })
	default:
		show(c, ref.Number)
	}
}
