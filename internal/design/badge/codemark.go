package badge

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type CodeMarkOptions struct {
	Font    ui.Font
	Tone    token.Tone
	Tooltip string
}

func CodeMarkWidth(c *ui.Context, text string, font ui.Font) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: text, Font: font.Family, Size: font.Size})
	return w
}

func CodeMark(c *ui.Context, e ui.Element, text string, o CodeMarkOptions) ui.Element {
	p := token.Of(c)
	fg, border := p.Text.Primary, p.Severity.Warning
	if o.Tone == token.ToneError {
		fg, border = p.Severity.Error, p.Severity.ErrorBorder
		e.Background(p.Severity.ErrorBackground)
	}
	e.AlignItems(ui.Center).Justify(ui.Center).Radius(token.RadiusInline.Px()).Border(token.BorderWidth.Px(), border)
	action.Tooltip(c, e, o.Tooltip)
	if text != "" {
		e.Children(func() {
			ui.Text(c, text).Font(o.Font.Family).FontSize(o.Font.Size).TextColor(fg).SingleLine().AccessHidden(true)
		})
	}
	return e
}
