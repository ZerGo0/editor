package workspace

import (
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
)

const (
	closeSidebarID  = "workbench.action.closeSidebar"
	unknownPanelID  = "workbench.action.terminal.toggleTerminal"
	sideBarVisibleK = "sideBarVisible"
)

func TestCloseSidebarAndRunCommandsFollowTheContext(t *testing.T) {
	env := newTestEnv(t)
	var w *Workspace
	env.Commands.Add(
		&command.Command{ID: closeSidebarID, Run: func(*mygo.Window) { w.CloseSidebar() }},
		&command.Command{ID: toggleSidebarID, Run: func(*mygo.Window) { w.ToggleSidebar() }},
	)
	w = newTestWorkspace(t, t.TempDir(), env, session.Project{})
	if !w.whenContext(nil)[sideBarVisibleK].(bool) {
		t.Fatal("the side bar starts hidden")
	}
	w.run(command.SequenceID([]string{closeSidebarID, unknownPanelID}))
	w.runPendingUpdates()
	if !w.sidebarHidden || w.whenContext(nil)[sideBarVisibleK].(bool) {
		t.Fatal("closing the side bar left it visible")
	}
	w.run(closeSidebarID)
	w.runPendingUpdates()
	if !w.sidebarHidden {
		t.Fatal("closing a hidden side bar must keep it hidden")
	}
	w.run(command.SequenceID([]string{unknownPanelID, toggleSidebarID}))
	w.runPendingUpdates()
	if w.sidebarHidden {
		t.Fatal("the toggle after an unknown command did not run")
	}
}

func TestActiveViewIDAndSourceControlFocus(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	if got := w.ActiveViewID(); got != "" {
		t.Errorf("ActiveViewID with no view = %q", got)
	}
	w.OpenView("git-diff:1", "Diff", nil)
	if got := w.ActiveViewID(); got != "git-diff:1" {
		t.Errorf("ActiveViewID = %q, want git-diff:1", got)
	}
	w.tools.side, w.focus.side = SourceControlView, true
	if !w.SourceControlFocused() {
		t.Error("SourceControlFocused is false with the changes list focused")
	}
	w.CloseSidebar()
	if w.SourceControlFocused() {
		t.Error("SourceControlFocused is true with the side bar hidden")
	}
}
