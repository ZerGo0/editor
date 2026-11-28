package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
	"github.com/ZerGo0/editor/internal/workspace/notices"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	newFilePerm  = 0o644
	dirPerm      = 0o755
	copySuffix   = " copy"
	maxCopyTries = 1000
	ownerAccess  = 0o700

	newFileLabel       = "New File"
	newFolderLabel     = "New Folder"
	renameLabel        = "Rename"
	moveLabel          = "Move"
	pasteLabel         = "Paste"
	trashLabel         = "Move to Trash"
	showHistoryLabel   = "Show History"
	openToSideID       = "explorer.openToSide"
	findInFolderID     = "filesExplorer.findInFolder"
	selectForCompareID = "selectForCompare"
	compareFilesID     = "compareFiles"
	openFolderID       = "workbench.action.files.openFolder"
	closeWindowID      = "workbench.action.closeWindow"
)

var (
	errEmptyName  = errors.New("Enter a name.")
	errBadName    = errors.New("The name must stay inside the project.")
	errSlashName  = errors.New("A file or folder name cannot end with a slash.")
	errIntoItself = errors.New("A folder cannot move into itself.")
	errNoCopyName = errors.New("Cannot find a free name for the copy.")
)

func filetreeHandlers(w *Workspace) filetree.Handlers {
	return filetree.Handlers{
		Open:       w.openFromTree,
		Menu:       w.treeMenu,
		Create:     w.createEntry,
		Rename:     w.renameEntry,
		Trash:      w.trash,
		Paste:      w.pasteEntries,
		Move:       w.moveEntries,
		Leave:      func() { w.focusTab = w.active.ActiveTab() },
		Import:     func(paths []string, dir string) { w.pasteEntries(paths, dir, false) },
		Error:      func(title string, err error) { w.showError(title, err) },
		Validate:   func(dir, name string) error { _, err := w.resolveName(dir, name); return err },
		OpenFolder: func() { w.run(openFolderID) },
		Close:      func() { w.run(closeWindowID) },
		Marker:     w.problemMarker,
	}
}

func (w *Workspace) openFromTree(p string, preview, focus bool) {
	w.OpenFile(p, preview, focus)
}

func (w *Workspace) openToSide(p string) {
	w.openIn(w.sideGroup(), p, false, true)
}

func (w *Workspace) findInFolder(dir string) {
	w.closeSearchPopups()
	st := w.search.dialog.Snapshot()
	st.Scope, st.Directory = finder.ScopeDirectory, w.rel(dir)
	if dir == w.Root {
		st.Scope, st.Directory = finder.ScopeProject, ""
	}
	w.search.dialog.Restore(st)
	w.search.dialog.Open(finder.TabText, "")
}

func (w *Workspace) showFileHistory(p string) {
	if h := w.historyUI(); h != nil {
		h.ShowHistory(p)
	}
}

func (w *Workspace) treeMenu(m *ui.Menu, p string) {
	dir := w.dirOf(p)
	root := p == w.Root
	file := !root && !w.tree.IsDir(p)
	targets := w.tree.Targets(p)
	if m.Item(newFileLabel + "...").Chosen() {
		w.startCreate(dir, false)
	}
	if m.Item(newFolderLabel + "...").Chosen() {
		w.startCreate(dir, true)
	}
	m.Separator()
	if file && commandItem(m, openToSideID).Chosen() {
		w.openToSide(p)
	}
	if m.Item("Reveal in Finder").Chosen() {
		w.revealInFinder(p)
	}
	if m.Item("Open in Terminal").Chosen() {
		w.openInTerminal(dir)
	}
	if !file && commandItem(m, findInFolderID).Chosen() {
		w.findInFolder(dir)
	}
	m.Separator()
	if m.Item("Cut").Shortcut(ui.Cmd, ui.KeyX).Disabled(root).Chosen() {
		w.tree.SetClipboard(targets, true)
	}
	if m.Item("Copy").Shortcut(ui.Cmd, ui.KeyC).Disabled(root).Chosen() {
		w.tree.SetClipboard(targets, false)
	}
	clip, cut := w.tree.Clipboard()
	if m.Item("Paste").Shortcut(ui.Cmd, ui.KeyV).Disabled(len(clip) == 0).Chosen() {
		w.pasteEntries(clip, dir, cut)
		if cut {
			w.tree.ClearClipboard()
		}
	}
	if file {
		if commandItem(m, selectForCompareID).Chosen() {
			w.compareSource = p
		}
		if w.canCompareWith(p) && commandItem(m, compareFilesID).Chosen() {
			w.compareWith(p)
		}
	}
	m.Separator()
	w.pathItems(m, p)
	if file && m.Item(showHistoryLabel).Chosen() {
		w.showFileHistory(p)
	}
	if root {
		return
	}
	m.Separator()
	if m.Item(renameLabel+"...").Shortcut(0, ui.KeyF2).Disabled(len(targets) > 1).Chosen() {
		w.tree.StartRename(p)
	}
	if m.Item(trashLabel).Shortcut(ui.Cmd, ui.KeyBackspace).Chosen() {
		w.trash(targets)
	}
}

