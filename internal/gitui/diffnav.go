package gitui

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/egoist/mygo/ui"
)

const (
	diffCategory              = "Diff"
	nextDifferenceCommand     = "editor.action.accessibleDiffViewer.next"
	previousDifferenceCommand = "editor.action.accessibleDiffViewer.prev"
	nextChangeCommand         = "workbench.action.compareEditor.nextChange"
	previousChangeCommand     = "workbench.action.compareEditor.previousChange"
	openFileCommand           = "git.openFile2"
	noDiffToStep              = "Open a diff first."
)

func (g *Git) activeView() string {
	if v, ok := g.h.(activeViewer); ok {
		return v.ActiveViewID()
	}
	return ""
}

func (g *Git) DiffContext() (active, focused bool) {
	id := g.activeView()
	if t, ok := g.tabs[id]; ok {
		return true, t.focused
	}
	if t, ok := g.patchTabs[id]; ok {
		return true, t.view.Focused()
	}
	return false, false
}

func (g *Git) stepActiveDiff(delta int) {
	id := g.activeView()
	if t, ok := g.tabs[id]; ok && t.state != nil {
		t.goToChange(delta)
		return
	}
	if t, ok := g.patchTabs[id]; ok {
		t.view.Step(delta)
		return
	}
	g.h.Notify(noDiffToStep)
}

func (t *diffTab) changeTarget(delta int) int {
	i, inside := t.state.CurrentHunk()
	if delta < 0 && !inside {
		return i
	}
	return i + delta
}

func (t *diffTab) goToChange(delta int) {
	target := t.changeTarget(delta)
	if target < 0 || target >= len(t.state.Hunks()) {
		return
	}
	t.state.GoToHunk(target)
	t.state.Focus()
}

func (t *diffTab) changeButton(c *ui.Context, id, label string, icon *ui.SVG, delta int) {
	info, _ := action.Lookup(id)
	target := t.changeTarget(delta)
	disabled := target < 0 || target >= len(t.state.Hunks())
	if action.IconButton(c, icon, label, action.IconOptions{Keys: info.Keys, Disabled: disabled, ID: id}).Clicked() && !disabled {
		t.goToChange(delta)
	}
}
