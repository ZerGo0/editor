package workspace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	newFilePerm  = 0o644
	dirPerm      = 0o755
	copySuffix   = " copy"
	maxCopyTries = 1000
)

var (
	errEmptyName  = errors.New("Enter a name.")
	errBadName    = errors.New("The name must stay inside the project.")
	errIntoItself = errors.New("A folder cannot move into itself.")
	errNoCopyName = errors.New("Cannot find a free name for the copy.")
)

func filetreeHandlers(w *Workspace) filetree.Handlers {
	return filetree.Handlers{
		Open:   w.openFromTree,
		Menu:   w.treeMenu,
		Create: w.createEntry,
		Rename: w.renameEntry,
		Trash:  w.trash,
		Paste:  w.pasteEntries,
		Move:   w.moveEntries,
		Leave:  func() { w.focusTab = w.active.ActiveTab() },
		Import: func(paths []string, dir string) { w.pasteEntries(paths, dir, false) },
		Notify: w.setNotice,
		Marker: w.problemMarker,
	}
}

func (w *Workspace) openFromTree(p string, preview, focus bool) {
	w.OpenFile(p, preview, focus)
}

func (w *Workspace) treeMenu(m *ui.Menu, p string) {
	dir := w.dirOf(p)
	if m.Item("New File...").Chosen() {
		w.startCreate(dir, false)
	}
	if m.Item("New Folder...").Chosen() {
		w.startCreate(dir, true)
	}
	m.Separator()
	if m.Item("Reveal in Finder").Chosen() {
		w.revealInFinder(p)
	}
	if m.Item("Open in Terminal").Chosen() {
		w.openInTerminal(dir)
	}
	m.Separator()
	root := p == w.Root
	if m.Item("Cut").Shortcut(ui.Cmd, ui.KeyX).Disabled(root).Chosen() {
		w.tree.SetClipboard([]string{p}, true)
	}
	if m.Item("Copy").Shortcut(ui.Cmd, ui.KeyC).Disabled(root).Chosen() {
		w.tree.SetClipboard([]string{p}, false)
	}
	clip, cut := w.tree.Clipboard()
	if m.Item("Paste").Shortcut(ui.Cmd, ui.KeyV).Disabled(len(clip) == 0).Chosen() {
		w.pasteEntries(clip, dir, cut)
		if cut {
			w.tree.ClearClipboard()
		}
	}
	m.Separator()
	w.pathItems(m, p)
	if root {
		return
	}
	m.Separator()
	if m.Item("Rename...").Shortcut(0, ui.KeyF2).Chosen() {
		w.tree.StartRename(p)
	}
	if m.Item("Move to Trash").Shortcut(ui.Cmd, ui.KeyBackspace).Chosen() {
		w.trash(p)
	}
}

func (w *Workspace) dirOf(p string) string {
	switch {
	case p == "":
		return w.Root
	case w.tree.IsDir(p):
		return p
	}
	return filepath.Dir(p)
}

func (w *Workspace) selectionDir() string {
	if p := w.tree.Selected(); p != "" {
		return w.dirOf(p)
	}
	if t := w.active.ActiveTab(); t != nil && !isVirtual(t.Path) && fsutil.Within(w.Root, t.Path) {
		return filepath.Dir(t.Path)
	}
	return w.Root
}

func (w *Workspace) NewFile() {
	w.startCreate(w.selectionDir(), false)
}

func (w *Workspace) NewFolder() {
	w.startCreate(w.selectionDir(), true)
}

func (w *Workspace) startCreate(dir string, folder bool) {
	fromEditor := !w.focus.side && w.active.ActiveTab() != nil
	w.ShowSideView(ExplorerView)
	w.tools.explorerCollapsed = false
	w.tree.StartCreate(dir, folder, fromEditor)
}

func (w *Workspace) resolveName(dir, name string) (string, error) {
	if name == "" {
		return "", errEmptyName
	}
	p := filepath.Join(dir, name)
	if !filepath.IsLocal(name) || !fsutil.Within(w.Root, p) {
		return "", errBadName
	}
	if _, err := os.Lstat(p); err == nil {
		return "", fmt.Errorf("%s already exists.", filepath.Base(p))
	}
	return p, nil
}

func (w *Workspace) createEntry(dir, name string, folder bool) error {
	p, err := w.resolveName(dir, name)
	if err != nil {
		return err
	}
	if folder {
		err = os.MkdirAll(p, dirPerm)
	} else {
		err = createFile(p)
	}
	if err != nil {
		return err
	}
	w.tree.Invalidate(dir)
	w.tree.Reveal(p)
	if !folder {
		w.OpenFile(p, false, true)
	}
	return nil
}

