package findwidget

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type Button struct {
	Icon  *ui.SVG
	Label string
	Keys  string
}

var (
	PreviousMatch   = Button{Icon: icons.ArrowUp, Label: "Previous Match", Keys: "⇧↩"}
	NextMatch       = Button{Icon: icons.ArrowDown, Label: "Next Match", Keys: "↩"}
	FindInSelection = Button{Icon: icons.Selection, Label: "Find in Selection", Keys: "⌥⌘L"}
	Close           = Button{Icon: icons.Close, Label: "Close", Keys: "Esc"}
	ReplaceOne      = Button{Icon: icons.Replace, Label: "Replace", Keys: "↩"}
	ReplaceAll      = Button{Icon: icons.ReplaceAll, Label: "Replace All", Keys: "⌘↩"}
)

func (b Button) Clicked(c *ui.Context, disabled bool) bool {
	return action.IconButton(c, b.Icon, b.Label, action.IconOptions{Keys: b.Keys, Disabled: disabled, KeepFocus: true}).Clicked()
}

func (b Button) Toggle(c *ui.Context, on *bool) ui.Element {
	return action.ToggleIconButton(c, b.Icon, b.Label, on, action.IconOptions{Keys: b.Keys, KeepFocus: true})
}
