package workspace

import (
	"path/filepath"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/document"
)

func (w *Workspace) SaveAs() {
	d := w.activeTextDoc()
	if d == nil {
		return
	}
	w.active.ActiveTab().Preview = false
	w.saveAs(d)
}

func (w *Workspace) saveAs(d *document.Document) {
	if d.NotUTF8 {
		return
	}
	if isUntitled(d.Path) {
		w.saveUntitled([]*document.Document{d}, nil)
		return
	}
	if w.win == nil {
		return
	}
	w.askSavePath(d.Path, func(p string) {
		if p == d.Path {
			w.saveThen([]*document.Document{d}, nil)
			return
		}
		w.finishSaveAs(d, p)
	})
}

func (w *Workspace) RevertFile() {
	d := w.activeTextDoc()
	if d == nil || !onDisk(d.Path) {
		return
	}
	applog.Go(func() {
		s, err := d.ReadDisk()
		w.update(func() {
			if err != nil {
				w.showError("Cannot revert "+filepath.Base(d.Path), err)
				return
			}
			d.Buffer.MarkSaved()
			d.ApplyDisk(s)
		})
	})
}
