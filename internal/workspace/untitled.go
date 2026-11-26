package workspace

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/document"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
)

const (
	untitledPrefix = "untitled:"
	untitledName   = "Untitled-"
	defaultSaveExt = ".txt"
	saveAsTitle    = "Save As"
)

func isUntitled(p string) bool {
	return strings.HasPrefix(p, untitledPrefix)
}

func onDisk(p string) bool {
	return !isVirtual(p) && !isUntitled(p)
}

func (w *Workspace) NewUntitled() {
	w.OpenFile(w.freeUntitledPath(), false, true)
}

func (w *Workspace) freeUntitledPath() string {
	used := map[string]bool{}
	for _, t := range w.tabs() {
		used[t.Path] = true
	}
	for p := range w.docs {
		used[p] = true
	}
	for n := 1; ; n++ {
		p := untitledPrefix + untitledName + strconv.Itoa(n)
		if !used[p] && !w.hasBackup(p) {
			return p
		}
	}
}

func splitUntitled(docs []*document.Document) (files, untitled []*document.Document) {
	for _, d := range docs {
		if isUntitled(d.Path) {
			untitled = append(untitled, d)
		} else {
			files = append(files, d)
		}
	}
	return files, untitled
}

func (w *Workspace) saveThen(docs []*document.Document, done func()) {
	files, untitled := splitUntitled(docs)
	w.confirmOverwrite(files, func() {
		if w.save(files) {
			w.saveUntitled(untitled, done)
		}
	})
}

func (w *Workspace) saveUntitled(docs []*document.Document, done func()) {
	if len(docs) == 0 {
		if done != nil {
			done()
		}
		return
	}
	if w.win == nil {
		return
	}
	d := docs[0]
	name := w.tabTitle(d.Path) + extensionOf(d.Buffer.Language())
	go func() {
		p, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:            w.win,
			Title:             saveAsTitle,
			DefaultPath:       filepath.Join(w.Root, name),
			CreateDirectories: true,
		})
		if err != nil {
			w.showError("Cannot save "+name, err)
			return
		}
		if p == "" {
			return
		}
		w.update(func() {
			if w.finishSaveAs(d, p) {
				w.saveUntitled(docs[1:], done)
			}
		})
	}()
}

func extensionOf(lang *editor.Language) string {
	if lang != nil && len(lang.Extensions) > 0 {
		return lang.Extensions[0]
	}
	return defaultSaveExt
}

func (w *Workspace) finishSaveAs(d *document.Document, p string) bool {
	old := d.Path
	if e, ok := w.docs[old]; !ok || e.doc != d {
		return false
	}
	if _, open := w.docs[p]; open {
		w.showError("Cannot save "+w.tabTitle(old), fmt.Errorf("%s is open in another tab. Close it first.", filepath.Base(p)))
		return false
	}
	lang := d.Buffer.Language()
	w.moved(old, p)
	if d.Buffer.Language() == nil {
		d.Buffer.SetLanguage(lang)
	}
	if !w.save([]*document.Document{d}) {
		return false
	}
	w.tree.Invalidate(filepath.Dir(p))
	w.refreshEditorConfig([]string{p})
	return true
}
