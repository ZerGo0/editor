package searchview

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fileicons"
	"github.com/egoist/mygo/ui"
)

const (
	maxFieldLines = 8
	beforeContext = 24
)

func fileLeading(name string) func(c *ui.Context) {
	return func(c *ui.Context) { fileicons.For(name).ViewSize(c, float64(token.IconSize.Px())).Shrink(0) }
}

func folderLeading(name string, open bool) func(c *ui.Context) {
	return func(c *ui.Context) { fileicons.Folder(name, open).ViewSize(c, float64(token.IconSize.Px())).Shrink(0) }
}

func formatCount(n int) string {
	if n >= ResultLimit {
		return itoa(ResultLimit) + "+"
	}
	return itoa(n)
}
