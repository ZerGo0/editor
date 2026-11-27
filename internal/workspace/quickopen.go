package workspace

import (
	"github.com/egoist/mygo/ui"
)

func (w *Workspace) quickInputView(c *ui.Context) {
	open := w.palette.IsOpen() || w.searchOpen()
	w.palette.View(c)
	w.search.dialog.View(c)
	w.search.recentFiles.View(c)
	if open && !w.palette.IsOpen() && !w.searchOpen() && w.focusTab == nil {
		w.focusTab = w.active.ActiveTab()
		c.Invalidate()
	}
}
