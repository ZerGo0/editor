package workspace

import (
	"context"
	"log"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/egoist/mygo"
)

func (w *Workspace) visibleOutsidePaths() map[string]bool {
	out := map[string]bool{}
	for _, g := range w.layout.Groups() {
		if t := g.ActiveTab(); t != nil && onDisk(t.Path) && !fsutil.Within(w.Root, t.Path) {
			out[t.Path] = true
		}
	}
	return out
}

func (w *Workspace) syncOutsideWatches() {
	if w.win == nil {
		return
	}
	visible := w.visibleOutsidePaths()
	for p, stop := range w.outside {
		if !visible[p] {
			stop()
			delete(w.outside, p)
		}
	}
	for p := range visible {
		if _, ok := w.outside[p]; ok {
			continue
		}
		if w.outside == nil {
			w.outside = map[string]context.CancelFunc{}
		}
		ctx, stop := context.WithCancel(w.ctx)
		w.outside[p] = stop
		applog.Go(func() { w.watchOutside(ctx, p) })
	}
}

func (w *Workspace) watchOutside(ctx context.Context, p string) {
	watcher, err := mygo.WatchFiles(ctx, p, mygo.FileWatchOptions{HintsOnly: true})
	if err != nil {
		log.Printf("watch %s: %v", p, err)
		return
	}
	defer watcher.Close()
	w.update(func() { w.outsideChanged(p) })
	for {
		if _, err := watcher.Next(ctx); err != nil {
			return
		}
		w.update(func() { w.outsideChanged(p) })
	}
}

func (w *Workspace) outsideChanged(p string) {
	w.reloadChanged(p)
	w.reloadViewers(p)
}
