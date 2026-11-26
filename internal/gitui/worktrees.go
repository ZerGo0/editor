package gitui

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const lockedReason = "locked"

type worktreeState struct {
	open      bool
	loading   bool
	err       error
	gen       int
	items     []git.Worktree
	list      ui.ListState
	selected  int
	adding    bool
	path      string
	branch    string
	newBranch bool
}

func (g *Git) openWorktrees() {
	if !g.ready() {
		return
	}
	g.worktrees = worktreeState{open: true, selected: -1}
	g.loadWorktrees()
}

func (g *Git) openNewWorktree(branch string) {
	if !g.ready() {
		return
	}
	g.worktrees = worktreeState{open: true, selected: -1, adding: true, branch: branch}
	g.worktrees.path = g.suggestWorktreePath(branch)
	g.loadWorktrees()
}

func (g *Git) suggestWorktreePath(branch string) string {
	root := g.repo.WorkDir()
	name := filepath.Base(root)
	if branch != "" {
		_, short, ok := strings.Cut(branch, "/")
		if !ok {
			short = branch
		}
		name += "-" + strings.ReplaceAll(short, "/", "-")
	}
	return filepath.Join(filepath.Dir(root), name)
}

func (g *Git) loadWorktrees() {
	s := &g.worktrees
	s.gen++
	gen, r := s.gen, g.repo
	s.loading, s.err = true, nil
	go func() {
		items, err := r.Worktrees(g.ctx)
		g.post(func() {
			if gen != s.gen {
				return
			}
			s.loading, s.err, s.items = false, err, items
		})
	}()
}

func (g *Git) worktreesDialog(c *ui.Context) {
	s := &g.worktrees
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Worktrees", wideDialog, func() {
		if s.adding {
			g.worktreeForm(c)
			return
		}
		switch {
		case s.err != nil:
			errorState(c, "Cannot list the worktrees", s.err, g.loadWorktrees)
		case s.loading && s.items == nil:
			loadingState(c, "Loading worktrees")
		default:
			g.worktreeList(c)
		}
		w, ok := s.selectedWorktree()
		main := ok && s.selected == 0
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			if ui.Button(c, "New Worktree...").Clicked() {
				s.adding, s.path, s.branch, s.newBranch = true, g.suggestWorktreePath(""), "", false
			}
			if ui.Button(c, "Copy Path").Disabled(!ok).Clicked() {
				c.WriteClipboard(w.Path)
			}
			if ui.Button(c, "Remove").Disabled(!ok || main || g.busy != "").Clicked() {
				g.removeWorktree(w)
			}
			ui.Box(c).Grow(1)
			if ui.Button(c, "Close").Clicked() {
				s.open = false
			}
		})
	})
}

func (g *Git) worktreeList(c *ui.Context) {
	s := &g.worktrees
	items := s.items
	if len(items) == 0 {
		emptyState(c, "No worktrees", "")
		return
	}
	s.list.Key = func(i int) any { return items[i].Path }
	s.list.Label = func(i int) string { return items[i].Path }
	s.list.Selected = &s.selected
	ui.List(c.Key("worktrees"), &s.list, len(items)).FillWidth().Height(listHeight).Label("Worktrees").Rows(func(row ui.ListRow) {
		w := items[row.Index]
		rc := row.Context
		ui.Row(rc).Height(rowHeight).FillWidth().Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
			path := ui.Text(rc, w.Path).FontSize(bodyFont).SingleLine().Grow(1).Shrink(1).MinWidth(0).TruncateStart()
			if row.Index == 0 {
				path.Bold()
			}
			if w.Prunable {
				path.TextColor(rc.Theme().TextMuted)
			}
			label := w.Branch
			if w.Detached {
				label = shortHash(w.HEAD)
			}
			muted(rc, label).SingleLine().Shrink(0)
			if w.Locked {
				ui.Icon(rc, icons.Lock).Size(iconSize, iconSize).TextColor(rc.Theme().Warning).Shrink(0)
				badge(rc, lockedReason, rc.Theme().Warning)
			}
		})
	})
}

func (s *worktreeState) selectedWorktree() (git.Worktree, bool) {
	if s.selected < 0 || s.selected >= len(s.items) {
		return git.Worktree{}, false
	}
	return s.items[s.selected], true
}

func (g *Git) worktreeForm(c *ui.Context) {
	s := &g.worktrees
	ui.TextInput(c.Key("wt-path"), &s.path).AutoFocus().FillWidth().Placeholder("Location").Label("Worktree Location")
	ui.TextInput(c.Key("wt-branch"), &s.branch).FillWidth().Placeholder("Branch or commit").Label("Worktree Branch")
	ui.Checkbox(c, &s.newBranch, "Create a new branch with this name")
	path, branch := strings.TrimSpace(s.path), strings.TrimSpace(s.branch)
	disabled := !filepath.IsAbs(path) || (branch != "" && checkName(branch) != nil) || (s.newBranch && branch == "") || g.busy != ""
	if path != "" && !filepath.IsAbs(path) {
		ui.Text(c, "Enter an absolute path.").FontSize(smallFont).TextColor(c.Theme().Danger)
	}
	dialogButtons(c, func() { s.adding = false }, "Create Worktree", disabled, func() {
		s.adding = false
		newBranch := s.newBranch
		g.doSimple("New Worktree", func(ctx context.Context, r repo) error {
			return r.AddWorktree(ctx, path, branch, newBranch)
		}, func() {
			g.h.Notify("Created the worktree at " + path)
			g.loadWorktrees()
		})
	})
}

func (g *Git) removeWorktree(w git.Worktree) {
	g.confirm(question{
		message:     "Remove the worktree at " + w.Path + "?",
		detail:      "Its folder is deleted. Uncommitted changes in it are lost.",
		action:      "Remove",
		destructive: true,
	}, func() {
		g.doSimple("Remove Worktree", func(ctx context.Context, r repo) error {
			return r.RemoveWorktree(ctx, w.Path, true)
		}, g.loadWorktrees)
	})
}
