package issueui

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const retryLabel = "Retry"

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

func mutedText(c *ui.Context, text string) ui.Element {
	return design.Text(c, text, token.TextCaption)
}

func errorNotice(c *ui.Context, err error, retry func()) {
	feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneError, Text: errorText(err), Action: retryLabel, OnAction: retry})
}

func smallButton(c *ui.Context, label string, disabled bool) ui.Element {
	return action.Button(c, label, action.ButtonOptions{Small: true, Disabled: disabled})
}

func ghostButton(c *ui.Context, label, id string) ui.Element {
	return action.Button(c, label, action.ButtonOptions{Variant: action.Ghost, Small: true, ID: id})
}

func primaryButton(c *ui.Context, label string, disabled bool) ui.Element {
	return action.Button(c, label, action.ButtonOptions{Variant: action.Primary, Small: true, Disabled: disabled})
}

func options(values []string) []action.Option[string] {
	out := make([]action.Option[string], len(values))
	for i, v := range values {
		out[i] = action.Option[string]{Value: v, Label: v}
	}
	return out
}
