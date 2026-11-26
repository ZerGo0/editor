package gitui

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func (g *Git) TitleActions(c *ui.Context) {
	if !g.ready() {
		return
	}
	if iconButton(c, "Commit Changes", icons.Check, g.commitBlocker(g.snap.status, commitSmart) != "") {
		g.doCommit(commitSmart, afterNone)
	}
	if iconButton(c, "Refresh", icons.Refresh, false) {
		g.refreshNow()
	}
	action.MenuIconButton(c, icons.More, "More Actions...", action.IconOptions{}, g.moreMenu)
}

func (g *Git) actionTitle(id string) string {
	for _, a := range Actions {
		if a.ID == id {
			return a.Title
		}
	}
	return id
}

func (g *Git) menuRun(m *ui.Menu, id string, disabled bool) {
	if m.Item(g.actionTitle(id)).Disabled(disabled).Chosen() {
		g.Run(id)
	}
}

func (g *Git) moreMenu(m *ui.Menu) {
	busy := g.busy != ""
	g.menuRun(m, "git.pull", busy)
	g.menuRun(m, "git.push", busy)
	g.menuRun(m, "git.fetch", busy)
	m.Separator()
	m.Submenu("Commit", func(m *ui.Menu) {
		st := g.snap.status
		if m.Item(commitLabel).Disabled(g.commitBlocker(st, commitSmart) != "").Chosen() {
			g.doCommit(commitSmart, afterNone)
		}
		if m.Item("Commit Staged").Disabled(g.commitBlocker(st, commitStaged) != "").Chosen() {
			g.doCommit(commitStaged, afterNone)
		}
		if m.Item("Commit All").Disabled(g.commitBlocker(st, commitAllFiles) != "").Chosen() {
			g.doCommit(commitAllFiles, afterNone)
		}
		if m.Item("Commit and Push").Disabled(g.commitBlocker(st, commitSmart) != "").Chosen() {
			g.doCommit(commitSmart, afterPush)
		}
		m.Separator()
		if m.Item("Amend").Checked(g.commit.amend).Chosen() {
			g.toggleAmend()
		}
	})
	m.Submenu("Changes", func(m *ui.Menu) {
		groups := changeGroups(g.snap.status)
		if m.Item("Stage All Changes").Disabled(busy || len(groups[groupChanges]) == 0).Chosen() {
			g.stage(rowPaths(groups[groupChanges])...)
		}
		if m.Item("Unstage All Changes").Disabled(busy || len(groups[groupStaged]) == 0).Chosen() {
			g.unstage(rowPaths(groups[groupStaged])...)
		}
		if m.Item("Discard All Changes").Danger().Disabled(busy || len(groups[groupChanges]) == 0).Chosen() {
			g.rollback(groups[groupChanges])
		}
		m.Separator()
		g.menuRun(m, "git.shelve", busy)
		g.menuRun(m, "git.shelf", false)
	})
	m.Submenu("Pull, Push", func(m *ui.Menu) {
		if m.Item("Sync").Disabled(busy).Chosen() {
			g.syncChanges()
		}
		for _, id := range []string{"git.pull", "git.pullRebase", "git.update", "git.push", "git.fetch"} {
			g.menuRun(m, id, busy)
		}
	})
	m.Submenu("Branch", func(m *ui.Menu) {
		for _, id := range []string{"git.branches", "git.newBranch", "git.merge", "git.rebase", "git.rebaseInteractive", "git.checkoutRevision", "git.compareWithBranch"} {
			g.menuRun(m, id, false)
		}
	})
	m.Submenu("Remote", func(m *ui.Menu) { g.menuRun(m, "git.remotes", false) })
	m.Submenu("Stash", func(m *ui.Menu) {
		g.menuRun(m, "git.stash", busy)
		g.menuRun(m, "git.stashes", false)
	})
	m.Submenu("Tags", func(m *ui.Menu) { g.menuRun(m, "git.newTag", false) })
	m.Separator()
	g.menuRun(m, "git.showLog", false)
	g.menuRun(m, "git.annotate", false)
	g.menuRun(m, "git.resolveConflicts", g.conflictCount() == 0)
	g.menuRun(m, "git.worktrees", false)
	g.menuRun(m, "git.submodules", false)
	m.Separator()
	if m.Item("View as List").Checked(g.changes.listMode).Chosen() {
		g.changes.setListMode(true)
	}
	if m.Item("View as Tree").Checked(!g.changes.listMode).Chosen() {
		g.changes.setListMode(false)
	}
}
