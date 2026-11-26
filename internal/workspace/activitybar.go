package workspace

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	activityBarWidth    = 44
	activityItemHeight  = 40
	activityIconSize    = 22
	activityIndicator   = 2
	activityIndicatorH  = 20
	activityBadgeSize   = 9
	activityBadgeMin    = 14
	activityBadgeOffset = 6
	activityPadding     = 4
	maxBadge            = 99
)

func (w *Workspace) activityBar(c *ui.Context, vibrant bool) {
	t := c.Theme()
	bar := ui.Column(c).Width(activityBarWidth).Shrink(0).FillHeight().AlignItems(ui.Center).Padding(activityPadding, 0).
		Role(ui.RoleToolbar).Label("Activity Bar")
	if !vibrant {
		bar.Background(t.Surface)
	}
	if w.sidebarHidden {
		bar.BorderWidth(0, 1, 0, 0).BorderColor(t.Border)
	}
	bar.Children(func() {
		for _, v := range w.tools.sideViews {
			if !w.tools.hiddenViews[v.id] {
				w.activityItem(c, v)
			}
		}
		ui.Box(c).Grow(1)
		w.settingsButton(c)
	})
	bar.ContextMenu(w.activityMenu)
}

func (w *Workspace) activityItem(c *ui.Context, v toolView) {
	t := c.Theme()
	active := v.id == w.tools.side && !w.sidebarHidden
	b := ui.ButtonBase(c.Key("activity-"+v.id)).Size(activityBarWidth, activityItemHeight).Center().
		Label(v.title).Tooltip(w.withKeys(v.title, v.command))
	color := t.TextMuted
	if active || b.Hovered() || b.Pressed() {
		color = t.Text
	}
	b.TextColor(color).Children(func() {
		if active {
			ui.Box(c).Absolute().Left(0).Top((activityItemHeight-activityIndicatorH)/2).Size(activityIndicator, activityIndicatorH).
				Radius(activityIndicator).Background(activeAccent(c))
		}
		ui.Icon(c, v.icon).Size(activityIconSize, activityIconSize)
		if v.badge != nil {
			activityBadge(c, v.badge())
		}
	})
	b.OnClick(func() {
		if active {
			w.ToggleSidebar()
			return
		}
		w.ShowSideView(v.id)
	})
}

func activityBadge(c *ui.Context, n int) {
	if n <= 0 {
		return
	}
	t := c.Theme()
	text := strconv.Itoa(n)
	if n > maxBadge {
		text = strconv.Itoa(maxBadge) + "+"
	}
	ui.Text(c, text).FontSize(activityBadgeSize).Bold().TextColor(t.AccentText).Background(activeAccent(c)).
		Radius(activityBadgeMin/2).MinWidth(activityBadgeMin).Height(activityBadgeMin).TextAlign(ui.Center).Padding(0, 3).
		Absolute().Right(activityBadgeOffset).Bottom(activityBadgeOffset)
}

func (w *Workspace) activityMenu(m *ui.Menu) {
	visible := 0
	for _, v := range w.tools.sideViews {
		if !w.tools.hiddenViews[v.id] {
			visible++
		}
	}
	for _, v := range w.tools.sideViews {
		shown := !w.tools.hiddenViews[v.id]
		if m.Item(v.title).Checked(shown).Disabled(shown && visible == 1).Chosen() {
			w.setViewHidden(v.id, shown)
		}
	}
}

func (w *Workspace) setViewHidden(id string, hidden bool) {
	if w.tools.hiddenViews == nil {
		w.tools.hiddenViews = map[string]bool{}
	}
	w.tools.hiddenViews[id] = hidden
	if !hidden || w.tools.side != id {
		return
	}
	for _, v := range w.tools.sideViews {
		if !w.tools.hiddenViews[v.id] {
			w.tools.side = v.id
			return
		}
	}
}

func (w *Workspace) settingsButton(c *ui.Context) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("manage")).Size(activityBarWidth, activityItemHeight).Center().
		Label("Manage").Tooltip("Manage")
	color := t.TextMuted
	if b.Hovered() || b.Pressed() {
		color = t.Text
	}
	b.TextColor(color).Children(func() { ui.Icon(c, icons.Settings).Size(activityIconSize, activityIconSize) })
	b.Menu(func(m *ui.Menu) {
		w.commandItem(m, "Command Palette...", "workbench.action.showCommands")
		m.Separator()
		w.commandItem(m, "Settings", "workbench.action.openSettings")
		w.commandItem(m, "Keyboard Shortcuts", "workbench.action.openGlobalKeybindings")
	})
}
