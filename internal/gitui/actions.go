package gitui

import (
	"github.com/ZerGo0/editor/internal/git"
)

type Action struct {
	ID       string
	Title    string
	Category string
	Key      string
}

const category = "Git"

var Actions = []Action{
	{ID: "git.commit", Title: "Commit", Category: category},
	{ID: "git.commitAndPush", Title: "Commit and Push", Category: category},
	{ID: "git.amend", Title: "Toggle Amend Commit", Category: category, Key: "ctrl+alt+m"},
	{ID: "git.refresh", Title: "Refresh", Category: category},
	{ID: "git.fetch", Title: "Fetch", Category: category},
	{ID: "git.pull", Title: "Pull", Category: category},
	{ID: "git.pullRebase", Title: "Pull (Rebase)", Category: category},
	{ID: "git.update", Title: "Update Project", Category: category},
	{ID: "git.push", Title: "Push...", Category: category},
	{ID: "git.branches", Title: "Checkout to...", Category: category, Key: "ctrl+shift+`"},
	{ID: "git.newBranch", Title: "Create Branch...", Category: category, Key: "ctrl+alt+n"},
	{ID: "git.checkoutRevision", Title: "Checkout Tag or Revision...", Category: category},
	{ID: "git.merge", Title: "Merge Branch...", Category: category},
	{ID: "git.rebase", Title: "Rebase Branch...", Category: category},
	{ID: "git.newTag", Title: "Create Tag", Category: category},
	{ID: "git.remotes", Title: "Manage Remotes...", Category: category},
	{ID: "git.stash", Title: "Stash", Category: category},
	{ID: "git.stashes", Title: "Pop Stash...", Category: category},
	{ID: "git.shelve", Title: "Shelve Changes...", Category: category},
	{ID: "git.shelf", Title: "Shelf...", Category: category},
	{ID: "git.showLog", Title: "Show Log", Category: category},
	{ID: "git.fileHistory", Title: "Show History for Current File", Category: category},
	{ID: "git.selectionHistory", Title: "Show History for Selection", Category: category},
	{ID: "git.annotate", Title: "Annotate with Git Blame", Category: category},
	{ID: "git.rollback", Title: "Rollback Current File...", Category: category},
	{ID: "git.rollbackLines", Title: "Rollback Changed Lines", Category: category},
	{ID: "git.showChange", Title: "Show Change at Caret", Category: category},
	{ID: "git.stageFile", Title: "Stage Current File", Category: category},
	{ID: "git.showDiff", Title: "Show Diff with HEAD", Category: category},
	{ID: "git.compareWithBranch", Title: "Compare with Branch...", Category: category},
	{ID: "git.rebaseInteractive", Title: "Interactive Rebase...", Category: category},
	{ID: "git.resolveConflicts", Title: "Resolve Conflicts...", Category: category},
	{ID: "git.continue", Title: "Continue Operation", Category: category},
	{ID: "git.skip", Title: "Skip Commit", Category: category},
	{ID: "git.abort", Title: "Abort Operation", Category: category},
	{ID: "git.addToIgnore", Title: "Add Current File to .gitignore", Category: category},
	{ID: "git.addToExclude", Title: "Add Current File to .git/info/exclude", Category: category},
	{ID: "git.worktrees", Title: "Worktrees...", Category: category},
	{ID: "git.submodules", Title: "Submodules...", Category: category},
	{ID: "git.init", Title: "Initialize Repository", Category: category},
}

func (g *Git) Run(id string) bool {
	if id == "git.init" {
		g.initRepository()
		return true
	}
	fn, ok := g.actionTable()[id]
	if !ok {
		return false
	}
	if !g.ready() {
		g.notReady()
		return true
	}
	fn()
	return true
}

func (g *Git) notReady() {
	switch g.phase {
	case phaseNoGit:
		g.h.Notify("Git is not installed.")
	case phaseNoRepo:
		g.h.Notify("This folder is not a Git repository. Run Initialize Repository first.")
	case phaseLoading:
		g.h.Notify("The repository is still loading.")
	default:
		g.h.Notify("Cannot open the repository.")
	}
}

