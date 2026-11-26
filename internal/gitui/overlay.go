package gitui

import "github.com/egoist/mygo/ui"

func (g *Git) Overlay(c *ui.Context) {
	if g.clipboard != "" {
		c.WriteClipboard(g.clipboard)
		g.clipboard = ""
	}
	g.branchesPopup(c)
	g.changePopup(c)
	g.pushDialog(c)
	g.stashDialog(c)
	g.stashesDialog(c)
	g.shelfDialog(c)
	g.rebaseDialog(c)
	g.conflictsDialog(c)
	g.worktreesDialog(c)
	g.submodulesDialog(c)
	g.remotesDialog(c)
	g.newBranchDialog(c)
	g.tagDialog(c)
}
