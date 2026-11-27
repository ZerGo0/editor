package picker

import (
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type PreviewOptions struct {
	Label   string
	Title   string
	Message string
	Busy    bool
	Layout  *Layout
	Results *ResultsState
}

func Preview(c *ui.Context, st *editor.State, o PreviewOptions) ui.Element {
	p := token.Of(c)
	height, floor := token.SearchPreviewHeight.Px(), token.RowHeight.Px()
	if o.Layout != nil {
		height, floor = o.Layout.previewHeight(), token.PaneMinHeight.Px()
	}
	pane := ui.Column(c.Key("picker-preview-pane")).FillWidth().Height(height).MinHeight(floor).Shrink(1)
	pane.Children(func() {
		box := ui.Column(c).FillWidth().Grow(1).MinHeight(0).Clip().Radius(token.RadiusControl.Px()).
			Background(p.Layer.Editor).Border(token.BorderWidth.Px(), p.Border.Default).Label(o.Label)
		box.Children(func() {
			previewTitle(c, p, o.Title)
			switch {
			case o.Busy:
				feedback.Loading(c, o.Message)
			case st == nil:
				feedback.Empty(c, feedback.EmptyOptions{Title: o.Message})
			default:
				editor.View(c.Key("picker-preview"), st).Grow(1).FillWidth().MinHeight(0).Label(o.Label)
			}
		})
		if o.Layout != nil && o.Results != nil {
			splitter(c, p, pane, o)
		}
	})
	return pane
}

func previewTitle(c *ui.Context, p *token.Palette, title string) {
	ui.Row(c).FillWidth().Height(token.RowHeight.Px()).Shrink(0).AlignItems(ui.Center).Padding(0, token.PadRow.Px()).
		BorderWidth(0, 0, token.SeparatorWidth.Px(), 0).BorderColor(p.Border.Section).Children(func() {
		token.TextLabel.Apply(ui.Text(c, title)).TextColor(p.Text.Secondary).SingleLine().MinWidth(0).Shrink(1)
	})
}

func splitter(c *ui.Context, p *token.Palette, pane ui.Element, o PreviewOptions) {
	sash := ui.Box(c.Key("picker-splitter")).Absolute().Left(0).Right(0).Top(-token.PadPopover.Px()).Height(token.SashSize.Px()).
		Cursor(ui.CursorResizeRow).ZIndex(token.LayerSticky.Z()).Role(ui.RoleSplitter).Label("Resize " + o.Label)
	if sash.Hovered() || sash.Dragging() {
		sash.Background(p.Accent.Focus)
	}
	if _, dy, ok := sash.Dragged(); ok && dy != 0 && o.Results.height > 0 {
		o.Layout.resizePreview(pane.Bounds().H, o.Results.height, dy)
	}
}
