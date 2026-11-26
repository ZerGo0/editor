package workspace

import (
	"context"
	"log"
	"path"
	"path/filepath"

	"github.com/egoist/mygo"
)

func (w *Workspace) watch(ctx context.Context) {
	watcher, err := mygo.WatchFiles(ctx, w.Root, mygo.FileWatchOptions{
		Recursive: true,
		HintsOnly: true,
		Ignore: func(p string, isDir bool) bool {
			return isDir && path.Base(p) == ".git"
		},
	})
	if err != nil {
		log.Printf("watch %s: %v", w.Root, err)
		return
	}
	defer watcher.Close()
	for {
		events, err := watcher.Next(ctx)
		if err != nil {
			return
		}
		dirs := make([]string, 0, len(events))
		for _, e := range events {
			dirs = append(dirs, filepath.Join(w.Root, filepath.FromSlash(e.Path)))
		}
		w.update(func() { w.diskChanged(dirs) })
	}
}

func (w *Workspace) diskChanged(dirs []string) {
	for _, dir := range dirs {
		w.tree.Invalidate(dir)
		w.reloadChanged(dir)
		w.reloadViewers(dir)
	}
	if w.git != nil {
		w.git.DiskChanged()
	}
	w.search.view.Changed(dirs)
	w.scheduleReindex()
	w.scheduleIgnored()
}
