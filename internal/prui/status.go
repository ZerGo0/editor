package prui

import (
	"fmt"

	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func (p *PRs) StatusItems(c *ui.Context) {
	if p.Closed || p.Phase != ghshared.PhaseReady || p.current.pr == nil {
		return
	}
	pr := p.current.pr
	state := p.checks[pr.Number]
	label := fmt.Sprintf("Pull request #%d", pr.Number)
	tip := fmt.Sprintf("#%d %s", pr.Number, pr.Title)
	if text := checkStateLabel(state); text != "" {
		label += ", " + text
		tip += "\n" + text
	}
	_, tone, _ := checkAppearance(state)
	chrome.StatusItem(c, fmt.Sprintf("#%d", pr.Number), chrome.StatusOptions{Icon: icons.PullRequest, Label: label, Tooltip: tip, Tone: statusTone(tone),
		OnClick: func() { p.openPR(pr.Number, pr) }})
}

func statusTone(t token.Tone) token.Tone {
	if t == token.ToneSuccess {
		return token.ToneNeutral
	}
	return t
}