func (g *Git) actionTable() map[string]func() {
	return map[string]func(){
		"git.commit":            g.focusCommit,
		"git.commitAndPush":     func() { g.doCommit(commitSmart, afterPush) },
		"git.amend":             g.toggleAmend,
		"git.refresh":           g.refreshNow,
		"git.fetch":             g.fetch,
		"git.pull":              g.pull,
		"git.pullRebase":        func() { g.pullWith(git.PullOptions{Rebase: true}) },
		"git.update":            g.update,
		"git.push":              g.openPush,
		"git.branches":          g.openBranches,
		"git.newBranch":         func() { g.newBranch("") },
		"git.checkoutRevision":  g.checkoutRevision,
		"git.merge":             func() { g.openBranchPicker("Merge into "+g.headLabel(), g.mergeInto) },
		"git.rebase":            func() { g.openBranchPicker("Rebase "+g.headLabel()+" onto", g.rebaseOnto) },
		"git.newTag":            func() { g.newTag("") },
		"git.remotes":           g.openRemotes,
		"git.stash":             g.openStash,
		"git.stashes":           g.openStashes,
		"git.shelve":            func() { g.askShelve(g.changedTrackedPaths()) },
		"git.shelf":             g.openShelf,
		"git.showLog":           g.showLog,
		"git.fileHistory":       g.activeFileHistory,
		"git.selectionHistory":  g.selectionHistory,
		"git.annotate":          g.annotateActive,
		"git.rollback":          g.rollbackActive,
		"git.rollbackLines":     g.rollbackLines,
		"git.showChange":        g.showChange,
		"git.stageFile":         g.stageActive,
		"git.showDiff":          g.showActiveDiff,
		"git.compareWithBranch": g.compareWithBranch,
		"git.rebaseInteractive": g.startRebase,
		"git.resolveConflicts":  g.openConflicts,
		"git.continue":          g.continueOp,
		"git.skip":              g.skipOp,
		"git.abort":             g.abortOp,
		"git.addToIgnore":       func() { g.ignoreActive(false) },
		"git.addToExclude":      func() { g.ignoreActive(true) },
		"git.worktrees":         g.openWorktrees,
		"git.submodules":        g.openSubmodules,
	}
}

func (g *Git) toggleAmend() {
	g.commit.amend = !g.commit.amend
	if g.commit.amend {
		g.loadAmendMessage()
	}
	g.focusCommit()
}

func (g *Git) activeFileHistory() {
	if rel, ok := g.activeRel(); ok {
		g.fileHistory(rel)
	}
}

func (g *Git) annotateActive() {
	path := g.h.ActivePath()
	st := g.h.ActiveEditor()
	if path == "" || st == nil {
		g.h.Notify("Open a file of this repository first.")
		return
	}
	g.Annotate(path, st)
}

func (g *Git) statusRows(rel string) []changeRow {
	st := g.snap.status
	if st == nil {
		return nil
	}
	var rows []changeRow
	for _, f := range st.Files {
		if f.Path != rel {
			continue
		}
		switch {
		case f.Conflict:
		case f.Index == codeUntracked:
			rows = append(rows, changeRow{group: groupChanges, file: f})
		default:
			if isChanged(f.Index) {
				rows = append(rows, changeRow{group: groupStaged, file: f})
			}
			if isChanged(f.WorkTree) {
				rows = append(rows, changeRow{group: groupChanges, file: f})
			}
		}
	}
	return rows
}

func isChanged(code byte) bool {
	return code != 0 && code != ' ' && code != '.' && code != statusIgnored
}

func (g *Git) rollbackActive() {
	rel, ok := g.activeRel()
	if !ok {
		return
	}
	rows := g.statusRows(rel)
	if len(rows) == 0 {
		g.h.Notify("The current file has no changes.")
		return
	}
	g.rollback(rows)
}

func (g *Git) stageActive() {
	if rel, ok := g.activeRel(); ok {
		g.stage(rel)
	}
}

func (g *Git) ignoreActive(exclude bool) {
	rel, ok := g.activeRel()
	if !ok {
		return
	}
	if exclude {
		g.addToExclude(rel)
		return
	}
	g.addToIgnore(rel)
}

func (g *Git) compareWithBranch() {
	title := "Compare with Branch"
	g.openBranchPicker(title, func(ref string) {
		if g.h.ActivePath() != "" {
			if _, ok := g.paths.rel(g.h.ActivePath()); ok {
				g.compareFileWith(ref)
				return
			}
		}
		g.compareWithLocal(ref)
	})
}
