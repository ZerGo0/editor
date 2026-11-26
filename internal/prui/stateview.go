package prui

import (
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	ghInstallURL    = "https://cli.github.com"
	ghLoginCommand  = "gh auth login"
	gitRemoteSample = "git remote add origin https://github.com/OWNER/REPO.git"
	copyCommand     = "Copy Command"
	installLink     = "Get the GitHub CLI"
)

type stateMessage struct {
	title   string
	detail  string
	command string
	link    string
	spin    bool
	retry   string
}

func (p *PRs) stateMessage() stateMessage {
	switch p.phase {
	case phaseNoRepo:
		return stateMessage{title: "No Git repository", detail: "Open a folder that is a Git repository to see its pull requests.", retry: "Check Again"}
	case phaseNoRemote:
		return stateMessage{title: "No GitHub remote", detail: "Add a GitHub remote to this repository:", command: gitRemoteSample, retry: "Check Again"}
	case phaseNoGH:
		return stateMessage{title: "GitHub CLI not found", detail: "Install the GitHub CLI and run " + ghLoginCommand, link: ghInstallURL, retry: "Check Again"}
	case phaseNotLoggedIn:
		host := p.repo.Host
		command := ghLoginCommand
		if host != "" && host != dotComHost {
			command += " --hostname " + host
		}
		return stateMessage{title: "Sign in to GitHub", detail: "Run this command in a terminal, then check again:", command: command, retry: "Check Again"}
	case phaseError:
		detail := "Something went wrong."
		if p.phaseErr != nil {
			detail = errorText(p.phaseErr)
		}
		return stateMessage{title: "Cannot load pull requests", detail: detail, retry: "Retry"}
	}
	return stateMessage{title: "Connecting to GitHub...", spin: true}
}

func (p *PRs) Ready() bool {
	return !p.closed && p.phase == phaseReady
}

func (p *PRs) StateView(c *ui.Context) bool {
	if p.Ready() {
		return false
	}
	if !p.closed {
		p.stateView(c)
	}
	return true
}

func (p *PRs) stateView(c *ui.Context) {
	m := p.stateMessage()
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		if m.spin {
			feedback.Loading(c, m.title)
			return
		}
		o := feedback.EmptyOptions{Icon: icons.Github, Title: m.title, Message: m.detail, Code: m.command, Action: m.retry, OnAction: p.connect}
		switch {
		case m.link != "":
			o.Link, o.OnLink = installLink, func() { openExternal(c, m.link) }
		case m.command != "":
			o.Link, o.OnLink = copyCommand, func() { c.WriteClipboard(m.command) }
		}
		feedback.Empty(c, o)
	})
}
