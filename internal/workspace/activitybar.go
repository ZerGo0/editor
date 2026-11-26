package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	activityLabel = "Activity Bar"
	manageLabel   = "Manage"
)

func (w *Workspace) visibleViews() []toolView {
	return slices.DeleteFunc(slices.Clone(w.tools.sideViews), func(v toolView) bool { return w.tools.hiddenViews[v.id] })
}

func (w *Workspace) activityBar(c *ui.Context) {
	views := w.visibleViews()
	items := make([]chrome.Activity, len(views))
	for i, v := range views {
		items[i] = chrome.Activity{Icon: v.icon, Label: v.title, Keys: w.keyLabel(v.command)}
		if v.badge != nil {
			items[i].Badge = v.badge()
		}
	}
	selected := slices.IndexFunc(views, func(v toolView) bool { return v.id == w.tools.side })
	row := ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Padding(0, token.PadRow.Px(), 0, 0).
		BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(token.Of(c).Border.Section)
	row.Children(func() {
		bar := chrome.ActivityBar(c, activityLabel, &selected, items).Grow(1).MinWidth(0)
		w.takeFocus(partSide, bar)
		if bar.Changed() && selected >= 0 && selected < len(views) {
			w.ShowSideView(views[selected].id)
		}
		action.MenuIconButton(c, icons.Settings, manageLabel, action.IconOptions{}, w.manageMenu)
	})
	row.ContextMenu(w.activityMenu)
}

func (w *Workspace) manageMenu(m *ui.Menu) {
	w.commandItem(m, "Command Palette...", "workbench.action.showCommands")
	m.Separator()
	w.commandItem(m, "Settings", "workbench.action.openSettings")
	w.commandItem(m, "Keyboard Shortcuts", "workbench.action.openGlobalKeybindings")
}

func (w *Workspace) activityMenu(m *ui.Menu) {
	visible := len(w.visibleViews())
	for _, v := range w.tools.sideViews {
		shown := !w.tools.hiddenViews[v.id]
		if m.Item(v.title).Checked(shown).Disabled(shown && visible == 1).Chosen() {
			w.setViewHidden(v.id, shown)
		}
	}
}

func (w *Workspace) setViewHidden(id string, hidden bool) {
	w.tools.hiddenViews[id] = hidden
	if !hidden || w.tools.side != id {
		return
	}
	if views := w.visibleViews(); len(views) > 0 {
		w.tools.side = views[0].id
	}
}
