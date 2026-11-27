package prui

import (
	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/egoist/mygo/ui"
)

var stateWording = ghshared.StateWording{
	NoRepoDetail: "Open a folder that is a Git repository to see its pull requests.",
	ErrorTitle:   "Cannot load pull requests",
}

func (p *PRs) Ready() bool {
	return !p.Closed && p.Phase == ghshared.PhaseReady
}

func (p *PRs) StateView(c *ui.Context) bool {
	if p.Ready() {
		return false
	}
	if !p.Closed {
		p.stateView(c)
	}
	return true
}

func (p *PRs) stateView(c *ui.Context) {
	ghshared.StateView(c, p.StateMessage(p.repo.Host), p.Connect).Grow(1)
}
