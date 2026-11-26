package prui

import (
	"fmt"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func (p *PRs) StatusItems(c *ui.Context) {
	if p.closed || p.phase != phaseReady || p.current.pr == nil {
		return
	}
	pr := p.current.pr
	t := c.Theme()
	state := p.checks[pr.Number]
	label := fmt.Sprintf("Pull request #%d", pr.Number)
	tip := fmt.Sprintf("#%d %s", pr.Number, pr.Title)
	if text := checkStateLabel(state); text != "" {
		label += ", " + text
		tip += "\n" + text
	}
	b := ui.ButtonBase(c.Key("pr-status")).Gap(spacingSmall).Padding(0, spacingSmall).Radius(spacingSmall).AlignItems(ui.Center).
		TextColor(t.TextMuted).Label(label).Tooltip(tip)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, icons.PullRequest).Size(iconSmall, iconSmall)
		ui.Text(c, fmt.Sprintf("#%d", pr.Number)).FontSize(fontBody).SingleLine()
		checkIcon(c, state)
	})
	b.OnClick(func() { p.openPR(pr.Number, pr) })
}
