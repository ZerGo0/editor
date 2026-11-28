package app

import (
	"time"

	"github.com/egoist/mygo"
)

const sessionSaveDelay = time.Second

func (a *App) sessionChanged() {
	if a.saveTimer != nil {
		a.saveTimer.Reset(sessionSaveDelay)
		return
	}
	a.saveTimer = time.AfterFunc(sessionSaveDelay, func() { mygo.RunOnMain(a.flushSession) })
}

func (a *App) flushSession() {
	if a.quitting {
		return
	}
	for root, pw := range a.projects {
		if !pw.closing {
			a.store.State.Projects[root] = pw.ws.Snapshot()
		}
	}
	a.saveSession()
}
