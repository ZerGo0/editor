package app

import (
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	sessionFile      = "session.json"
	projectWidth     = 1280
	projectHeight    = 820
	projectMinWidth  = 640
	projectMinHeight = 400
)

type Options struct {
	Hidden         bool
	DiscardOnClose bool
}

type projectWindow struct {
	win     *mygo.Window
	ws      *workspace.Workspace
	closing bool
}

type App struct {
	options  Options
	store    *session.Store
	settings *settings.Store
	keys     *keymap.Store
	commands *command.Registry
	env      *workspace.Env
	projects map[string]*projectWindow
	welcome  *mygo.Window
	quitting bool
}

func New(options Options) *App {
	return &App{options: options, projects: map[string]*projectWindow{}}
}

func (a *App) Run(args []string) error {
	extendPath()
	wd, err := os.Getwd()
	if err != nil {
		wd = "/"
	}
	mygo.App.OnSecondInstance(func(args []string, dir string) {
		if !a.openArgs(args, dir) {
			a.focusAny()
		}
	})
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnActivate(func(visible bool) {
		if !visible {
			a.showWelcome()
		}
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.quitting = true
		a.store.State.Open = a.openRoots()
		a.saveSession()
	})
	mygo.App.OnQuitCanceled(func() { a.quitting = false })
	mygo.App.WhenReady(func() {
		a.loadSession()
		a.loadConfig()
		a.refreshMenu()
		if !a.openArgs(args, wd) {
			a.restore()
		}
	})
	return mygo.App.Run()
}

func (a *App) loadSession() {
	path := sessionFile
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
		path = filepath.Join(dir, sessionFile)
	}
	store, err := session.Load(path)
	if err != nil {
		log.Printf("load session: %v", err)
	}
	a.store = store
}

func (a *App) saveSession() {
	if err := a.store.Save(); err != nil {
		log.Printf("save session: %v", err)
	}
}

func (a *App) openRoots() []string {
	roots := make([]string, 0, len(a.projects))
	for root := range a.projects {
		roots = append(roots, root)
	}
	slices.Sort(roots)
	return roots
}

func (a *App) restore() {
	opened := false
	if !a.settings.Bool(settings.RestoreWindows) {
		a.showWelcome()
		return
	}
	for _, root := range a.store.State.Open {
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			a.openProject(root)
			opened = true
		}
	}
	if !opened {
		a.showWelcome()
	}
}

func (a *App) focusAny() {
	for _, pw := range a.projects {
		pw.win.Restore()
		pw.win.Focus()
		return
	}
	a.showWelcome()
}

func (a *App) openProject(root string) *projectWindow {
	if pw, ok := a.projects[root]; ok {
		pw.win.Restore()
		pw.win.Focus()
		return pw
	}
	ws := workspace.New(root, a.store.State.Projects[root], a.env)
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:          ws.Name,
		Width:          projectWidth,
		Height:         projectHeight,
		MinWidth:       projectMinWidth,
		MinHeight:      projectMinHeight,
		StateKey:       "project:" + root,
		TitleBarStyle:  mygo.TitleBarHiddenInset,
		TitleBarHeight: workspace.TitleBarHeight,
		Vibrancy:       mygo.VibrancySidebar,
		Hidden:         a.options.Hidden,
		Content:        ui.View(ws.View),
	})
	ws.Attach(win)
	pw := &projectWindow{win: win, ws: ws}
	a.projects[root] = pw
	win.OnClose(func(e *mygo.CloseEvent) { a.confirmClose(pw, e) })
	win.OnClosed(func() { a.projectClosed(root, pw) })
	a.store.AddRecent(root)
	a.store.State.Open = a.openRoots()
	if err := mygo.App.AddRecentDocument(root); err != nil {
		log.Printf("add recent document: %v", err)
	}
	a.saveSession()
	a.refreshMenu()
	a.closeWelcome()
	return pw
}

func (a *App) confirmClose(pw *projectWindow, e *mygo.CloseEvent) {
	if pw.closing || a.options.DiscardOnClose || !pw.ws.HasUnsaved() {
		return
	}
	e.PreventDefault()
	quitting := a.quitting
	pw.ws.ConfirmClose(func(ok bool) {
		if !ok {
			return
		}
		pw.closing = true
		if quitting {
			mygo.App.Quit()
			return
		}
		pw.win.Close()
	})
}

func (a *App) projectClosed(root string, pw *projectWindow) {
	a.store.State.Projects[root] = pw.ws.Snapshot()
	pw.ws.Close()
	delete(a.projects, root)
	if !a.quitting {
		a.store.State.Open = a.openRoots()
		if len(a.projects) == 0 {
			a.showWelcome()
		}
	}
	a.saveSession()
}

func (a *App) workspaceFor(win *mygo.Window) *projectWindow {
	if win == nil {
		win = mygo.FocusedWindow()
	}
	for _, pw := range a.projects {
		if pw.win == win {
			return pw
		}
	}
	if len(a.projects) == 1 {
		for _, pw := range a.projects {
			return pw
		}
	}
	return nil
}
