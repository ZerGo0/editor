package issueui

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

func stateTone(issue github.Issue) token.Tone {
	if isOpen(issue) {
		return token.ToneSuccess
	}
	return token.ToneInfo
}

func stateTag(c *ui.Context, issue github.Issue) ui.Element {
	return badge.Tag(c, stateLabel(issue), badge.TagOptions{Tone: stateTone(issue)})
}

func mutedTag(c *ui.Context, text string) ui.Element {
	return badge.Tag(c, text, badge.TagOptions{})
}

func labelTag(c *ui.Context, label github.Label) ui.Element {
	return badge.Tag(c, label.Name, badge.TagOptions{Hex: label.Color})
}

func primaryButton(c *ui.Context, label string, disabled bool) ui.Element {
	return action.Button(c, label, action.ButtonOptions{Variant: action.Primary, Small: true, Disabled: disabled})
}
