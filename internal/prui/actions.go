package prui

import (
	"fmt"

	"github.com/ZerGo0/editor/internal/ghshared"
)

const actionCategory = "GitHub"

type Action struct {
	ID       string
	Title    string
	Category string
	Key      string
}

const (
	ActionRefresh      = "pr.refresh"
	ActionCreate       = "pr.create"
	ActionOpenCurrent  = "pr.openCurrent"
	ActionCheckout     = "pr.checkout"
	ActionSearch       = "pr.search"
	ActionSaveQuery    = "pr.saveQuery"
	ActionExitReview   = "pr.exitReview"
	ActionCloseEditors = "pr.closeEditors"
	ActionCopyLink     = "pr.copyLink"
	ActionOpenOnGitHub = "pr.openOnGitHub"
)

var Actions = []Action{
	{ID: ActionRefresh, Title: "Refresh Pull Requests", Category: actionCategory},
	{ID: ActionCreate, Title: "Create Pull Request", Category: actionCategory},
	{ID: ActionOpenCurrent, Title: "Open Pull Request of Current Branch", Category: actionCategory},
	{ID: ActionCheckout, Title: "Checkout Pull Request...", Category: actionCategory},
	{ID: ActionSearch, Title: "Search Pull Requests...", Category: actionCategory},
	{ID: ActionSaveQuery, Title: "Save Current Search as Query...", Category: actionCategory},
	{ID: ActionExitReview, Title: "Checkout Default Branch (Exit Review Mode)", Category: actionCategory},
	{ID: ActionCloseEditors, Title: "Close All Pull Request Editors", Category: actionCategory},
	{ID: ActionCopyLink, Title: "Copy Link of the Current Pull Request", Category: actionCategory},
	{ID: ActionOpenOnGitHub, Title: "Open the Current Pull Request on GitHub", Category: actionCategory},
}

func (p *PRs) Run(id string) bool {
	switch id {
	case ActionRefresh:
		p.refresh()
	case ActionCreate:
		p.openCreate()
	case ActionOpenCurrent:
		if !p.requireReady() {
			return true
		}
		p.openCurrent()
	case ActionCheckout:
		p.askCheckout()
	case ActionSearch:
		p.askSearch()
	case ActionSaveQuery:
		if p.requireReady() {
			p.askSaveQuery()
		}
	case ActionExitReview:
		if p.requireReady() {
			p.exitReview()
		}
	case ActionCloseEditors:
		p.closeReviewTabs()
	case ActionCopyLink:
		if p.requireCurrent() {
			p.env.CopyText(p.current.pr.URL)
			p.Host.Notify("Copied the pull request link")
		}
	case ActionOpenOnGitHub:
		if p.requireCurrent() {
			if err := p.env.OpenURL(p.current.pr.URL); err != nil {
				p.Host.ShowError("Cannot open the browser", err)
			}
		}
	default:
		return false
	}
	return true
}

func (p *PRs) requireReady() bool {
	if p.Phase != ghshared.PhaseReady {
		p.Host.Notify(p.StateMessage(p.repo.Host).Title)
		return false
	}
	return true
}

func (p *PRs) requireCurrent() bool {
	if !p.requireReady() {
		return false
	}
	if p.current.pr == nil {
		p.Host.Notify("The current branch has no pull request.")
		return false
	}
	return true
}

func (p *PRs) askCheckout() {
	if !p.requireReady() {
		return
	}
	p.Host.Ask("Checkout Pull Request", "Checkout", "", func(value string) error {
		number, err := parsePRNumber(value)
		if err != nil {
			return err
		}
		p.checkoutPR(number, &p.checkoutBusy)
		return nil
	})
}

func (p *PRs) askSearch() {
	if !p.requireReady() {
		return
	}
	p.Host.Ask("Search Pull Requests", "Search", p.side.field, func(value string) error {
		p.side.field = value
		p.runSearch(value)
		p.Host.Notify(fmt.Sprintf("Search results are in the %s sidebar.", sideViewName))
		return nil
	})
}
