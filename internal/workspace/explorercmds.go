package workspace

import (
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/applog"
)

const (
	compareFilesPrefix = "compare-files:"
	compareIDSeparator = "\x00"
)

func (w *Workspace) explorerFocused() bool {
	return w.focus.side && w.tools.side == ExplorerView && !w.keys.textFocus
}

func (w *Workspace) selectionIsFolder() bool {
	p := w.tree.Selected()
	return p == "" || w.tree.IsDir(p)
}

func (w *Workspace) selectedFile() string {
	if p := w.tree.Selected(); p != "" && !w.tree.IsDir(p) {
		return p
	}
	return ""
}

func (w *Workspace) OpenSelectedToSide() {
	if p := w.selectedFile(); p != "" {
		w.openToSide(p)
	}
}

func (w *Workspace) FindInSelectedFolder() {
	if w.selectionIsFolder() {
		w.findInFolder(w.dirOf(w.tree.Selected()))
	}
}

func (w *Workspace) FindInExplorer() {
	w.ShowSideView(ExplorerView)
	w.tree.StartFind()
}

func (w *Workspace) SelectForCompare() {
	if p := w.selectedFile(); p != "" {
		w.compareSource = p
	}
}

func (w *Workspace) CompareWithSelected() {
	if p := w.selectedFile(); p != "" {
		w.compareWith(p)
	}
}

func (w *Workspace) canCompareWith(p string) bool {
	return w.compareSource != "" && w.compareSource != p
}

func (w *Workspace) compareWith(right string) {
	left := w.compareSource
	if !w.canCompareWith(right) {
		return
	}
	w.openIn(w.active, right, false, false)
	e := w.entry(right)
	var text string
	open := w.doc(left) != nil
	if open {
		text = w.doc(left).Buffer.String()
	}
	show := func() {
		w.whenLoaded(e, func() {
			title := filepath.Base(left) + compareArrow + filepath.Base(right)
			w.openCompare(compareFilesPrefix+left+compareIDSeparator+right, title, right, text, false)
		})
	}
	if open {
		show()
		return
	}
	applog.Go(func() {
		data, err := os.ReadFile(left)
		w.update(func() {
			if err != nil {
				w.showError("Cannot compare "+filepath.Base(left), err)
				return
			}
			text = string(data)
			show()
		})
	})
}

func (w *Workspace) whenLoaded(e *docEntry, fn func()) {
	if e.loading {
		e.ready = append(e.ready, fn)
		return
	}
	fn()
}