func commandItem(m *ui.Menu, id string) *ui.MenuItem {
	info, _ := action.Lookup(id)
	return action.Item(m, info.Title, action.ItemOptions{Detail: info.Keys, Disabled: info.Disabled, ID: id})
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
	if strings.HasSuffix(name, "/") {
		return "", errSlashName
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
	label := newFileLabel
	if folder {
		label = newFolderLabel
	}
	w.tree.Record(&filetree.Change{Label: label, Created: []filetree.Trashed{{Path: p}}})
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
	if err := w.movePath(p, to); err != nil {
		return err
	}
	w.tree.Record(&filetree.Change{Label: renameLabel, Moves: []filetree.Move{{From: p, To: to}}})
	return nil
}

func (w *Workspace) movePath(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	w.afterMove(from, to)
	return nil
}

func (w *Workspace) afterMove(from, to string) {
	w.moved(from, to)
	w.historyRenamed(from, to)
	w.tree.Invalidate(filepath.Dir(from))
	w.tree.Invalidate(filepath.Dir(to))
	w.tree.Reveal(to)
}

func (w *Workspace) moveEntries(paths []string, dir string) {
	ch := &filetree.Change{Label: moveLabel}
	defer func() {
		if len(ch.Moves) > 0 {
			w.tree.Record(ch)
		}
	}()
	for _, p := range paths {
		if filepath.Dir(p) == dir {
			continue
		}
		to, err := w.moveInto(p, dir)
		if err != nil {
			w.setNotice("Cannot move " + filepath.Base(p) + ": " + err.Error())
			return
		}
		ch.Moves = append(ch.Moves, filetree.Move{From: p, To: to})
	}
}

func (w *Workspace) moveInto(p, dir string) (string, error) {
	if p == dir || fsutil.Within(p, dir) {
		return "", errIntoItself
	}
	to, err := w.resolveName(dir, filepath.Base(p))
	if err != nil {
		return "", err
	}
	return to, w.movePath(p, to)
}

func (w *Workspace) pasteEntries(paths []string, dir string, cut bool) {
	if cut {
		w.moveEntries(paths, dir)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	progress := &pasteProgress{}
	timer := time.AfterFunc(token.SpinnerDelay, func() {
		id := w.notify(notices.Notice{Tone: token.ToneInfo, Title: pasteTitle(paths), Busy: true, Actions: []notices.Action{{Label: cancelLabel, Run: cancel}}})
		if !progress.show(id) {
			w.notes.Close(id)
		}
	})
	applog.Go(func() {
		ch := &filetree.Change{Label: pasteLabel}
		for _, p := range paths {
			to, err := copyEntry(ctx, p, dir)
			if errors.Is(err, context.Canceled) {
				break
			}
			if err != nil {
				w.showError("Cannot paste "+filepath.Base(p), err)
				break
			}
			ch.Created = append(ch.Created, filetree.Trashed{Path: to})
		}
		timer.Stop()
		cancel()
		if id := progress.finish(); id != "" {
			w.notes.Close(id)
		}
		w.update(func() {
			w.tree.Invalidate(dir)
			if n := len(ch.Created); n > 0 {
				w.tree.Record(ch)
				w.tree.Reveal(ch.Created[n-1].Path)
			}
		})
	})
}

type pasteProgress struct {
	mu   sync.Mutex
	id   string
	done bool
}

func (p *pasteProgress) show(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.id = id
	return !p.done
}

func (p *pasteProgress) finish() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done = true
	return p.id
}

func pasteTitle(paths []string) string {
	if len(paths) == 1 {
		return "Copying " + filepath.Base(paths[0]) + "..."
	}
	return "Copying " + strconv.Itoa(len(paths)) + " items..."
}

func copyEntry(ctx context.Context, src, dir string) (string, error) {
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
		return to, copyTree(ctx, src, to, info.Mode().Perm())
	}
	return to, copyFile(ctx, src, to, info.Mode().Perm())
}

func copyTree(ctx context.Context, src, dst string, perm os.FileMode) error {
	if err := os.Mkdir(dst, perm|ownerAccess); err != nil {
		return err
	}
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.IsDir():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.Mkdir(target, info.Mode().Perm()|ownerAccess)
		case d.Type().IsRegular():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return copyFile(ctx, p, target, info.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		_ = os.RemoveAll(dst)
	}
	return err
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
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

func copyFile(ctx context.Context, src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, ctxReader{ctx: ctx, r: in})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(dst)
	}
	return err
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

func (w *Workspace) trash(paths []string) {
	if len(paths) == 0 {
		return
	}
	message, detail := fmt.Sprintf("Move %q to the Trash?", filepath.Base(paths[0])), "You can restore it from the Trash."
	if len(paths) > 1 {
		message, detail = fmt.Sprintf("Move %d items to the Trash?", len(paths)), "You can restore them from the Trash."
	}
	applog.Go(func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             w.win,
			Type:               mygo.MessageWarning,
			Message:            message,
			Detail:             detail,
			Buttons:            []string{trashLabel, "Cancel"},
			CancelButton:       1,
			DestructiveButtons: []int{0},
		})
		if err != nil || res.Button != 0 {
			return
		}
		ch := &filetree.Change{Label: trashLabel}
		for _, p := range paths {
			trashed, err := mygo.Shell.TrashItemResult(p)
			if err != nil {
				w.showError("Cannot move "+filepath.Base(p)+" to the Trash", err)
				break
			}
			ch.Removed = append(ch.Removed, filetree.Trashed{Path: p, Trash: trashed})
		}
		w.update(func() {
			for _, e := range ch.Removed {
				w.removed(e.Path)
			}
			if len(ch.Removed) > 0 {
				w.tree.Record(ch)
			}
		})
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