func createFile(p string) error {
	if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, newFilePerm)
	if err != nil {
		return err
	}
	return f.Close()
}

func (w *Workspace) renameEntry(p, name string) error {
	to, err := w.resolveName(filepath.Dir(p), name)
	if err != nil {
		return err
	}
	return w.movePath(p, to)
}

func (w *Workspace) movePath(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	w.moved(from, to)
	w.historyRenamed(from, to)
	w.tree.Invalidate(filepath.Dir(from))
	w.tree.Invalidate(filepath.Dir(to))
	w.tree.Reveal(to)
	return nil
}

func (w *Workspace) moveEntries(paths []string, dir string) {
	for _, p := range paths {
		if filepath.Dir(p) == dir {
			continue
		}
		if err := w.moveInto(p, dir); err != nil {
			w.setNotice("Cannot move " + filepath.Base(p) + ": " + err.Error())
			return
		}
	}
}

func (w *Workspace) moveInto(p, dir string) error {
	if p == dir || fsutil.Within(p, dir) {
		return errIntoItself
	}
	to, err := w.resolveName(dir, filepath.Base(p))
	if err != nil {
		return err
	}
	return w.movePath(p, to)
}

func (w *Workspace) pasteEntries(paths []string, dir string, cut bool) {
	if cut {
		w.moveEntries(paths, dir)
		return
	}
	applog.Go(func() {
		var last string
		for _, p := range paths {
			to, err := copyEntry(p, dir)
			if err != nil {
				w.showError("Cannot paste "+filepath.Base(p), err)
				break
			}
			last = to
		}
		w.update(func() {
			w.tree.Invalidate(dir)
			if last != "" {
				w.tree.Reveal(last)
			}
		})
	})
}

func copyEntry(src, dir string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	to, err := freeCopyName(dir, filepath.Base(src), func(p string) bool {
		_, err := os.Lstat(p)
		return err == nil
	})
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		if fsutil.Within(src, dir) {
			return "", errIntoItself
		}
		return to, os.CopyFS(to, os.DirFS(src))
	}
	return to, copyFile(src, to, info.Mode().Perm())
}

func freeCopyName(dir, name string, exists func(string) bool) (string, error) {
	if p := filepath.Join(dir, name); !exists(p) {
		return p, nil
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem, ext = name, ""
	}
	for i := range maxCopyTries {
		suffix := copySuffix
		if i > 0 {
			suffix += " " + strconv.Itoa(i+1)
		}
		if p := filepath.Join(dir, stem+suffix+ext); !exists(p) {
			return p, nil
		}
	}
	return "", errNoCopyName
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func errorWithOutput(err error, out []byte) error {
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return fmt.Errorf("%w: %s", err, msg)
	}
	return err
}

func (w *Workspace) moved(from, to string) {
	remap := func(p string) (string, bool) {
		if p == from {
			return to, true
		}
		if fsutil.Within(from, p) {
			return filepath.Join(to, strings.TrimPrefix(p, from+string(filepath.Separator))), true
		}
		return p, false
	}
	for path, e := range w.docs {
		if np, ok := remap(path); ok {
			delete(w.docs, path)
			w.deleteBackup(path)
			w.docs[np] = e
			if e.doc != nil {
				e.doc.Rename(np)
			}
		}
	}
	for _, t := range w.tabs() {
		if np, ok := remap(t.Path); ok {
			t.Path = np
			w.dropViewer(t)
		}
	}
}

func (w *Workspace) trash(p string) {
	name := filepath.Base(p)
	applog.Go(func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             w.win,
			Type:               mygo.MessageWarning,
			Message:            fmt.Sprintf("Move %q to the Trash?", name),
			Detail:             "You can restore it from the Trash.",
			Buttons:            []string{"Move to Trash", "Cancel"},
			CancelButton:       1,
			DestructiveButtons: []int{0},
		})
		if err != nil || res.Button != 0 {
			return
		}
		if err := mygo.Shell.TrashItem(p); err != nil {
			w.showError("Cannot move "+name+" to the Trash", err)
			return
		}
		w.update(func() { w.removed(p) })
	})
}

func (w *Workspace) removed(p string) {
	for _, g := range w.layout.Groups() {
		var gone []*layout.Tab
		for _, t := range g.Tabs {
			if t.Path == p || fsutil.Within(p, t.Path) {
				gone = append(gone, t)
			}
		}
		if len(gone) > 0 {
			w.removeTabs(g, gone)
		}
	}
	w.tree.Invalidate(filepath.Dir(p))
}
