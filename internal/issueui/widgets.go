package issueui

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	fontBody      = 12
	fontSmall     = 11
	fontTitle     = 16
	fontBadge     = 10
	iconSmall     = 12
	iconMedium    = 14
	buttonSize    = 24
	rowPadX       = 10
	rowPadY       = 6
	cardPad       = 10
	cardRadius    = 6
	pillRadius    = 8
	pillPadX      = 6
	spacingSmall  = 4
	spacingMedium = 8
	spacingLarge  = 12
	messagePad    = 16
	messageGap    = 6
	contentMax    = 960
	rowIconGap    = 2
)

func iconButton(c *ui.Context, key, label string, svg *ui.SVG) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Size(buttonSize, buttonSize).Radius(cardRadius).Center().
		TextColor(t.TextMuted).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, svg).Size(iconMedium, iconMedium) })
	return b
}

func pill(c *ui.Context, text string, bg ui.Color) ui.Element {
	return ui.Text(c, text).FontSize(fontBadge).Bold().SingleLine().TextColor(c.Theme().AccentText).Background(bg).Radius(pillRadius).Padding(1, pillPadX).Shrink(0)
}

func mutedPill(c *ui.Context, text string) ui.Element {
	t := c.Theme()
	return ui.Text(c, text).FontSize(fontBadge).SingleLine().TextColor(t.TextMuted).Background(t.SurfacePressed).Radius(pillRadius).Padding(1, pillPadX).Shrink(0)
}

func labelPill(c *ui.Context, label github.Label) ui.Element {
	t := c.Theme()
	bg, fg := labelColors(label.Color, t.SurfacePressed, t.TextMuted)
	return ui.Text(c, label.Name).FontSize(fontBadge).SingleLine().TextColor(fg).Background(bg).Radius(pillRadius).Padding(1, pillPadX).Shrink(0)
}

func mutedText(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(fontSmall).TextColor(c.Theme().TextMuted)
}

func loadingRow(c *ui.Context, text string) {
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Padding(rowPadY, rowPadX).Children(func() {
		ui.Spinner(c).Label(text)
		mutedText(c, text)
	})
}

func errorBlock(c *ui.Context, err error, retry func()) {
	t := c.Theme()
	ui.Column(c).Gap(messageGap).Padding(rowPadY, rowPadX).FillWidth().Children(func() {
		ui.Text(c, errorText(err)).FontSize(fontBody).TextColor(t.Danger)
		ui.Button(c, "Retry").OnClick(retry)
	})
}

func card(c *ui.Context, title string, count int, body func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Radius(cardRadius).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Padding(spacingMedium, cardPad).FillWidth().Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, title).Bold().FontSize(fontBody)
			if count > 0 {
				mutedPill(c, strconv.Itoa(count))
			}
		})
		ui.Column(c).Gap(spacingMedium).Padding(cardPad).FillWidth().Children(body)
	})
}
