package workspace

import (
	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/workspace/notices"
	"github.com/egoist/mygo/ui"
)

const (
	panicLoopBuilds  = 6
	openLogsFolderID = "workbench.action.openLogsFolder"
	panicTitle       = "Something went wrong"
	panicDetail      = "The editor recovered from an internal error. crash.log in the logs folder has the details."
	panicAction      = "Open Logs Folder"
	panicLoopTitle   = "This window keeps failing"
	panicLoopDetail  = "Save your changes, then close the window and open the folder again."
)

func (w *Workspace) View(c *ui.Context) {
	defer applog.Recover("window view", w.viewPanicked)
	w.view(c)
	w.failedBuilds = 0
}

func (w *Workspace) viewPanicked(*applog.Panic) {
	w.failedBuilds++
	switch w.failedBuilds {
	case 1:
		w.ShowPanic()
	case panicLoopBuilds:
		if w.OnPanicLoop != nil {
			w.OnPanicLoop()
			return
		}
		w.ShowPanicLoop()
	}
}

func (w *Workspace) ShowPanic() {
	w.notify(notices.Notice{Tone: token.ToneError, Title: panicTitle, Detail: panicDetail, Sticky: true,
		Actions: []notices.Action{{Label: panicAction, Run: func() { w.env.Commands.Run(openLogsFolderID, w.win) }}}})
}

func (w *Workspace) ShowPanicLoop() {
	w.notify(notices.Notice{Tone: token.ToneError, Title: panicLoopTitle, Detail: panicLoopDetail, Sticky: true})
}

func (w *Workspace) guarded(fn func()) func() {
	return func() {
		defer applog.Recover("window update", func(*applog.Panic) { w.ShowPanic() })
		fn()
	}
}
