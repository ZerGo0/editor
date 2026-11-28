package gitui

import (
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	jumpToSourceKey  = ui.KeyF4
	showDiffMods     = ui.Super
	showDiffKey      = ui.KeyD
	copyRevisionMods = ui.Super | ui.Alt | ui.Shift
	copyRevisionKey  = ui.KeyC
)

func (g *Git) commitFileKeys(l ui.Element, cm git.Commit, f git.ChangedFile) {
	switch {
	case l.Shortcut(0, jumpToSourceKey):
		g.jumpToSource(f.Path)
	case l.Shortcut(showDiffMods, showDiffKey):
		g.openDiffWith(commitSource(cm, f), false, true)
	}
}

func copyRevisionShortcut(c *ui.Context, l ui.Element, cm git.Commit) {
	if l.Shortcut(copyRevisionMods, copyRevisionKey) {
		c.WriteClipboard(cm.OID)
	}
}
