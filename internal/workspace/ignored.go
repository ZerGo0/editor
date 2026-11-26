package workspace

import (
	"context"
	"log"
	"time"

	"github.com/ZerGo0/editor/internal/git"
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
	go func() {
		s, err := filetree.LoadIgnored(ctx, root)
		files := gitFiles(ctx, root)
		if ctx.Err() != nil {
			return
		}
		w.update(func() {
			if gen != w.ignored.gen {
				return
			}
			w.tree.SetGit(files)
			if err != nil {
				log.Printf("ignored files: %v", err)
				return
			}
			w.tree.SetIgnored(s)
		})
	}()
}

func (w *Workspace) scheduleIgnored() {
	if w.ignored.timer != nil {
		w.ignored.timer.Stop()
	}
	w.ignored.timer = time.AfterFunc(ignoredDelay, func() { w.update(w.refreshIgnored) })
}

func gitFiles(ctx context.Context, root string) []git.FileStatus {
	repo, err := git.Open(ctx, root)
	if err != nil {
		return nil
	}
	st, err := repo.Status(ctx)
	if err != nil {
		return nil
	}
	return st.Files
}
