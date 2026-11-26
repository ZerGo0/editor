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
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            parent,
			Title:             "Open Folder",
			Directory:         true,
			CreateDirectories: true,
		})
		if err != nil {
			log.Printf("open folder: %v", err)
			return
		}
		if len(paths) > 0 {
			mygo.RunOnMain(func() { a.openPath(paths[0]) })
		}
	}()
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil {
		if r, err := filepath.Rel(home, path); err == nil && filepath.IsLocal(r) {
			return filepath.Join("~", r)
		}
	}
	return path
}
