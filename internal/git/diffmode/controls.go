package diffmode

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	whitespaceLabel = "Ignore Whitespace"
	movesToggle     = "Show Moved Code"
	busyLabel       = "Comparing"
)

func Controls(c *ui.Context, e *Engine) {
	options := make([]action.Option[git.WhitespaceMode], len(WhitespaceChoices))
	for i, choice := range WhitespaceChoices {
		options[i] = action.Option[git.WhitespaceMode]{Value: choice.Mode, Label: choice.Label}
	}
	action.Select(c, whitespaceLabel, &Session.Whitespace, options, action.SelectOptions{ID: "diff-whitespace"})
	action.Checkbox(c, movesToggle, &Session.ShowMoves, action.ChoiceOptions{ID: "diff-show-moves"}).MinHeight(token.HitTargetMin.Px())
	if e != nil && e.Busy() {
		feedback.Spinner(c, busyLabel)
	}
	if e != nil && e.Notice() != "" {
		token.TextLabel.Apply(ui.Text(c, e.Notice())).TextColor(token.Of(c).Tone(token.ToneWarning)).SingleLine().Shrink(1).MinWidth(0)
	}
}
