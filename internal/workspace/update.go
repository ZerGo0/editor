package workspace

import (
	"sync"

	"github.com/egoist/mygo"
)

type updateQueue struct {
	mu      sync.Mutex
	win     *mygo.Window
	pending []func()
}

func (w *Workspace) update(fn func()) {
	fn = w.guarded(fn)
	q := &w.updates
	q.mu.Lock()
	win := q.win
	if win == nil {
		q.pending = append(q.pending, fn)
	}
	q.mu.Unlock()
	if win != nil {
		win.Update(fn)
	}
}

func (w *Workspace) attachUpdates(win *mygo.Window) {
	q := &w.updates
	q.mu.Lock()
	q.win = win
	pending := q.pending
	q.pending = nil
	q.mu.Unlock()
	for _, fn := range pending {
		win.Update(fn)
	}
}

func (w *Workspace) runPendingUpdates() {
	q := &w.updates
	q.mu.Lock()
	pending := q.pending
	q.pending = nil
	q.mu.Unlock()
	for _, fn := range pending {
		fn()
	}
}
