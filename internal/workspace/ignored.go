package workspace

import (
	"log"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
)

const ignoredDelay = 500 * time.Millisecond

type ignoredState struct {
	gen   int
	timer *time.Timer
}

func (w *Workspace) refreshIgnored() {
	w.ignored.gen++
	gen, ctx, root := w.ignored.gen, w.ctx, w.Root
	applog.Go(func() {
		s, err := filetree.LoadIgnored(ctx, root)
		if ctx.Err() != nil {
			return
		}
		w.update(func() {
			if gen != w.ignored.gen {
				return
			}
			if err != nil {
				log.Printf("ignored files: %v", err)
				return
			}
			w.tree.SetIgnored(s)
		})
	})
}

func (w *Workspace) scheduleIgnored() {
	if w.ignored.timer != nil {
		w.ignored.timer.Stop()
	}
	w.ignored.timer = time.AfterFunc(ignoredDelay, func() { w.update(w.refreshIgnored) })
}
