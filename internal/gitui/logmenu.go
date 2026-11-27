package gitui

import (
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

func (s *logState) commitMenu(c *ui.Context, m *ui.Menu, cm git.Commit) {
	s.g.commitActions(c, m, cm)
	m.Separator()
	root := len(cm.Parents) == 0
	if m.Item("Go to Parent Commit").Disabled(root).Chosen() {
		s.focusCommit(cm.Parents[0])
	}
	if m.Item("Go to Child Commit").Disabled(s.childOf(cm.OID) < 0).Chosen() {
		s.selected = s.childOf(cm.OID)
		s.list.ScrollIntoView(s.selected)
	}
}

func (g *Git) commitActions(c *ui.Context, m *ui.Menu, cm git.Commit) {
	busy := g.busy != ""
	root := len(cm.Parents) == 0
	merge := len(cm.Parents) > 1
	head := g.snap.status != nil && g.snap.status.OID == cm.OID
	if m.Item("Copy Revision Number").Chosen() {
		c.WriteClipboard(cm.OID)
	}
	if m.Item("Create Patch").Chosen() {
		g.copyCommitPatch(cm)
	}
	if m.Item("Show Changes").Chosen() {
		g.openCommitTab(cm)
	}
	if m.Item("Compare with Local").Chosen() {
		g.compareWithLocal(cm.OID)
	}
	m.Separator()
	if m.Item("Cherry-Pick").Disabled(busy).Chosen() {
		g.cherryPick(cm)
	}
	if m.Item("Checkout Revision").Disabled(busy).Chosen() {
		g.checkout(cm.OID, shortHash(cm.OID))
	}
	m.Submenu("Reset Current Branch to Here", func(m *ui.Menu) {
		for _, mode := range resetModes {
			if m.Item(mode.label).Subtitle(mode.detail).Disabled(busy).Chosen() {
				g.resetTo(cm.OID, mode)
			}
		}
	})
	if m.Item("Revert Commit").Disabled(busy).Chosen() {
		g.revertCommit(cm)
	}
	if m.Item("Undo Commit").Disabled(busy || !head || root || merge).Chosen() {
		g.undoCommit(cm)
	}
	if m.Item("Edit Commit Message...").Shortcut(0, ui.KeyF2).Disabled(busy || root || merge).Chosen() {
		g.rewordCommit(cm)
	}
	if m.Item("Drop Commit").Danger().Disabled(busy || root || merge).Chosen() {
		g.dropCommit(cm)
	}
	if m.Item("Interactively Rebase from Here...").Disabled(busy || root || merge).Chosen() {
		g.openRebase(cm.Parents[0])
	}
	if m.Item("Push All up to Here...").Disabled(busy).Chosen() {
		g.pushUpTo(cm)
	}
	m.Separator()
	if m.Item("Create Branch...").Chosen() {
		g.newBranch(cm.OID)
	}
	if m.Item("Create Tag...").Chosen() {
		g.newTag(cm.OID)
	}
}

func (s *logState) childOf(oid string) int {
	at := -1
	for i, c := range s.commits {
		if c.OID == oid {
			at = i
			break
		}
	}
	for i := at - 1; i >= 0; i-- {
		for _, p := range s.commits[i].Parents {
			if p == oid {
				return i
			}
		}
	}
	return -1
}
