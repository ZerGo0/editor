package gitui

import (
	"github.com/egoist/mygo/ui"
)

const (
	iconButtonSize = 22
	iconSize       = 14
	buttonRadius   = 5
	smallFont      = 12
	bodyFont       = 13
	sectionFont    = 11
	rowHeight      = 22
	rowPadding     = 8
	toolbarHeight  = 30
	stateGap       = 8
	statePadding   = 16
	badgeRadius    = 4
	badgePaddingX  = 4
	badgeFont      = 11
	dialogGap      = 10
	dialogWidth    = 520
	wideDialog     = 720
	listHeight     = 260
	spinnerSize    = 16
	tintAlpha      = 0.14
	strongAlpha    = 0.28
)

func iconButton(c *ui.Context, label string, svg *ui.SVG, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(label)).Size(iconButtonSize, iconButtonSize).Radius(buttonRadius).Center().
		TextColor(t.TextMuted).Label(label).Tooltip(label).Disabled(disabled)
	if b.Hovered() && !disabled {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.Children(func() { ui.Icon(c, svg).Size(iconSize, iconSize) })
	return b
}

func textButton(c *ui.Context, label string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(label)).Height(iconButtonSize).Padding(0, 6).Radius(buttonRadius).Center().
		Label(label).Disabled(disabled)
	if b.Hovered() && !disabled {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		text := ui.Text(c, label).FontSize(smallFont).SingleLine()
		if disabled {
			text.TextColor(t.TextMuted)
		}
	})
	return b
}

func muted(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(smallFont).TextColor(c.Theme().TextMuted)
}

func emptyState(c *ui.Context, title, detail string) {
	t := c.Theme()
	ui.Column(c).FillWidth().Grow(1).Center().Gap(stateGap).Padding(statePadding).Children(func() {
		ui.Text(c, title).FontSize(bodyFont).TextColor(t.TextMuted).TextAlign(ui.Center)
		if detail != "" {
			ui.Text(c, detail).FontSize(smallFont).TextColor(t.TextMuted).TextAlign(ui.Center)
		}
	})
}

func loadingState(c *ui.Context, label string) {
	ui.Row(c).FillWidth().Grow(1).Center().Gap(stateGap).Padding(statePadding).Children(func() {
		ui.Spinner(c).Size(spinnerSize, spinnerSize).Label(label)
		muted(c, label)
	})
}

func errorState(c *ui.Context, title string, err error, retry func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Grow(1).Center().Gap(stateGap).Padding(statePadding).Children(func() {
		ui.Text(c, title).FontSize(bodyFont).Bold().TextAlign(ui.Center)
		if err != nil {
			ui.Text(c, err.Error()).FontSize(smallFont).TextColor(t.Danger).TextAlign(ui.Center)
		}
		if retry != nil && ui.Button(c, "Retry").Clicked() {
			retry()
		}
	})
}

func sectionTitle(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(sectionFont).Bold().TextColor(c.Theme().TextMuted).SingleLine()
}

func badge(c *ui.Context, s string, color ui.Color) {
	t := c.Theme()
	ui.Text(c, s).FontSize(badgeFont).SingleLine().Padding(0, badgePaddingX).Radius(badgeRadius).
		TextColor(color).Background(color.Alpha(tintAlpha)).BorderWidth(1).BorderColor(t.Border).Shrink(0)
}

func dialogButtons(c *ui.Context, cancel func(), action string, disabled bool, ok func()) {
	ui.Row(c).Gap(stateGap).Justify(ui.End).FillWidth().Children(func() {
		if ui.Button(c, "Cancel").Clicked() {
			cancel()
		}
		if ui.PrimaryButton(c, action).Disabled(disabled).Clicked() && !disabled {
			ok()
		}
	})
}

func modal(c *ui.Context, open *bool, title string, width float32, body func()) {
	if !*open {
		return
	}
	ui.Modal(c, open, func() {
		ui.Column(c).Gap(dialogGap).Width(width).Role(ui.RoleDialog).Label(title).Children(func() {
			ui.Text(c, title).Bold()
			body()
		})
	})
}
