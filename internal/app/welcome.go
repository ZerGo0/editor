package app

import (
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/ZerGo0/editor/resources"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	welcomeWidth   = 620
	welcomeHeight  = 460
	welcomeTitle   = "Welcome"
	recentTitle    = "Recent Projects"
	removeRecent   = "Remove from Recent Projects"
	missingFolder  = "No longer exists"
	openFolderID   = "workbench.action.files.openFolder"
	openFileID     = "workbench.action.files.openFile"
	settingsID     = "workbench.action.openSettings"
	keybindingsID  = "workbench.action.openGlobalKeybindings"
	newFileTitle   = "New File"
	newFileButton  = "Create File"
	newFileTooltip = "Create a file and open its folder"
)

type welcomeState struct {
	list    ui.ListState
	row     int
	missing map[string]bool
	notice  *unopened
}

func (a *App) showWelcome() {
	a.checkRecent()
	if a.welcome != nil {
		a.raise(a.welcome)
		return
	}
	a.welcome = a.newWindow(mygo.WindowOptions{
		Title:     welcomeTitle,
		Width:     welcomeWidth,
		Height:    welcomeHeight,
		MinWidth:  welcomeWidth,
		MinHeight: welcomeHeight,
		Content:   ui.View(a.welcomeView),
	})
	win := a.welcome
	win.OnFocus(func() {
		a.checkRecent()
		a.syncViewMenu()
		win.Update(func() {})
	})
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

func (a *App) checkRecent() {
	missing := map[string]bool{}
	for _, root := range a.store.State.Recent {
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			missing[root] = true
		}
	}
	a.home.missing = missing
}

func (a *App) welcomeView(c *ui.Context) {
	c.SetZoom(workspace.Zoom(a.settings.Int(settings.ZoomLevel)))
	p := workspace.ApplyTheme(c, a.settings)
	root := c.Root().Background(p.Layer.Editor)
	for _, path := range root.DroppedFiles() {
		a.openPath(path)
	}
	ui.Column(c).Fill().Padding(token.PadView.Px()).Gap(token.GapSection.Px()).Children(func() {
		design.Inline(c, token.GapGroup, func() {
			if logo := resources.Logo(); logo != nil {
				ui.Image(c, logo).Size(token.IconButtonSize.Px(), token.IconButtonSize.Px())
			}
			design.Text(c, "Editor", token.TextHeading)
		})
		a.welcomeNotice(c)
		a.welcomeStart(c)
		design.Text(c, recentTitle, token.TextTitle)
		a.recentProjects(c)
		a.welcomeLinks(c)
	})
}

func (a *App) welcomeNotice(c *ui.Context) {
	n := a.home.notice
	if n == nil {
		return
	}
	o := feedback.NoticeOptions{Tone: token.ToneWarning, Text: n.message, OnDismiss: func() { a.home.notice = nil }}
	if n.recent != "" {
		o.Action, o.OnAction = removeRecent, func() { a.removeRecent(n.recent) }
	}
	feedback.Notice(c, o)
}

func (a *App) welcomeStart(c *ui.Context) {
	design.Inline(c, token.GapControl, func() {
		if a.commandButton(c, openFolderID, "Open Folder...", action.ButtonOptions{Variant: action.Primary, Icon: icons.FolderOpen, ID: "open-folder"}).Clicked() {
			a.openFolderDialog(a.welcome)
		}
		if a.commandButton(c, openFileID, "Open File...", action.ButtonOptions{Icon: icons.File, ID: "open-file"}).Clicked() {
			a.openFileDialog(a.welcome)
		}
		if action.Button(c, newFileTitle+"...", action.ButtonOptions{Icon: icons.FileAdd, ID: "new-file", Tooltip: newFileTooltip}).Clicked() {
			a.newFileDialog(a.welcome)
		}
	})
}

func (a *App) commandButton(c *ui.Context, id, label string, o action.ButtonOptions) ui.Element {
	o.Keys = a.keys.Current.Label(id)
	return action.Button(c, label, o)
}

func (a *App) recentProjects(c *ui.Context) {
	recent := a.store.State.Recent
	a.home.list.Selected = &a.home.row
	l := list.View(c, &a.home.list, len(recent), list.ViewOptions{Label: recentTitle, Empty: func(c *ui.Context) {
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.FolderOpen, Title: "No recent projects", Message: "Open a folder to start."})
	}}, func(i int) list.Item {
		root := recent[i]
		name := filepath.Base(root)
		it := list.Item{
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
		if a.home.missing[root] {
			gone := "The folder " + name + " no longer exists"
			it.Muted, it.Tooltip = true, gone
			it.Pills = []list.Pill{{Text: missingFolder, Icon: icons.Warning, Tone: token.ToneWarning, Tooltip: gone}}
		}
		return it
	}).Grow(1).FillWidth()
	row := a.home.row
	if row < 0 || row >= len(recent) || !l.FocusWithin() {
		return
	}
	switch {
	case l.Submitted():
		a.openPath(recent[row])
	case l.Shortcut(0, ui.KeyBackspace) || l.Shortcut(0, ui.KeyDelete) || l.Shortcut(ui.Cmd, ui.KeyBackspace):
		a.removeRecent(recent[row])
		a.home.row = min(row, len(recent)-2)
	}
}

func (a *App) welcomeLinks(c *ui.Context) {
	pw := a.lastActiveWindow()
	if pw == nil {
		return
	}
	design.Inline(c, token.GapControl, func() {
		for _, link := range []struct{ id, label string }{{settingsID, "Settings"}, {keybindingsID, "Keyboard Shortcuts"}} {
			if a.commandButton(c, link.id, link.label, action.ButtonOptions{Variant: action.Ghost, Icon: icons.Settings, ID: link.id}).Clicked() {
				a.runMenuCommand(link.id, pw.win)
			}
		}
	})
}

func (a *App) removeRecent(root string) {
	a.store.RemoveRecent(root)
	if n := a.home.notice; n != nil && n.recent == root {
		a.home.notice = nil
	}
	a.saveSession()
	a.refreshMenu()
}
