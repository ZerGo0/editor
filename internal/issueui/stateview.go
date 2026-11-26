package issueui

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

func (i *Issues) stateMessage() stateMessage {
	switch i.phase {
	case phaseNoRepo:
		return stateMessage{title: "No Git repository", detail: "Open a folder that is a Git repository to see its issues.", retry: "Check Again"}
	case phaseNoRemote:
		return stateMessage{title: "No GitHub remote", detail: "Add a GitHub remote to this repository:", command: gitRemoteSample, retry: "Check Again"}
	case phaseNoGH:
		return stateMessage{title: "GitHub CLI not found", detail: "Install the GitHub CLI and run " + ghLoginCommand, link: ghInstallURL, retry: "Check Again"}
	case phaseNotLoggedIn:
		command := ghLoginCommand
		if host := i.repo.Host; host != "" && host != dotComHost {
			command += " --hostname " + host
		}
		return stateMessage{title: "Sign in to GitHub", detail: "Run this command in a terminal, then check again:", command: command, retry: "Check Again"}
	case phaseError:
		detail := "Something went wrong."
		if i.phaseErr != nil {
			detail = errorText(i.phaseErr)
		}
		return stateMessage{title: "Cannot connect to GitHub", detail: detail, retry: "Retry"}
	}
	return stateMessage{title: "Connecting to GitHub...", spin: true}
}

func (i *Issues) stateView(c *ui.Context) {
	m := i.stateMessage()
	ui.Column(c).FillWidth().Children(func() {
		if m.spin {
			feedback.Loading(c, m.title)
			return
		}
		o := feedback.EmptyOptions{Icon: icons.Github, Title: m.title, Message: m.detail, Code: m.command, Action: m.retry, OnAction: i.connect}
		switch {
		case m.link != "":
			o.Link, o.OnLink = installLink, func() { openExternal(c, m.link) }
		case m.command != "":
			o.Link, o.OnLink = copyCommand, func() { c.WriteClipboard(m.command) }
		}
		feedback.Empty(c, o)
	})
}
