package prui

import (
	"github.com/egoist/mygo/ui"
)

const (
	ghInstallURL    = "https://cli.github.com"
	ghLoginCommand  = "gh auth login"
	gitRemoteSample = "git remote add origin https://github.com/OWNER/REPO.git"
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
		return stateMessage{title: "Sign in to GitHub with " + ghLoginCommand, detail: "Run this command in a terminal, then check again:", command: command, retry: "Check Again"}
	case phaseError:
		detail := "Something went wrong."
		if p.phaseErr != nil {
			detail = errorText(p.phaseErr)
		}
		return stateMessage{title: "Cannot load pull requests", detail: detail, retry: "Retry"}
	}
	return stateMessage{title: "Connecting to GitHub...", spin: true}
}

func (p *PRs) stateView(c *ui.Context) {
	m := p.stateMessage()
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Gap(spacingMedium).Padding(messagePad).Children(func() {
		if m.spin {
			ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Label(m.title)
				ui.Text(c, m.title).FontSize(fontBody)
			})
			return
		}
		ui.Text(c, m.title).Bold().FontSize(fontBody)
		ui.Text(c, m.detail).FontSize(fontSmall).TextColor(t.TextMuted)
		if m.command != "" {
			ui.Text(c, m.command).FontSize(fontSmall).Font("monospace").Selectable().Background(t.Surface).Radius(cardRadius).Padding(spacingSmall, spacingMedium)
			ui.Button(c, "Copy Command").OnClick(func() { c.WriteClipboard(m.command) })
		}
		if m.link != "" {
			ui.Link(c, "Get the GitHub CLI", m.link)
		}
		if m.retry != "" {
			ui.Button(c, m.retry).OnClick(p.connect)
		}
	})
}
