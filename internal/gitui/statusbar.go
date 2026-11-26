package gitui

import (
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func (g *Git) StatusItems(c *ui.Context) {
	if !g.ready() || g.snap.status == nil {
		return
	}
	if st := g.snap.op; st.Kind != git.StateNone {
		title := opTitle(st)
		chrome.StatusItem(c, title, chrome.StatusOptions{Icon: icons.Warning, Tooltip: "Git Operation " + title, Tone: token.ToneWarning, Menu: g.operationMenu, ID: "git-op"})
	}
	name := g.headLabel()
	chrome.StatusItem(c, name, chrome.StatusOptions{Icon: icons.GitBranch, Tooltip: "Git Branch " + name, OnClick: g.openBranches, ID: "git-branch"})
	st := g.snap.status
	switch {
	case g.busy != "":
		chrome.StatusItem(c, g.busy+"...", chrome.StatusOptions{Icon: icons.Refresh, Tooltip: g.busyLabel(), ID: "git-sync"})
	case st.Upstream != "":
		chrome.StatusItem(c, syncLabel(st.Ahead, st.Behind), chrome.StatusOptions{Icon: icons.Refresh, Tooltip: "Synchronize Changes", OnClick: g.syncChanges, ID: "git-sync"})
	case len(g.snap.remotes) > 0 && !st.Detached && st.Branch != "":
		chrome.StatusItem(c, "", chrome.StatusOptions{Icon: icons.Push, Tooltip: "Publish Branch", OnClick: g.openPush, ID: "git-sync"})
	}
}
func (g *Git) operationMenu(m *ui.Menu) {
	busy := g.busy != ""
	conflicts := g.conflictCount()
	if m.Item("Resolve Conflicts...").Disabled(conflicts == 0).Chosen() {
		g.openConflicts()
	}
	if m.Item("Continue").Disabled(busy || conflicts > 0).Chosen() {
		g.continueOp()
	}
	if m.Item("Skip").Disabled(busy || g.snap.op.Kind == git.StateMerging).Chosen() {
		g.skipOp()
	}
	if m.Item("Abort").Danger().Disabled(busy).Chosen() {
		g.abortOp()
	}
}
