package app

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/egoist/mygo"
)

type unopened struct {
	message string
	detail  string
	recent  string
}

func (a *App) reportUnopened(path string, err error, window *projectWindow) {
	applog.Go(func() { mygo.RunOnMain(func() { a.showUnopened(path, err, window) }) })
}

func (a *App) showUnopened(path string, err error, window *projectWindow) {
	n := unopened{message: "Cannot open " + filepath.Base(path), detail: err.Error()}
	if errors.Is(err, fs.ErrNotExist) {
		n.message, n.detail = filepath.Base(path)+" does not exist", fsutil.DisplayPath(path)
		if slices.Contains(a.store.State.Recent, path) {
			n.message, n.recent = "The folder "+filepath.Base(path)+" no longer exists", path
		}
	}
	pw := window
	if pw == nil {
		pw = a.lastActiveWindow()
	}
	if pw == nil {
		a.home.notice = &n
		a.showWelcome()
		a.welcome.Update(func() {})
		return
	}
	action, remove := "", func() {}
	if n.recent != "" {
		action, remove = removeRecent, func() { a.removeRecent(n.recent) }
	}
	pw.ws.Update(func() { pw.ws.NotifyAction(n.message, n.detail, action, remove) })
}
