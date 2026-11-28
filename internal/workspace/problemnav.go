package workspace

import (
	"cmp"
	"maps"
	"slices"

	"github.com/ZerGo0/editor/internal/problems"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui/editor"
)

const beforeFirstLine = -1

type problemSpot struct {
	rank, line, col int
}

func (s problemSpot) compare(o problemSpot) int {
	return cmp.Or(cmp.Compare(s.rank, o.rank), cmp.Compare(s.line, o.line), cmp.Compare(s.col, o.col))
}

func nextProblem(files map[string][]problems.Problem, here session.Location, step int, acrossFiles bool) (session.Location, bool) {
	var paths []string
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if len(files[p]) > 0 && (acrossFiles || p == here.Path) {
			paths = append(paths, p)
		}
	}
	var spots []problemSpot
	for rank, p := range paths {
		for _, it := range files[p] {
			spots = append(spots, problemSpot{rank, it.Line, it.Col})
		}
	}
	if len(spots) == 0 {
		return session.Location{}, false
	}
	slices.SortFunc(spots, problemSpot.compare)
	rank, found := slices.BinarySearch(paths, here.Path)
	at := problemSpot{rank, here.Line, here.Col}
	if !found {
		at.line = beforeFirstLine
	}
	pick := spots[0]
	if step < 0 {
		pick = spots[len(spots)-1]
		for _, s := range slices.Backward(spots) {
			if s.compare(at) < 0 {
				pick = s
				break
			}
		}
	} else if i := slices.IndexFunc(spots, func(s problemSpot) bool { return s.compare(at) > 0 }); i >= 0 {
		pick = spots[i]
	}
	return session.Location{Path: paths[pick.rank], Line: pick.line, Col: pick.col}, true
}

func (w *Workspace) GoToProblem(step int, acrossFiles bool) {
	t := w.active.ActiveTab()
	if t == nil {
		return
	}
	here := session.Location{Path: t.Path}
	if t.Editor != nil {
		pos := t.Editor.CursorPos()
		here.Line, here.Col = pos.Line, pos.Col
	}
	files := map[string][]problems.Problem{}
	for p, e := range w.insights.problems {
		files[p] = e.items
	}
	loc, ok := nextProblem(files, here, step, acrossFiles)
	if !ok {
		w.setNotice("No problems found.")
		return
	}
	w.goToLocation(w.active, loc)
	if t := w.active.ActiveTab(); t != nil && t.Editor != nil && t.Path == loc.Path {
		t.Editor.Run(editor.ShowHover)
	}
}
