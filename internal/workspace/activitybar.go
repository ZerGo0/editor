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
	searchActivityLabel   = "Search"
	searchActivityCommand = "workbench.action.findInFiles"
	activityLabel         = "Activity Bar"
	manageLabel           = "Manage"
)

func (w *Workspace) visibleViews() []toolView {
	return slices.DeleteFunc(slices.Clone(w.tools.sideViews), func(v toolView) bool { return w.tools.hiddenViews[v.id] })
}

type activityEntry struct {
	item chrome.Activity
	view string
}

func (w *Workspace) activityEntries() (entries []activityEntry, selected int) {
	views := w.visibleViews()
	searching := w.searchTextShown()
	for _, v := range views {
		item := chrome.Activity{Icon: v.icon, Label: v.title, Keys: w.keyLabel(v.command)}
		if v.badge != nil {
			item.Badge = v.badge()
			if v.badgeTip != nil && item.Badge > 0 {
				item.BadgeTip = v.badgeTip(item.Badge)
			}
		}
		if v.id == w.tools.side && !searching {
			selected = len(entries)
		}
		entries = append(entries, activityEntry{item: item, view: v.id})
		if v.id == ExplorerView {
			entries = append(entries, w.searchEntry())
		}
	}
	if !slices.ContainsFunc(entries, func(e activityEntry) bool { return e.view == "" }) {
		entries = slices.Insert(entries, 0, w.searchEntry())
		selected++
	}
	if searching {
		selected = slices.IndexFunc(entries, func(e activityEntry) bool { return e.view == "" })
	}
	return entries, selected
}

func (w *Workspace) searchEntry() activityEntry {
	return activityEntry{item: chrome.Activity{Icon: icons.Search, Label: searchActivityLabel, Keys: w.keyLabel(searchActivityCommand)}}
}

func (w *Workspace) activityBar(c *ui.Context, v *toolView, showActions bool) {
	entries, selected := w.activityEntries()
	items := make([]chrome.Activity, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	row := ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Padding(0, token.PadRow.Px(), 0, 0).
		BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(token.Of(c).Border.Section)
	row.Children(func() {
		bar := chrome.ActivityBar(c, activityLabel, &selected, items).Shrink(1).MinWidth(0)
		w.takeFocus(partSide, bar)
		if bar.Changed() && selected >= 0 && selected < len(entries) {
			if entries[selected].view == "" {
				w.ShowSearch()
			} else {
				w.ShowSideView(entries[selected].view)
			}
		}
		ui.Box(c).Grow(1)
		if v != nil && v.actions != nil {
			actions := action.Toolbar(c, v.title)
			if !showActions {
				actions.Opacity(token.OpacityHidden.Value())
			}
			actions.Children(func() { v.actions(c) })
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
