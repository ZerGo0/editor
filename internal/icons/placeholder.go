package icons

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func Placeholder(c *ui.Context, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Shrink(0).Radius(token.RadiusInline.Px()).Background(token.Of(c).Icon).AccessHidden(true)
}
