package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func Toolbar(c *ui.Context, name string) ui.Element {
	return ui.Row(c).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).Role(ui.RoleToolbar).Label(name + " Actions")
}
