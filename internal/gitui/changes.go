package gitui

import (
	"context"
	"strconv"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const statusWidth = 12

type group int

const (
	groupConflicts group = iota
	groupStaged
	groupUnstaged
	groupUntracked
	groupCount
)

var groupTitles = [groupCount]string{"Merge Conflicts", "Staged", "Changes", "Unversioned Files"}

type changeRow struct {
	header bool
	group  group
	count  int
	file   git.FileStatus
}

func (r changeRow) code() byte {
	switch r.group {
	case groupConflicts:
		return 'U'
	case groupStaged:
		return r.file.Index
	case groupUntracked:
		return '?'
	}
	return r.file.WorkTree
}

func (r changeRow) key() string {
	if r.header {
		return "h" + strconv.Itoa(int(r.group))
	}
	return strconv.Itoa(int(r.group)) + ":" + r.file.Path
}

func (r changeRow) label() string {
	if r.header {
		return groupTitles[r.group]
	}
	return r.file.Path
}

type changesState struct {
	list      ui.ListState
	selected  int
	collapsed [groupCount]bool
}

func buildRows(st *git.Status, collapsed [groupCount]bool) []changeRow {
	if st == nil {
		return nil
	}
	groups := [groupCount][]git.FileStatus{st.Conflicted(), st.Staged(), st.Unstaged(), st.Untracked()}
	var rows []changeRow
	for gi, files := range groups {
		if len(files) == 0 {
			continue
		}
		rows = append(rows, changeRow{header: true, group: group(gi), count: len(files)})
		if collapsed[gi] {
			continue
		}
		for _, f := range files {
			rows = append(rows, changeRow{group: group(gi), file: f})
		}
	}
	return rows
}

func groupPaths(st *git.Status, gr group) []string {
	var files []git.FileStatus
	switch gr {
	case groupConflicts:
		files = st.Conflicted()
	case groupStaged:
		files = st.Staged()
	case groupUnstaged:
		files = st.Unstaged()
	case groupUntracked:
		files = st.Untracked()
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}

func (g *Git) changesList(c *ui.Context) {
	s := &g.changes
	rows := buildRows(g.snap.status, s.collapsed)
	s.list.Key = func(i int) any { return rows[i].key() }
	s.list.Header = func(i int) bool { return rows[i].header }
	s.list.Label = func(i int) string { return rows[i].label() }
	s.list.Selected = &s.selected
	list := ui.List(c.Key("changes"), &s.list, len(rows)).Grow(1).FillWidth().MinHeight(0).Label("Changes")
	list.Rows(func(row ui.ListRow) { g.changeRowView(row, rows[row.Index]) })
	list.Children(func() {
		if len(rows) == 0 {
			emptyState(c, "No changes", "Edit files in this folder to see them here.")
		}
	})
	if list.Submitted() && s.selected >= 0 && s.selected < len(rows) && !rows[s.selected].header {
		g.openRowDiff(rows[s.selected])
	}
}

func (g *Git) changeRowView(row ui.ListRow, r changeRow) {
	c := row.Context
	if r.header {
		g.groupHeader(c, r)
		return
	}
	t := c.Theme()
	name, dir := splitPath(r.file.Path)
	code := r.code()
	el := ui.Row(c).Height(rowHeight).FillWidth().Padding(0, rowPadding, 0, rowPadding+statusWidth).Gap(6).AlignItems(ui.Center).
		Tooltip(statusName(code) + ": " + r.file.Path)
	el.Children(func() {
		ui.Text(c, statusLetter(code)).FontSize(smallFont).Bold().TextColor(statusColor(t, code)).Width(statusWidth).Shrink(0)
		text := ui.Text(c, name).FontSize(bodyFont).SingleLine().Shrink(0)
		if code == 'D' {
			text.Strikethrough()
		}
		if dir != "" {
			muted(c, dir).SingleLine().Shrink(1).MinWidth(0).TruncateStart()
		}
		ui.Box(c).Grow(1)
		if row.Hovered() || row.Selected() {
			g.rowActions(c, r)
		}
	})
	if el.RowClicked() {
		g.openRowDiff(r)
	}
	el.ContextMenu(func(m *ui.Menu) { g.fileMenu(c, m, r) })
}

func (g *Git) rowActions(c *ui.Context, r changeRow) {
	busy := g.busy != ""
	switch r.group {
	case groupStaged:
		if iconButton(c, "Unstage "+r.file.Path, icons.Minus, busy).Clicked() {
			g.unstage(r.file.Path)
		}
	case groupUnstaged, groupUntracked:
		if iconButton(c, "Rollback "+r.file.Path, icons.Undo, busy).Clicked() {
			g.rollback([]changeRow{r})
		}
		if iconButton(c, "Stage "+r.file.Path, icons.Plus, busy).Clicked() {
			g.stage(r.file.Path)
		}
	case groupConflicts:
		if textButton(c, "Merge...", busy).Clicked() {
			g.openMergeTool(r.file.Path)
		}
	}
}

func (g *Git) groupHeader(c *ui.Context, r changeRow) {
	t := c.Theme()
	s := &g.changes
	ui.Row(c).Height(rowHeight).FillWidth().Padding(0, rowPadding, 0, 2).Gap(4).AlignItems(ui.Center).Background(t.Background).Children(func() {
		chevron := icons.ChevronDown
		if s.collapsed[r.group] {
			chevron = icons.ChevronRight
		}
		title := groupTitles[r.group]
		if iconButton(c, "Toggle "+title, chevron, false).Clicked() {
			s.collapsed[r.group] = !s.collapsed[r.group]
		}
		sectionTitle(c, title)
		muted(c, strconv.Itoa(r.count))
		ui.Box(c).Grow(1)
		busy := g.busy != ""
		switch r.group {
		case groupStaged:
			if iconButton(c, "Unstage All", icons.Minus, busy).Clicked() {
				g.unstage(groupPaths(g.snap.status, groupStaged)...)
			}
		case groupUnstaged:
			if iconButton(c, "Stage All Changes", icons.Plus, busy).Clicked() {
				g.stage(groupPaths(g.snap.status, groupUnstaged)...)
			}
		case groupUntracked:
			if iconButton(c, "Stage All Unversioned Files", icons.Plus, busy).Clicked() {
				g.stage(groupPaths(g.snap.status, groupUntracked)...)
			}
		}
	})
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
	switch r.group {
	case groupStaged:
		g.openDiff(diffStaged, r.file.Path)
	case groupUntracked:
		g.openDiff(diffUntracked, r.file.Path)
	case groupConflicts:
		g.openMergeTool(r.file.Path)
	default:
		g.openDiff(diffWorkTree, r.file.Path)
	}
}
