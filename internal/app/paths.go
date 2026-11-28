package app

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/review"
	"github.com/egoist/mygo"
)

const newFileMode = 0o644

func (a *App) openArgs(args []string, dir string) bool {
	opened := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == review.RequestFlag && i+1 < len(args) {
			i++
			opened = a.openReview(args[i]) || opened
			continue
		}
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
	resolved, info, err := resolvePath(path)
	if err != nil {
		log.Printf("open %s: %v", resolved, err)
		a.reportUnopened(path, err, window)
		return false
	}
	path = resolved
	if info.IsDir() {
		a.openProject(path)
		return true
	}
	pw := window
	if pw == nil {
		if pw = a.windowForFile(path); pw != nil {
			a.raiseProject(pw)
		} else {
			pw = a.openProject(filepath.Dir(path))
		}
	}
	pw.ws.Update(func() { pw.ws.OpenFile(path, false, true) })
	return true
}

func (a *App) windowForFile(path string) *projectWindow {
	if pw := a.projectContaining(path); pw != nil {
		return pw
	}
	return a.lastActiveWindow()
}

func (a *App) touchWindow(pw *projectWindow) {
	a.focusSeq++
	pw.focused = a.focusSeq
}

func (a *App) lastActiveWindow() *projectWindow {
	var last *projectWindow
	for _, pw := range a.projects {
		if last == nil || pw.focused > last.focused {
			last = pw
		}
	}
	return last
}

func (a *App) openFile(path string) {
	if !a.ready {
		a.launchFiles = append(a.launchFiles, path)
		return
	}
	a.openPath(path)
}

func (a *App) openLaunchFiles() bool {
	files := a.launchFiles
	a.launchFiles = nil
	opened := false
	for _, path := range files {
		if a.openPath(path) {
			opened = true
		}
	}
	return opened
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
	applog.Go(func() {
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
	})
}

func (a *App) newFileDialog(parent *mygo.Window) {
	applog.Go(func() {
		path, err := a.pickSavePath(parent)
		if err == nil && path != "" {
			err = createFile(path)
		}
		if err != nil {
			log.Printf("new file: %v", err)
			mygo.Dialog.Error("Cannot create "+filepath.Base(path), err.Error())
			return
		}
		if path != "" {
			mygo.RunOnMain(func() { a.openPath(path) })
		}
	})
}

func (a *App) pickSavePath(parent *mygo.Window) (string, error) {
	if len(a.options.PickedPaths) > 0 {
		return a.options.PickedPaths[0], nil
	}
	return mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: parent, Title: newFileTitle, ButtonLabel: newFileButton, CreateDirectories: true})
}

func createFile(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, newFileMode)
	if err != nil {
		return err
	}
	return f.Close()
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
