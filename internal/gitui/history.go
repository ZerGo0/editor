package gitui

import (
	"context"
	"fmt"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

func (g *Git) activeRel() (string, bool) {
	if !g.ready() {
		return "", false
	}
	rel, ok := g.paths.rel(g.h.ActivePath())
	if !ok {
		g.h.Notify("Open a file of this repository first.")
	}
	return rel, ok
}

func (g *Git) fileHistory(rel string) {
	if !g.ready() || rel == "" {
		return
	}
	id := "git-history:" + rel
	s, ok := g.histories[id]
	switch {
	case !ok:
		s = newLogState(g, rel)
		g.histories[id] = s
	case s.started:
		s.reload()
	}
	name, _ := splitPath(rel)
	g.h.OpenView(id, "History: "+name, func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			if g.gate(c) {
				s.view(c)
			}
		})
	})
}

func (s *logState) open(cm git.Commit) {
	if s.fixedPath == "" {
		s.g.openCommitTab(cm)
		return
	}
	src := diffSource{kind: diffCommit, path: s.fixedPath, rev: cm.OID}
	if len(cm.Parents) > 0 {
		src.parent = cm.Parents[0]
	}
	s.g.openDiffSource(src)
}

func (g *Git) selectionHistory() {
	rel, ok := g.activeRel()
	if !ok {
		return
	}
	st := g.h.ActiveEditor()
	if st == nil || st.Buffer == nil {
		g.h.Notify("Open a file of this repository first.")
		return
	}
	sel := st.Selection()
	from, to := st.Buffer.Pos(sel.Start()), st.Buffer.Pos(sel.End())
	end := to.Line
	if !sel.Empty() && to.Col == 0 && end > from.Line {
		end--
	}
	start, last := from.Line+1, end+1
	name, _ := splitPath(rel)
	id := fmt.Sprintf("git-lines:%s:%d-%d", rel, start, last)
	title := fmt.Sprintf("History: %s:%d-%d", name, start, last)
	g.openPatchTab(id, title, func(ctx context.Context, r repo) ([]patchItem, error) {
		entries, err := r.LineLog(ctx, rel, start, last)
		if err != nil {
			return nil, err
		}
		var items []patchItem
		for _, e := range entries {
			cm := e.Commit
			items = append(items, patchItem{title: cm.Subject, commit: &cm})
			items = append(items, fileDiffItems(e.Diffs)...)
		}
		return items, nil
	})
}
