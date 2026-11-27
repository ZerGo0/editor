package app

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

func (a *App) openArgs(args []string, dir string) bool {
	opened := false
	for _, arg := range args {
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		if !filepath.IsAbs(arg) {
			arg = filepath.Join(dir, arg)
		}
		if a.openPath(arg) {
			opened = true
		}
	}
	return opened
}

func (a *App) openPath(path string) bool {
	path, err := canonical(path)
	if err != nil {
		log.Printf("open %s: %v", path, err)
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		log.Printf("open %s: %v", path, err)
		return false
	}
	if info.IsDir() {
		a.openProject(path)
		return true
	}
	pw := a.projectContaining(path)
	if pw == nil {
		pw = a.openProject(filepath.Dir(path))
	} else {
		pw.win.Restore()
		pw.win.Focus()
	}
	pw.win.Update(func() { pw.ws.OpenFile(path, false, true) })
	return true
}

func (a *App) projectContaining(path string) *projectWindow {
	var best *projectWindow
	for root, pw := range a.projects {
		r, err := filepath.Rel(root, path)
		if err == nil && filepath.IsLocal(r) && (best == nil || len(root) > len(best.ws.Root)) {
			best = pw
		}
	}
	return best
}

func canonical(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return path, err
	}
	return filepath.EvalSymlinks(path)
}

func (a *App) openFolderDialog(parent *mygo.Window) {
	a.openDialog(parent, "Open Folder", true)
}

func (a *App) openFileDialog(parent *mygo.Window) {
	a.openDialog(parent, "Open File", false)
}

func (a *App) openDialog(parent *mygo.Window, title string, directory bool) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            parent,
			Title:             title,
			Directory:         directory,
			CreateDirectories: directory,
		})
		if err != nil {
			log.Printf("%s: %v", strings.ToLower(title), err)
			return
		}
		if len(paths) == 0 {
			return
		}
		if directory {
			mygo.RunOnMain(func() { a.openFolder(parent, paths[0], true) })
			return
		}
		mygo.RunOnMain(func() { a.openPath(paths[0]) })
	}()
}

func (a *App) openFolder(from *mygo.Window, root string, replace bool) {
	source := a.workspaceFor(from)
	if c, err := canonical(root); err == nil {
		root = c
	}
	_, openElsewhere := a.projects[root]
	if !a.openPath(root) || !replace || source == nil || openElsewhere {
		return
	}
	if target := a.projectContaining(root); target != nil && target != source {
		mygo.RunOnMain(source.win.Close)
	}
}
