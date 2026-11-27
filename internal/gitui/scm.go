package gitui

import (
	"context"
	"path"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	paneChanges       = "changes"
	paneGraph         = "graph"
	changesPaneWeight = 2
	graphPaneWeight   = 1
)

type changesState struct {
	list     ui.ListState
	sel      ui.Selection[string]
	cursor   int
	closed   map[string]bool
	listMode bool
	stack    pane.StackState
	entries  []changeEntry
	source   *git.Status
	fresh    bool
}

func (s *changesState) current(st *git.Status) []changeEntry {
	if !s.fresh || s.source != st {
		s.entries = flattenChanges(changeGroups(st), !s.listMode, s.closed)
		s.source, s.fresh = st, true
	}
	return s.entries
}

func (s *changesState) toggle(key string) {
	if s.closed == nil {
		s.closed = map[string]bool{}
	}
	s.closed[key] = !s.closed[key]
	s.fresh = false
}

func (s *changesState) setOpen(key string, open bool) {
	if s.closed == nil {
		s.closed = map[string]bool{}
	}
	if s.closed[key] == !open {
		return
	}
	s.closed[key] = !open
	s.fresh = false
}

func (s *changesState) setListMode(on bool) {
	s.listMode, s.fresh = on, false
}

func (g *Git) SideView(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		if g.gate(c) {
			g.scmView(c)
		}
	})
}

func (g *Git) gate(c *ui.Context) bool {
	switch g.phase {
	case phaseLoading:
		changesSkeleton(c, "Loading repository")
	case phaseNoGit:
		emptyState(c, "Git is not installed", "Install Git, then reopen this folder.")
	case phaseNoRepo:
		feedback.Empty(c, feedback.EmptyOptions{
			Title:    "This folder is not a Git repository",
			Message:  "Initialize a repository to track changes, or open a folder that has one.",
			Action:   "Initialize Repository",
			OnAction: g.initRepository,
		})
	case phaseError:
		errorState(c, "Cannot open the repository", g.openErr, g.open)
	default:
		return g.repo != nil
	}
	return false
}

func (g *Git) busyLabel() string {
	if g.progress != "" {
		return g.busy + ": " + g.progress
	}
	return g.busy + "..."
}

func (g *Git) scmView(c *ui.Context) {
	if g.busy != "" {
		feedback.ProgressBar(c, g.busyLabel()).Absolute().Top(token.Space0.Px()).Left(token.Space0.Px()).Right(token.Space0.Px()).
			ZIndex(token.LayerSticky.Z()).Tooltip(g.busyLabel())
	}
	g.operationBanner(c)
	ui.Column(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
		pane.Stack(c, &g.changes.stack, []pane.Pane{
			{ID: paneChanges, Title: "Changes", Body: g.changesPane},
			{ID: paneGraph, Title: "Graph", Actions: g.graphActions, Busy: g.graph.loading, Body: g.graphPane},
		})
	})
}

func (g *Git) changesPane(c *ui.Context) {
	st := g.snap.status
	switch {
	case st == nil && g.snap.err != nil:
		errorState(c, "Cannot read the status", g.snap.err, g.refreshNow)
		return
	case st == nil:
		changesSkeleton(c, "Loading changes")
		return
	}
	g.commitBox(c)
	if g.snap.err != nil {
		errorText(c, g.snap.err.Error()).Padding(token.Space0.Px(), token.PadView.Px())
	}
	g.changesList(c)
}

func (g *Git) changesList(c *ui.Context) {
	s := &g.changes
	entries := s.current(g.snap.status)
	if len(entries) == 1 && len(entries[0].rows) == 0 {
		emptyState(c, "No changes", "Edit files in this folder to see them here.")
		return
	}
	s.list.Key = func(i int) any { return entries[i].key }
	s.list.Label = func(i int) string { return entries[i].label }
	s.list.Selection = &s.sel
	s.list.Selected = &s.cursor
	l := list.View(c.Key("changes"), &s.list, len(entries), list.ViewOptions{Label: "Changes", Tree: true}, func(i int) list.Item {
		return g.changeItem(c, entries[i])
	})
	if s.cursor < 0 || s.cursor >= len(entries) {
		return
	}
	e := entries[s.cursor]
	switch {
	case l.Submitted() && e.kind == entryFile:
		g.openRowDiffWith(e.rows[0], false, true)
	case l.Submitted():
		s.toggle(e.key)
	case e.kind != entryFile && l.Shortcut(0, ui.KeyLeft):
		s.setOpen(e.key, false)
	case e.kind != entryFile && l.Shortcut(0, ui.KeyRight):
		s.setOpen(e.key, true)
	}
}

