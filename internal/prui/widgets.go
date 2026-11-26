package prui

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const retryLabel = "Retry"

func stateTone(pr github.PullRequest) token.Tone {
	switch prStateLabel(pr) {
	case prStateMerged:
		return token.ToneInfo
	case prStateClosed:
		return token.ToneError
	case prStateDraft:
		return token.ToneNeutral
	}
	return token.ToneSuccess
}

func stateTag(c *ui.Context, pr github.PullRequest) ui.Element {
	return badge.Tag(c, prStateLabel(pr), badge.TagOptions{Tone: stateTone(pr)})
}

func mutedTag(c *ui.Context, text string) ui.Element {
	return badge.Tag(c, text, badge.TagOptions{})
}

func branchChip(c *ui.Context, name string) ui.Element {
	return badge.Tag(c, name, badge.TagOptions{Icon: icons.GitBranch, Tooltip: "Branch"})
}

func checkAppearance(state checkState) (*ui.SVG, token.Tone, bool) {
	switch state {
	case checkSuccess:
		return icons.Check, token.ToneSuccess, true
	case checkFailure:
		return icons.Error, token.ToneError, true
	case checkPending:
		return icons.Dot, token.ToneWarning, true
	}
	return nil, token.ToneNeutral, false
}

func checkIcon(c *ui.Context, state checkState) {
	svg, tone, ok := checkAppearance(state)
	if !ok {
		return
	}
	ui.Icon(c, svg).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(token.Of(c).Tone(tone)).Shrink(0).Label(checkStateLabel(state))
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

func options(values []string) []action.Option[string] {
	out := make([]action.Option[string], len(values))
	for i, v := range values {
		out[i] = action.Option[string]{Value: v, Label: v}
	}
	return out
}
