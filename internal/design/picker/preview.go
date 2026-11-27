package picker

import (
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type PreviewOptions struct {
	Label   string
	Message string
	Busy    bool
}

func Preview(c *ui.Context, st *editor.State, o PreviewOptions) ui.Element {
	p := token.Of(c)
	box := ui.Column(c).FillWidth().Height(token.SearchPreviewHeight.Px()).MinHeight(token.RowHeight.Px()).Shrink(1).Clip().Radius(token.RadiusControl.Px()).
		Background(p.Layer.Editor).Border(token.BorderWidth.Px(), p.Border.Default).Label(o.Label)
	box.Children(func() {
		switch {
		case o.Busy:
			feedback.Loading(c, o.Message)
		case st == nil:
			feedback.Empty(c, feedback.EmptyOptions{Title: o.Message})
		default:
			editor.View(c.Key("picker-preview"), st).Grow(1).FillWidth().MinHeight(0).Label(o.Label)
		}
	})
	return box
}
