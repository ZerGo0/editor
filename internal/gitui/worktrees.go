package gitui

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const lockedReason = "locked"

type worktreeState struct {
	listState[git.Worktree]
	adding    bool
	path      string
	branch    string
	newBranch bool
}

func (g *Git) openWorktrees() {
	if !g.ready() {
		return
	}
	g.worktrees = worktreeState{listState: openList[git.Worktree]()}
	g.loadWorktrees()
}

func (g *Git) openNewWorktree(branch string) {
	if !g.ready() {
		return
	}
	g.worktrees = worktreeState{listState: openList[git.Worktree](), adding: true, branch: branch}
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
	gen, r := s.begin(), g.repo
	applog.Go(func() {
		items, err := r.Worktrees(g.ctx)
		g.post(func() { s.apply(gen, items, err) })
	})
}

func (g *Git) worktreesDialog(c *ui.Context) {
	s := &g.worktrees
	if s.open && !g.ready() {
		s.open = false
	}
	modal(c, &s.open, "Worktrees", token.DialogWidthWide, func() {
		if s.adding {
			g.worktreeForm(c)
			return
		}
		switch {
		case s.err != nil:
			errorState(c, "Cannot list the worktrees", s.err, g.loadWorktrees)
		case s.loading && s.items == nil:
			boxedSkeleton(c, "Loading worktrees", func(i int) list.Item {
				return worktreeItem(git.Worktree{Path: placeholderPath(i), Branch: placeholderWord(i)}, i == 0)
			})
		default:
			g.worktreeList(c)
		}
		w, ok := s.selectedWorktree()
		main := ok && s.selected == 0
		dialog.Actions(c, func() {
			if button(c, "New Worktree...", false) {
				s.adding, s.path, s.branch, s.newBranch = true, g.suggestWorktreePath(""), "", false
			}
			if button(c, "Copy Path", !ok) {
				c.WriteClipboard(w.Path)
			}
			if button(c, "Remove", !ok || main || g.busy != "") {
				g.removeWorktree(w)
			}
			design.Spacer(c)
			if button(c, "Close", false) {
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
	boxedList(c, "Worktrees", &s.list, len(items), func(i int) list.Item { return worktreeItem(items[i], i == 0) })
}

func worktreeItem(w git.Worktree, main bool) list.Item {
	label := w.Branch
	if w.Detached {
		label = shortHash(w.HEAD)
	}
	it := list.Item{Label: w.Path, Icon: icons.Folder, Strong: main, Muted: w.Prunable, Tooltip: w.Path}
	it.Trailing = func(c *ui.Context) {
		secondary(c, label).SingleLine()
		if w.Locked {
			badge.Tag(c, lockedReason, badge.TagOptions{Tone: token.ToneWarning, Icon: icons.Lock})
		}
	}
	return it
}

func (s *worktreeState) selectedWorktree() (git.Worktree, bool) {
	if s.selected < 0 || s.selected >= len(s.items) {
		return git.Worktree{}, false
	}
	return s.items[s.selected], true
}

func (g *Git) worktreeForm(c *ui.Context) {
	s := &g.worktrees
	path, branch := strings.TrimSpace(s.path), strings.TrimSpace(s.branch)
	problem := ""
	if path != "" && !filepath.IsAbs(path) {
		problem = "Enter an absolute path."
	}
	field.TextInput(c, "Worktree Location", &s.path, field.InputOptions{Placeholder: "Location", Error: problem, AutoFocus: true, ID: "wt-path"})
	field.TextInput(c, "Worktree Branch", &s.branch, field.InputOptions{Placeholder: "Branch or commit", ID: "wt-branch"})
	action.Checkbox(c, "Create a new branch with this name", &s.newBranch, action.ChoiceOptions{})
	disabled := !filepath.IsAbs(path) || (branch != "" && checkName(branch) != nil) || (s.newBranch && branch == "") || g.busy != ""
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
