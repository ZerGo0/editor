package gitui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/git"
)

const (
	cloneCommand    = "git.clone"
	cloneTitle      = "Clone"
	cloneURLTitle   = "Repository URL"
	clonePickTitle  = "Choose a Folder to Clone Into"
	cloneOpenAction = "Open in New Window"
)

type folderPicker interface {
	PickFolder(title string) (string, error)
}

type folderOpener interface {
	OpenFolder(root string)
}

type actionNotifier interface {
	NotifyAction(title, detail, action string, onAction func())
}

func (g *Git) cloneRepository() {
	picker, ok := g.h.(folderPicker)
	if !ok {
		g.h.Notify("Clone is not available in this window.")
		return
	}
	g.h.Ask(cloneURLTitle, cloneTitle, "", func(u string) error {
		if err := git.CheckCloneURL(u); err != nil {
			return err
		}
		applog.Go(func() {
			parent, err := picker.PickFolder(clonePickTitle)
			if err != nil {
				log.Printf("clone: pick folder: %v", err)
				return
			}
			if parent != "" {
				g.post(func() { g.startClone(u, parent) })
			}
		})
		return nil
	})
}

func (g *Git) startClone(u, parent string) {
	if t := g.taskNamed(cloneTitle); t != nil {
		g.h.Notify(cloneTitle + " is still running.")
		return
	}
	name := git.CloneDirName(u)
	dest := filepath.Join(parent, name)
	if _, err := os.Lstat(dest); err == nil {
		g.h.ShowError("Cannot clone "+name, fmt.Errorf("%s already has a file or folder named %s. Choose another folder.", filepath.Base(parent), name))
		return
	}
	ctx, cancel := context.WithCancel(g.ctx)
	t := g.addTask(cloneTitle, cancel, true, false)
	t.detached = true
	sink := &progressSink{post: g.post, set: t.report}
	applog.Go(func() {
		err := git.Clone(ctx, u, dest, sink.line)
		cancel()
		g.post(func() {
			g.removeTask(t)
			switch {
			case errors.Is(err, context.Canceled):
			case err != nil:
				g.showError("Clone failed", err)
			default:
				g.cloned(dest)
			}
		})
	})
}

func (g *Git) cloned(dest string) {
	title := "Cloned " + filepath.Base(dest)
	opener, canOpen := g.h.(folderOpener)
	notifier, canAct := g.h.(actionNotifier)
	if !canOpen || !canAct {
		g.h.Notify(title)
		return
	}
	notifier.NotifyAction(title, dest, cloneOpenAction, func() { opener.OpenFolder(dest) })
}
