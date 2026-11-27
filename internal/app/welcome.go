package app

import (
	"path/filepath"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/ZerGo0/editor/resources"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	welcomeWidth  = 620
	welcomeHeight = 460
	welcomeTitle  = "Welcome"
	recentTitle   = "Recent Projects"
	removeRecent  = "Remove from Recent Projects"
)

func (a *App) showWelcome() {
	if a.welcome != nil {
		a.welcome.Restore()
		a.welcome.Focus()
		return
	}
	a.welcome = mygo.NewWindow(mygo.WindowOptions{
		Title:     welcomeTitle,
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
	p := workspace.ApplyTheme(c, a.settings)
	c.Root().Background(p.Layer.Editor)
	ui.Column(c).Fill().Padding(token.PadView.Px()).Gap(token.GapSection.Px()).Children(func() {
		design.Inline(c, token.GapGroup, func() {
			if logo := resources.Logo(); logo != nil {
				ui.Image(c, logo).Size(token.IconButtonSize.Px(), token.IconButtonSize.Px())
			}
			design.Text(c, "Editor", token.TextHeading)
		})
		open := action.Button(c, "Open Folder...", action.ButtonOptions{Variant: action.Primary, Icon: icons.FolderOpen, ID: "open-folder"}).Label("Open Folder")
		if open.Clicked() {
			a.openFolderDialog(a.welcome)
		}
		design.Text(c, recentTitle, token.TextTitle)
		recent := a.store.State.Recent
		list.View(c, &a.recentList, len(recent), list.ViewOptions{Label: recentTitle, Empty: func(c *ui.Context) {
			feedback.Empty(c, feedback.EmptyOptions{Icon: icons.FolderOpen, Title: "No recent projects", Message: "Open a folder to start."})
		}}, func(i int) list.Item {
			root := recent[i]
			name := filepath.Base(root)
			return list.Item{
				Label:       name,
				Description: fsutil.DisplayPath(root),
				Icon:        icons.Folder,
				Tooltip:     "Open " + name,
				OnClick:     func() { a.openPath(root) },
				Menu: func(m *ui.Menu) {
					if m.Item(removeRecent).Chosen() {
						a.removeRecent(root)
					}
				},
				Actions: func(c *ui.Context) {
					if action.IconButton(c, icons.Close, "Remove "+name+" from Recent Projects", action.IconOptions{Tooltip: removeRecent, ID: "remove"}).Clicked() {
						a.removeRecent(root)
					}
				},
			}
		}).Grow(1).FillWidth()
	})
}

func (a *App) removeRecent(root string) {
	a.store.RemoveRecent(root)
	a.saveSession()
	a.refreshMenu()
}
