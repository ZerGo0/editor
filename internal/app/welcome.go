package app

import (
	"path/filepath"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	welcomeWidth     = 620
	welcomeHeight    = 460
	welcomeTitleSize = 26
	recentIconSize   = 12
)

func (a *App) showWelcome() {
	if a.welcome != nil {
		a.welcome.Restore()
		a.welcome.Focus()
		return
	}
	a.welcome = mygo.NewWindow(mygo.WindowOptions{
		Title:     "Welcome",
		Width:     welcomeWidth,
		Height:    welcomeHeight,
		MinWidth:  welcomeWidth,
		MinHeight: welcomeHeight,
		Hidden:    a.options.Hidden,
		Content:   ui.View(a.welcomeView),
	})
	win := a.welcome
	win.OnClosed(func() {
		if a.welcome == win {
			a.welcome = nil
		}
	})
}

func (a *App) closeWelcome() {
	if a.welcome != nil {
		a.welcome.Close()
	}
}

func (a *App) welcomeView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(32).Gap(16).Children(func() {
		ui.Text(c, "Editor").FontSize(welcomeTitleSize).Bold()
		ui.PrimaryButton(c.Key("open-folder"), "").Children(func() {
			ui.Icon(c, icons.FolderOpen)
			ui.Text(c, "Open Folder...").SingleLine()
		}).Label("Open Folder").OnClick(func() { a.openFolderDialog(a.welcome) })
		ui.Text(c, "Recent Projects").Bold().Margin(8, 0, 0, 0)
		if len(a.store.State.Recent) == 0 {
			ui.Text(c, "No recent projects.").TextColor(t.TextMuted)
			return
		}
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).Gap(2).FillWidth().Children(func() {
				for _, root := range a.store.State.Recent {
					a.recentRow(c, root)
				}
			})
		})
	})
}

func (a *App) recentRow(c *ui.Context, root string) {
	t := c.Theme()
	name := filepath.Base(root)
	ui.Row(c.Key(root)).Gap(4).AlignItems(ui.Center).FillWidth().Children(func() {
		open := ui.ButtonBase(c.Key("open")).Grow(1).MinWidth(0).Padding(8, 10).Radius(6).Label("Open " + name)
		if open.Hovered() {
			open.Background(t.SurfaceHover)
		}
		open.Children(func() {
			ui.Column(c).Gap(2).MinWidth(0).Children(func() {
				ui.Text(c, name).Bold().SingleLine()
				ui.Text(c, displayPath(root)).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
		})
		open.OnClick(func() { a.openPath(root) })
		remove := ui.ButtonBase(c.Key("remove")).Size(28, 28).Radius(6).Center().TextColor(t.TextMuted).
			Label("Remove " + name + " from Recent Projects").Tooltip("Remove from Recent Projects")
		if remove.Hovered() {
			remove.Background(t.SurfaceHover)
		}
		remove.Children(func() { ui.Icon(c, icons.Close).Size(recentIconSize, recentIconSize) })
		remove.OnClick(func() {
			a.store.RemoveRecent(root)
			a.saveSession()
			a.refreshMenu()
		})
	})
}
