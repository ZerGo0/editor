package prui

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
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
	checkRowGap   = 8
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

func stateColor(t *ui.Theme, pr github.PullRequest) ui.Color {
	switch prStateLabel(pr) {
	case prStateMerged:
		return t.Accent
	case prStateClosed:
		return t.Danger
	case prStateDraft:
		return t.TextMuted
	}
	return t.Success
}

func checkIcon(c *ui.Context, state checkState) {
	t := c.Theme()
	var svg *ui.SVG
	var color ui.Color
	switch state {
	case checkSuccess:
		svg, color = icons.Check, t.Success
	case checkFailure:
		svg, color = icons.Error, t.Danger
	case checkPending:
		svg, color = icons.Dot, t.Warning
	default:
		return
	}
	ui.Icon(c, svg).Size(iconSmall, iconSmall).TextColor(color).Shrink(0).Label(checkStateLabel(state))
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
	cardWith(c, title, count, nil, body)
}

func cardWith(c *ui.Context, title string, count int, extra, body func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Radius(cardRadius).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Padding(spacingMedium, cardPad).FillWidth().Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, title).Bold().FontSize(fontBody)
			if count > 0 {
				mutedPill(c, strconv.Itoa(count))
			}
			ui.Box(c).Grow(1)
			if extra != nil {
				extra()
			}
		})
		ui.Column(c).Gap(spacingMedium).Padding(cardPad).FillWidth().Children(body)
	})
}
