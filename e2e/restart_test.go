//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ZerGo0/editor/internal/session"
)

func (q *QAApp) Restart() *QAApp {
	q.t.Helper()
	q.Quit()
	return launchQAIn(q.t, q.UserData, q.env, q.Root, q.Opts)
}

func (q *QAApp) CrashAndRelaunch() *QAApp {
	q.t.Helper()
	if q.conn != nil {
		_ = q.conn.Close()
	}
	_ = q.cmd.Process.Signal(syscall.SIGKILL)
	<-q.done
	return launchQAIn(q.t, q.UserData, q.env, q.Root, q.Opts)
}

func (q *QAApp) SessionFile() string {
	return filepath.Join(q.UserData, sessionFileName)
}

func (q *QAApp) SavedProject() session.Project {
	q.t.Helper()
	store, err := session.Load(q.SessionFile())
	if err != nil {
		q.t.Fatal(err)
	}
	return store.State.Projects[q.Root]
}

func (q *QAApp) WaitSaved(what string, cond func(session.Project) bool) session.Project {
	q.t.Helper()
	var last []byte
	var p session.Project
	q.Eventually(what+" in session.json", func() bool {
		data, err := os.ReadFile(q.SessionFile())
		if err != nil || bytes.Equal(data, last) {
			return false
		}
		last = data
		p = q.SavedProject()
		return cond(p)
	})
	return p
}
