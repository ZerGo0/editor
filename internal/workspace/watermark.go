package workspace

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/resources"
	"github.com/egoist/mygo/ui"
)

const watermarkLabel = "No open files"

type watermarkHint struct {
	label, command string
}

var watermarkHints = []watermarkHint{
	{label: "Show All Commands", command: "workbench.action.showCommands"},
	{label: "Go to File", command: "workbench.action.quickOpen"},
	{label: "Search Everywhere", command: "workbench.action.searchEverywhere"},
	{label: "Find in Files", command: "workbench.action.findInFiles"},
	{label: "Open Settings", command: "workbench.action.openSettings"},
}

func (w *Workspace) watermark(c *ui.Context) {
	ui.Column(c).Grow(1).FillWidth().Center().Gap(token.GapSection.Px()).Role(ui.RoleRegion).Label(watermarkLabel).Children(func() {
		if logo := resources.Logo(); logo != nil {
			ui.Image(c, logo).Size(token.WatermarkLogoSize.Px(), token.WatermarkLogoSize.Px()).Opacity(token.OpacityDisabled.Value()).
				AccessHidden(true)
		}
		ui.Column(c).Width(token.WatermarkWidth.Px()).Gap(token.GapStack.Px()).Children(func() {
			for _, h := range watermarkHints {
				keys := w.keyLabel(h.command)
				if keys == "" {
					continue
				}
				ui.Row(c).FillWidth().Gap(token.GapGroup.Px()).AlignItems(ui.Center).Children(func() {
					design.Text(c, h.label, token.TextLabel).Grow(1).Basis(0).TextAlign(ui.End).SingleLine()
					ui.Row(c).Grow(1).Basis(0).Children(func() { badge.Keys(c, keys) })
				})
			}
		})
	})
}
