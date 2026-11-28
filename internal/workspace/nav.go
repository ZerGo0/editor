package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/workspace/navhistory"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	bigJumpLines   = 10
	maxHistoryMenu = 15
	maxGroupRecent = 50
)

type navState struct {
	groups   map[int]*navGroup
	lastEdit *session.Location
	moving   bool
}

type navGroup struct {
	hist    navhistory.History
	recent  []string
	last    session.Location
	tab     int
	version uint64
}

func (w *Workspace) navOf(g *layout.Group) *navGroup {
	if w.nav.groups == nil {
		w.nav.groups = map[int]*navGroup{}
	}
	n := w.nav.groups[g.ID]
	if n == nil {
		n = &navGroup{}
		w.nav.groups[g.ID] = n
	}
	return n
}

func tabLocation(t *layout.Tab) session.Location {
	loc := session.Location{Path: t.Path}
	switch {
	case t.Editor != nil:
		pos := t.Editor.CursorPos()
		loc.Line, loc.Col = pos.Line, pos.Col
	case t.Restore != nil:
		loc.Line, loc.Col = t.Restore.Line, t.Restore.Col
	}
	return loc
}

func (w *Workspace) trackNavigation() {
	g := w.active
	t := g.ActiveTab()
	if t == nil {
		return
	}
	n := w.navOf(g)
	loc := tabLocation(t)
	switch {
	case w.nav.moving:
	case n.tab != 0 && n.tab != t.ID:
		n.hist.Record(n.last)
	case n.tab == t.ID && abs(loc.Line-n.last.Line) >= bigJumpLines:
		n.hist.Record(n.last)
	}
	w.nav.moving = false
	if n.tab != t.ID {
		n.recent = append([]string{t.Path}, slices.DeleteFunc(n.recent, func(p string) bool { return p == t.Path })...)
		n.recent = n.recent[:min(len(n.recent), maxGroupRecent)]
		n.version = 0
	}
	n.tab, n.last = t.ID, loc
	if t.Editor == nil || t.Editor.Buffer == nil {
		return
	}
	if v := t.Editor.Buffer.Version(); v != n.version {
		if n.version != 0 {
			w.nav.lastEdit = &loc
		}
		n.version = v
	}
}

func abs(n int) int {
	return max(n, -n)
}

func (w *Workspace) mruTab(g *layout.Group) int {
	for _, p := range w.navOf(g).recent {
		if i := g.Find(p); i >= 0 {
			return i
		}
	}
	return g.Active
}

func (w *Workspace) CommandEnabled(id string) bool {
	switch id {
	case navigateBackID:
		return w.canNavigate(true)
	case navigateFwdID:
		return w.canNavigate(false)
	case compareFilesID:
		return w.canCompareWith(w.selectedFile())
	}
	return true
}

func (w *Workspace) canNavigate(back bool) bool {
	h := &w.navOf(w.active).hist
	if back {
		return h.CanBack()
	}
	return h.CanForward()
}

func (w *Workspace) NavigateBack() {
	w.navigate(true, 1)
}

func (w *Workspace) NavigateForward() {
	w.navigate(false, 1)
}

func (w *Workspace) NavigateToLastEdit() {
	if l := w.nav.lastEdit; l != nil {
		n := w.navOf(w.active)
		n.hist.Record(n.last)
		w.goToLocation(w.active, *l)
	}
}

func (w *Workspace) navigate(back bool, steps int) {
	n := w.navOf(w.active)
	current := n.last
	if t := w.active.ActiveTab(); t != nil {
		current = tabLocation(t)
	}
	move := n.hist.Forward
	if back {
		move = n.hist.Back
	}
	if loc, ok := move(current, steps); ok {
		w.goToLocation(w.active, loc)
	}
}

func (w *Workspace) goToLocation(g *layout.Group, loc session.Location) {
	if _, err := os.Stat(loc.Path); onDisk(loc.Path) && err != nil {
		w.navOf(g).hist.Map(func(l session.Location) (session.Location, bool) { return l, l.Path != loc.Path })
		return
	}
	w.nav.moving = true
	t := w.openIn(g, loc.Path, false, true)
	pos := editor.Pos{Line: loc.Line, Col: loc.Col}
	if t.Editor != nil && loc.Line < t.Editor.Buffer.LineCount() {
		t.Editor.GoTo(pos)
		return
	}
	if t.Editor == nil {
		t.Restore = &layout.Position{Line: loc.Line, Col: loc.Col, Reveal: true}
	}
}

func (w *Workspace) historyMenu(m *ui.Menu, back bool) {
	h := &w.navOf(w.active).hist
	list := h.ForwardList()
	if back {
		list = h.BackList()
	}
	for i, l := range list[:min(len(list), maxHistoryMenu)] {
		label := fmt.Sprintf("%s:%d", w.tabTitle(l.Path), l.Line+1)
		item := m.Item(label)
		if dir := w.rel(filepath.Dir(l.Path)); onDisk(l.Path) && dir != "." {
			item.Trailing(dir)
		}
		if item.Chosen() {
			w.navigate(back, i+1)
		}
	}
}

func (w *Workspace) snapshotNav(g *layout.Group, sg *session.Group) {
	n := w.navOf(g)
	back, fwd := n.hist.Snapshot()
	sg.Back, sg.Forward = w.relLocations(back), w.relLocations(fwd)
	for _, p := range n.recent {
		if w.restorable(p) {
			sg.Recent = append(sg.Recent, w.rel(p))
		}
	}
}

func (w *Workspace) restoreNav(g *layout.Group, sg *session.Group) {
	n := w.navOf(g)
	n.hist.Restore(w.absLocations(sg.Back), w.absLocations(sg.Forward))
	for _, p := range sg.Recent {
		n.recent = append(n.recent, w.abs(p))
	}
}

func (w *Workspace) relLocations(in []session.Location) []session.Location {
	var out []session.Location
	for _, l := range in {
		if w.restorable(l.Path) {
			out = append(out, session.Location{Path: w.rel(l.Path), Line: l.Line, Col: l.Col})
		}
	}
	return out
}

func (w *Workspace) absLocations(in []session.Location) []session.Location {
	out := make([]session.Location, len(in))
	for i, l := range in {
		out[i] = session.Location{Path: w.abs(l.Path), Line: l.Line, Col: l.Col}
	}
	return out
}

func (w *Workspace) lastEditSnapshot() *session.Location {
	l := w.nav.lastEdit
	if l == nil || !w.restorable(l.Path) {
		return nil
	}
	return &session.Location{Path: w.rel(l.Path), Line: l.Line, Col: l.Col}
}
