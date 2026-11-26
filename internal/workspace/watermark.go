package workspace

import (
	"github.com/egoist/mygo/ui"
)

const (
	watermarkLabel      = "No open files"
	watermarkRowHeight  = 28
	watermarkGap        = 2
	watermarkLabelWidth = 170
	watermarkKeysWidth  = 120
	watermarkColumnGap  = 12
	watermarkRowPadding = 8
	keycapPadding       = 5
	keycapHeight        = 18
	keycapRadius        = 4
	doubleShiftLabel    = "Double ⇧"
)

type watermarkHint struct {
	label, command, keys string
}

var watermarkHints = []watermarkHint{
	{label: "Show All Commands", command: "workbench.action.showCommands"},
	{label: "Go to File", command: "workbench.action.quickOpen"},
	{label: "Search Everywhere", command: "workbench.action.searchEverywhere", keys: doubleShiftLabel},
	{label: "Find in Files", command: "workbench.action.findInFiles"},
	{label: "Open Settings", command: "workbench.action.openSettings"},
}

func (w *Workspace) watermark(c *ui.Context) {
	ui.Column(c).Grow(1).FillWidth().Center().Gap(watermarkGap).Label(watermarkLabel).Children(func() {
		for _, h := range watermarkHints {
			keys := h.keys
			if keys == "" {
				keys = w.keyLabel(h.command)
			}
			w.watermarkRow(c, h, keys)
		}
	})
}

func (w *Workspace) watermarkRow(c *ui.Context, h watermarkHint, keys string) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(h.command)).Height(watermarkRowHeight).Padding(0, watermarkRowPadding).Gap(watermarkColumnGap).
		AlignItems(ui.Center).Radius(iconButtonRadius).Label(h.label)
	color := t.TextMuted
	if pressable(b, t) {
		color = t.Text
	}
	b.Children(func() {
		ui.Text(c, h.label).TextColor(color).SingleLine().Width(watermarkLabelWidth).TextAlign(ui.End)
		ui.Row(c).Width(watermarkKeysWidth).Children(func() {
			if keys != "" {
				keycap(c, keys)
			}
		})
	})
	b.OnClick(func() { w.run(h.command) })
}

func keycap(c *ui.Context, keys string) {
	t := c.Theme()
	ui.Text(c, keys).FontSize(smallFontSize).TextColor(t.TextMuted).SingleLine().Height(keycapHeight).Padding(0, keycapPadding).
		Radius(keycapRadius).Background(t.Surface).Border(1, t.Border)
}
