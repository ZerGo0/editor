package icons

import (
	"github.com/ZerGo0/editor/internal/fileicons"
	"github.com/egoist/mygo/ui"
)

func FileIcon(c *ui.Context, name string, dir, open bool, size float32) ui.Element {
	icon := fileicons.For(name)
	if dir {
		icon = fileicons.Folder(name, open)
	}
	return icon.ViewSize(c, float64(size)).Shrink(0).AccessHidden(true)
}
