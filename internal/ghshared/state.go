package ghshared

import (
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	ghInstallURL    = "https://cli.github.com"
	ghLoginCommand  = "gh auth login"
	gitRemoteSample = "git remote add origin https://github.com/OWNER/REPO.git"
	copyCommand     = "Copy Command"
	installLink     = "Get the GitHub CLI"
	checkAgainLabel = "Check Again"
)

type StateMessage struct {
	Title   string
	Detail  string
	Command string
	Link    string
	Spin    bool
	Retry   string
}

type StateWording struct {
	NoRepoDetail string
	ErrorTitle   string
}

func (s *Session[C]) StateMessage(host string) StateMessage {
	w := s.Wording
	switch s.Phase {
	case PhaseNoRepo:
		return StateMessage{Title: "No Git repository", Detail: w.NoRepoDetail, Retry: checkAgainLabel}
	case PhaseNoRemote:
		return StateMessage{Title: "No GitHub remote", Detail: "Add a GitHub remote to this repository:", Command: gitRemoteSample, Retry: checkAgainLabel}
	case PhaseNoGH:
		return StateMessage{Title: "GitHub CLI not found", Detail: "Install the GitHub CLI and run " + ghLoginCommand, Link: ghInstallURL, Retry: checkAgainLabel}
	case PhaseNotLoggedIn:
		command := ghLoginCommand
		if host != "" && host != github.DotComHost {
			command += " --hostname " + host
		}
		return StateMessage{Title: "Sign in to GitHub", Detail: "Run this command in a terminal, then check again:", Command: command, Retry: checkAgainLabel}
	case PhaseError:
		detail := "Something went wrong."
		if s.PhaseErr != nil {
			detail = ErrorText(s.PhaseErr)
		}
		return StateMessage{Title: w.ErrorTitle, Detail: detail, Retry: RetryLabel}
	}
	return StateMessage{Title: "Connecting to GitHub...", Spin: true}
}

func TrackState[C, T any](s *Session[C], key string, saved *T, current func() T) {
	state := s.Host.ProjectState()
	state.Load(key, saved)
	state.Track(key, func() any { return current() })
}

func StateView(c *ui.Context, m StateMessage, connect func()) ui.Element {
	return ui.Column(c).FillWidth().Children(func() {
		if m.Spin {
			feedback.LoadingPane(c, m.Title)
			return
		}
		o := feedback.EmptyOptions{Icon: icons.Github, Title: m.Title, Message: m.Detail, Code: m.Command, Action: m.Retry, OnAction: connect}
		switch {
		case m.Link != "":
			o.Link, o.OnLink = installLink, func() { c.OpenURL(m.Link) }
		case m.Command != "":
			o.Link, o.OnLink = copyCommand, func() { c.WriteClipboard(m.Command) }
		}
		feedback.Empty(c, o)
	})
}
