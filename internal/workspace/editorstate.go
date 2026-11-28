package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const maxClosedViewStates = 50

func (w *Workspace) tabState(t *layout.Tab) session.Tab {
	st := session.Tab{Path: w.rel(t.Path), Preview: t.Preview, Pinned: t.Pinned}
	if isVirtual(t.Path) {
		st.Title = w.tabTitle(t.Path)
	}
	switch {
	case t.Editor != nil:
		p := t.Editor.CursorPos()
		st.Line, st.Col, st.ScrollY = p.Line, p.Col, t.Editor.ScrollY()
		st.Folds = t.Editor.Folded()
		if sels := t.Editor.Selections(); len(sels) > 1 || len(sels) == 1 && !sels[0].Empty() {
			st.Selections = make([][2]int, len(sels))
			for i, s := range sels {
				st.Selections[i] = [2]int{s.Anchor, s.Head}
			}
		}
	case t.Restore != nil && !t.Restore.Reveal:
		r := t.Restore
		st.Line, st.Col, st.ScrollY, st.Folds = r.Line, r.Col, r.ScrollY, r.Folds
		for _, s := range r.Selections {
			st.Selections = append(st.Selections, [2]int{s.Anchor, s.Head})
		}
	}
	return st
}

func restorePosition(st session.Tab) *layout.Position {
	r := &layout.Position{Line: st.Line, Col: st.Col, ScrollY: st.ScrollY, Folds: st.Folds}
	for _, s := range st.Selections {
		r.Selections = append(r.Selections, editor.Selection{Anchor: s[0], Head: s[1]})
	}
	return r
}

func applyPosition(ed *editor.State, buf *editor.Buffer, r *layout.Position) {
	if r.Line >= buf.LineCount() {
		return
	}
	for _, line := range r.Folds {
		if line < buf.LineCount() {
			ed.Fold(line)
		}
	}
	switch {
	case len(r.Selections) > 0:
		ed.SetSelections(r.Selections...)
	default:
		ed.GoTo(editor.Pos{Line: r.Line, Col: min(r.Col, len(buf.Line(r.Line)))})
	}
	if r.End != nil {
		ed.Select(buf.Offset(editor.Pos{Line: r.Line, Col: r.Col}), buf.Offset(*r.End))
	}
	if !r.Reveal {
		ed.SetScrollY(r.ScrollY)
	}
}

func (w *Workspace) rememberViewState(t *layout.Tab) {
	if isVirtual(t.Path) || isUntitled(t.Path) || t.Editor == nil && (t.Restore == nil || t.Restore.Reveal) {
		return
	}
	st := w.tabState(t)
	st.Preview, st.Pinned = false, false
	w.closedViews = append(slices.DeleteFunc(w.closedViews, func(c session.Tab) bool { return c.Path == st.Path }), st)
	if over := len(w.closedViews) - maxClosedViewStates; over > 0 {
		w.closedViews = slices.Delete(w.closedViews, 0, over)
	}
}

func (w *Workspace) takeViewState(path string) *layout.Position {
	rel := w.rel(path)
	i := slices.IndexFunc(w.closedViews, func(c session.Tab) bool { return c.Path == rel })
	if i < 0 {
		return nil
	}
	st := w.closedViews[i]
	w.closedViews = slices.Delete(w.closedViews, i, i+1)
	return restorePosition(st)
}

func (w *Workspace) snapshotClosedViews() []session.Tab {
	return slices.Clone(w.closedViews)
}

func (w *Workspace) observeStateInput(ev ui.InputEvent) {
	switch ev.Kind {
	case ui.InputKeyDown, ui.InputText, ui.InputPointerUp, ui.InputScroll:
		w.stateTouched = true
	}
}

func (w *Workspace) publishStateChange() {
	if w.stateTouched && w.OnStateChange != nil {
		w.stateTouched = false
		w.OnStateChange()
	}
}

func (w *Workspace) trackPaneSizes() {
	for key, s := range map[string]*pane.StackState{explorerSizesKey: &w.explorerPanes, githubSizesKey: &w.githubPanes} {
		w.views.Load(key, &s.Weights)
		w.views.Track(key, func() any { return s.Weights })
	}
}