func (g *Git) changeItem(c *ui.Context, e changeEntry) list.Item {
	s := &g.changes
	it := list.Item{Label: e.label, Depth: e.depth, Menu: func(m *ui.Menu) { g.changeMenu(c, m, e) }}
	switch e.kind {
	case entryGroup:
		it.Branch, it.Open, it.Strong, it.Count = true, e.open, true, len(e.rows)
		it.Actions = func(c *ui.Context) { g.groupActions(c, e) }
		it.OnClick = func() { s.toggle(e.key) }
	case entryDir:
		it.Branch, it.Open, it.Tooltip = true, e.open, e.path
		it.Leading = func(c *ui.Context) { icons.FileIcon(c, path.Base(e.path), true, e.open, token.IconSize.Px()) }
		it.Actions = func(c *ui.Context) { g.rowActions(c, e) }
		it.OnClick = func() { s.toggle(e.key) }
	default:
		r := e.rows[0]
		code := r.code()
		it.Description, it.Git, it.Strike = e.path, gitStatus(code), code == codeDeleted
		it.Tooltip = r.file.Path + " \u2022 " + statusName(code)
		it.Leading = func(c *ui.Context) { icons.FileIcon(c, e.label, false, false, token.IconSize.Px()) }
		it.Actions = func(c *ui.Context) { g.rowActions(c, e) }
		it.OnClick = func() { g.openRowDiffWith(r, true, false) }
		it.OnDoubleClick = func() { g.openRowDiffWith(r, false, true) }
	}
	return it
}

func (g *Git) targets(e changeEntry) []changeRow {
	s := &g.changes
	if s.sel.Has(e.key) && s.sel.Len() > 1 {
		return selectedRows(s.entries, s.sel.Has)
	}
	return e.rows
}

func (g *Git) rowActions(c *ui.Context, e changeEntry) {
	busy := g.busy != ""
	file := e.kind == entryFile
	switch e.group {
	case groupMerge:
		if file && iconButton(c, "Open Merge Editor", icons.Merge, busy) {
			g.openMergeTool(e.rows[0].file.Path)
		}
		if file && iconButton(c, "Open File", icons.File, false) {
			g.openFile(e.rows[0])
		}
		if iconButton(c, "Stage Changes", icons.Plus, busy) {
			g.stage(rowPaths(rowsIn(g.targets(e), groupMerge))...)
		}
	case groupStaged:
		if file && iconButton(c, "Open File", icons.File, false) {
			g.openFile(e.rows[0])
		}
		if iconButton(c, "Unstage Changes", icons.Minus, busy) {
			g.unstage(rowPaths(rowsIn(g.targets(e), groupStaged))...)
		}
	default:
		if file && iconButton(c, "Open File", icons.File, e.rows[0].code() == codeDeleted) {
			g.openFile(e.rows[0])
		}
		if iconButton(c, "Discard Changes", icons.Undo, busy) {
			g.rollback(rowsIn(g.targets(e), groupChanges))
		}
		if iconButton(c, "Stage Changes", icons.Plus, busy) {
			g.stage(rowPaths(rowsIn(g.targets(e), groupChanges))...)
		}
	}
}

func (g *Git) groupActions(c *ui.Context, e changeEntry) {
	busy := g.busy != "" || len(e.rows) == 0
	switch e.group {
	case groupMerge:
		if iconButton(c, "Resolve Conflicts...", icons.Merge, busy) {
			g.openConflicts()
		}
		if iconButton(c, "Stage All Merge Changes", icons.Plus, busy) {
			g.stage(rowPaths(e.rows)...)
		}
	case groupStaged:
		if iconButton(c, "Open Staged Changes", icons.Preview, len(e.rows) == 0) {
			g.openGroupChanges(groupStaged)
		}
		if iconButton(c, "Unstage All Changes", icons.Minus, busy) {
			g.unstage(rowPaths(e.rows)...)
		}
	default:
		if iconButton(c, "Open Changes", icons.Preview, len(e.rows) == 0) {
			g.openGroupChanges(groupChanges)
		}
		if iconButton(c, "Discard All Changes", icons.Undo, busy) {
			g.rollback(e.rows)
		}
		if iconButton(c, "Stage All Changes", icons.Plus, busy) {
			g.stage(rowPaths(e.rows)...)
		}
	}
}

func (g *Git) openFile(r changeRow) {
	g.h.OpenAt(g.paths.abs(r.file.Path), -1, -1)
}

func (g *Git) openGroupChanges(gr group) {
	g.openGroupAt(gr, "", false, true)
}

func (g *Git) stage(paths ...string) {
	if len(paths) == 0 {
		return
	}
	g.doSimple("Stage", func(ctx context.Context, r repo) error { return r.StageFiles(ctx, paths...) }, nil)
}

func (g *Git) unstage(paths ...string) {
	if len(paths) == 0 {
		return
	}
	g.doSimple("Unstage", func(ctx context.Context, r repo) error { return r.UnstageFiles(ctx, paths...) }, nil)
}

func (g *Git) openRowDiff(r changeRow) {
	g.openRowDiffWith(r, false, true)
}

func (g *Git) openRowDiffWith(r changeRow, preview, focus bool) {
	switch {
	case r.group == groupMerge:
		g.openMergeTool(r.file.Path)
	case r.group == groupStaged:
		g.openDiffWith(diffSource{kind: diffStaged, path: r.file.Path, oldPath: r.file.OrigPath}, preview, focus)
	case r.untracked():
		g.openDiffWith(diffSource{kind: diffUntracked, path: r.file.Path}, preview, focus)
	default:
		g.openDiffWith(diffSource{kind: diffWorkTree, path: r.file.Path}, preview, focus)
	}
}
