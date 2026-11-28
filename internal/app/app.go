package app

import (
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/telemetry"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/ZerGo0/editor/internal/workspace/notices"
	"github.com/ZerGo0/editor/resources"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	sessionFile      = "session.json"
	projectWidth     = 1280
	projectHeight    = 820
	projectMinWidth  = 640
	projectMinHeight = 400

	transparentOpacity = 0

	damagedSuffix       = " (damaged)"
	revealInFinderLabel = "Reveal in Finder"
)

type Options struct {
	Hidden             bool
	ShowUnfocused      bool
	Transparent        bool
	DiscardOnClose     bool
	NoBackups          bool
	NoMenuAccelerators bool
	PickedPaths        []string
}

type projectWindow struct {
	win     *mygo.Window
	ws      *workspace.Workspace
	closing bool
	focused uint64
}

type App struct {
	options        Options
	store          *session.Store
	settings       *settings.Store
	keys           *keymap.Store
	commands       *command.Registry
	env            *workspace.Env
	projects       map[string]*projectWindow
	welcome        *mygo.Window
	home           welcomeState
	quitting       bool
	focusSeq       uint64
	ready          bool
	launchFiles    []string
	drawing        *workspace.Workspace
	panicReloads   map[string]int
	menu           menuState
	saveTimer      *time.Timer
	damagedSession string
}

func New(options Options) *App {
	return &App{options: options, projects: map[string]*projectWindow{}, panicReloads: map[string]int{}}
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
	mygo.App.OnOpenFile(a.openFile)
	applog.OnRecovered(a.showPanic)
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
		telemetry.Collect()
	})
	mygo.App.OnQuitCanceled(func() { a.quitting = false })
	mygo.App.WhenReady(func() {
		if err := resources.RegisterFonts(); err != nil {
			log.Printf("fonts: %v", err)
		}
		a.loadSession()
		a.loadConfig()
		action.SetCommands(commandSource{app: a})
		a.refreshMenu()
		a.ready = true
		if !a.openArgs(args, wd) && !a.openLaunchFiles() {
			a.restore()
		}
		a.reportDamagedSession()
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
		a.damagedSession = keepDamagedSession(path)
	}
	a.store = store
}

func keepDamagedSession(path string) string {
	dest := strings.TrimSuffix(path, filepath.Ext(path)) + damagedSuffix + filepath.Ext(path)
	if err := os.Rename(path, dest); err != nil {
		log.Printf("keep damaged session: %v", err)
		return ""
	}
	return dest
}

func (a *App) reportDamagedSession() {
	kept := a.damagedSession
	if kept == "" {
		return
	}
	a.damagedSession = ""
	const title = "Cannot read the list of recent projects"
	detail := "The file was damaged, so the list starts empty. The damaged file is kept as " + fsutil.DisplayPath(kept) + "."
	if pw := a.lastActiveWindow(); pw != nil {
		pw.ws.Warn(title, detail, notices.Action{Label: revealInFinderLabel, Run: func() { mygo.Shell.ShowItemInFolder(kept) }})
		return
	}
	parent := a.welcome
	applog.Go(func() {
		if _, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: parent, Type: mygo.MessageWarning, Message: title, Detail: detail}); err != nil {
			log.Printf("damaged session notice: %v", err)
		}
	})
}

func (a *App) saveSession() {
	a.store.State.Commands = a.commands.Recent()
	if a.env != nil {
		a.store.State.Views = a.env.Global.Snapshot()
	}
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

func (a *App) raise(win *mygo.Window) {
	win.Restore()
	switch {
	case !a.options.Hidden:
		win.Focus()
	case a.options.ShowUnfocused:
		win.ShowInactive()
	}
}

func (a *App) newWindow(o mygo.WindowOptions) *mygo.Window {
	o.Hidden = a.options.Hidden
	win := mygo.NewWindow(o)
	if a.options.Transparent {
		win.SetOpacity(transparentOpacity)
		win.SetIgnoreMouseEvents(true)
		win.SetHasShadow(false)
	}
	if a.options.ShowUnfocused {
		win.SetActiveAppearance(true)
		win.ShowInactive()
	}
	return win
}

func (a *App) raiseProject(pw *projectWindow) {
	a.raise(pw.win)
	a.touchWindow(pw)
}

func (a *App) focusAny() {
	if pw := a.lastActiveWindow(); pw != nil {
		a.raiseProject(pw)
		return
	}
	a.showWelcome()
}

func (a *App) openProject(root string) *projectWindow {
	if pw, ok := a.projects[root]; ok {
		a.raiseProject(pw)
		return pw
	}
	ws := workspace.New(root, a.store.State.Projects[root], a.env)
	win := a.newWindow(mygo.WindowOptions{
		Title:                ws.Name,
		Width:                projectWidth,
		Height:               projectHeight,
		MinWidth:             projectMinWidth,
		MinHeight:            projectMinHeight,
		StateKey:             "project:" + root,
		TitleBarStyle:        mygo.TitleBarHiddenInset,
		TitleBarHeight:       workspace.TitleBarHeight,
		TrafficLightPosition: workspace.TrafficLightPosition(),
		Content:              ui.View(a.projectView(ws)),
	})
	ws.Attach(win)
	a.watchFullScreen(win)
	pw := &projectWindow{win: win, ws: ws}
	ws.OnViewState = func(s workspace.ViewState) {
		if a.workspaceFor(nil) == pw {
			a.applyViewState(s)
		}
	}
	ws.OnPanicLoop = func() { win.Update(func() { a.reloadAfterPanic(pw) }) }
	ws.OnStateChange = a.sessionChanged
	a.touchWindow(pw)
	win.OnFocus(func() {
		a.touchWindow(pw)
		a.syncViewMenu()
	})
	a.projects[root] = pw
	win.OnBlur(telemetry.CollectIfDue)
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

func (a *App) closeReason() workspace.CloseReason {
	if a.quitting {
		return workspace.AppQuit
	}
	return workspace.WindowClose
}

func (a *App) confirmClose(pw *projectWindow, e *mygo.CloseEvent) {
	reason := a.closeReason()
	if pw.closing || !a.needsSavePrompt(pw, reason) {
		return
	}
	e.PreventDefault()
	pw.ws.ConfirmClose(reason, func(ok bool) {
		if !ok {
			return
		}
		pw.closing = true
		if reason == workspace.AppQuit {
			mygo.App.Quit()
			return
		}
		pw.win.Close()
	})
}

func (a *App) needsSavePrompt(pw *projectWindow, reason workspace.CloseReason) bool {
	prompt := pw.ws.PrepareClose(reason)
	return prompt && !a.options.DiscardOnClose
}

func (a *App) confirmWindowClose(pw *projectWindow, done func(ok bool)) {
	if !a.needsSavePrompt(pw, workspace.WindowClose) {
		done(true)
		return
	}
	pw.ws.ConfirmClose(workspace.WindowClose, done)
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

func (a *App) inWorkspace(fn func(*workspace.Workspace)) func(*mygo.Window) {
	return func(win *mygo.Window) {
		if pw := a.workspaceFor(win); pw != nil {
			pw.ws.Update(func() { fn(pw.ws) })
			a.sessionChanged()
		}
	}
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
