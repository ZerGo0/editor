package app

import (
	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
)

const (
	maxPanicReloads    = 1
	windowFailedTitle  = "A window kept failing and closed"
	windowFailedDetail = "It failed again after a reload. Unsaved changes are kept and come back when you open the folder again. crash.log in the logs folder has the details."
)

func (a *App) runMenuCommand(id string, win *mygo.Window) {
	defer applog.Recover("menu command "+id, nil)
	a.commands.Run(id, win)
}

func (a *App) showPanic(*applog.Panic) {
	mygo.RunOnMain(func() {
		if pw := a.lastActiveWindow(); pw != nil {
			pw.ws.ShowPanic()
		}
	})
}

func (a *App) reloadAfterPanic(pw *projectWindow) {
	defer applog.Recover("window reload", nil)
	root := pw.ws.Root
	if a.projects[root] != pw || pw.closing {
		return
	}
	if pw.ws.PrepareClose(workspace.AppQuit) {
		pw.ws.ShowPanicLoop()
		return
	}
	pw.closing = true
	pw.win.Close()
	if a.panicReloads[root] >= maxPanicReloads {
		go mygo.Dialog.Error(windowFailedTitle, windowFailedDetail)
		return
	}
	a.panicReloads[root]++
	a.openProject(root)
}
