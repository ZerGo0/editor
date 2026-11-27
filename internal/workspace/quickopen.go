package workspace

import (
	"github.com/egoist/mygo/ui"
)

func (w *Workspace) quickInputView(c *ui.Context) {
	w.palette.View(c)
	w.search.dialog.View(c)
	w.search.recentFiles.View(c)
}
