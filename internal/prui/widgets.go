package prui

import (
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

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
