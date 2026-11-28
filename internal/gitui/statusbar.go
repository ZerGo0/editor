package gitui

import (
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func (g *Git) StatusItems(c *ui.Context) {
	ready := g.ready() && g.snap.status != nil
	if ready {
		if st := g.snap.op; st.Kind != git.StateNone {
			title := opTitle(st)
			chrome.StatusItem(c, title, chrome.StatusOptions{Icon: icons.Warning, Label: "Git Operation: " + title, Tooltip: "Git Operation " + title, Tone: token.ToneWarning, Menu: g.operationMenu, ID: "git-op"})
		}
		name := g.headLabel()
		chrome.StatusItem(c, name, chrome.StatusOptions{Icon: icons.GitBranch, Label: "Git Branch: " + name, Tooltip: "Git Branch " + name, OnClick: g.openBranches, ID: "git-branch"})
	}
	if t := g.currentTask(); t != nil {
		chrome.StatusItem(c, t.statusText(), chrome.StatusOptions{Busy: true, Tooltip: t.label(), ID: "git-sync"})
		if t.cancelable {
			chrome.StatusItem(c, "", chrome.StatusOptions{Icon: icons.Close, Tooltip: t.cancelLabel(), Command: cancelCommand, ID: "git-cancel"})
		}
		return
	}
	if !ready {
		return
	}
	st := g.snap.status
	switch {
	case st.Upstream != "":
		chrome.StatusItem(c, syncLabel(st.Ahead, st.Behind), chrome.StatusOptions{Icon: icons.Refresh, Label: syncName(st.Ahead, st.Behind), Tooltip: syncChangesLabel, OnClick: g.syncChanges, ID: "git-sync"})
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
