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
	return a.openPathIn(path, nil)
}

func (a *App) openPathIn(path string, window *projectWindow) bool {
	path, info, err := resolvePath(path)
	if err != nil {
		log.Printf("open %s: %v", path, err)
		return false
	}
	if info.IsDir() {
		a.openProject(path)
		return true
	}
	pw := window
	if pw == nil {
		if pw = a.projectContaining(path); pw != nil {
			pw.win.Restore()
			pw.win.Focus()
		} else {
			pw = a.openProject(filepath.Dir(path))
		}
	}
	pw.win.Update(func() { pw.ws.OpenFile(path, false, true) })
	return true
}

func resolvePath(path string) (string, os.FileInfo, error) {
	path, err := canonical(path)
	if err != nil {
		return path, nil, err
	}
	info, err := os.Stat(path)
	return path, info, err
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
		paths, err := a.pickPaths(parent, title, directory)
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
		mygo.RunOnMain(func() { a.openPathIn(paths[0], a.workspaceFor(parent)) })
	}()
}

func (a *App) pickPaths(parent *mygo.Window, title string, directory bool) ([]string, error) {
	if len(a.options.PickedPaths) > 0 {
		return a.options.PickedPaths, nil
	}
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:            parent,
		Title:             title,
		Directory:         directory,
		CreateDirectories: directory,
	})
}

func (a *App) openFolder(from *mygo.Window, root string, replace bool) {
	source := a.workspaceFor(from)
	if c, err := canonical(root); err == nil {
		root = c
	}
	_, openElsewhere := a.projects[root]
	if info, err := os.Stat(root); err != nil || !info.IsDir() || !replace || source == nil || openElsewhere {
		a.openPath(root)
		return
	}
	replaceWindow(
		func(done func(ok bool)) { a.confirmWindowClose(source, done) },
		func() bool { return a.openPath(root) },
		func() {
			source.closing = true
			mygo.RunOnMain(source.win.Close)
		},
	)
}

func replaceWindow(confirm func(done func(ok bool)), open func() bool, closeOld func()) {
	confirm(func(ok bool) {
		if ok && open() {
			closeOld()
		}
	})
}
