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
	ignoreMode      = git.WhitespaceIgnore
)

func Controls(c *ui.Context, e *Engine) {
	ignore := Session.Whitespace != git.WhitespaceNone
	if action.Checkbox(c, whitespaceLabel, &ignore, action.ChoiceOptions{ID: "diff-whitespace"}).Changed() {
		Session.Whitespace = git.WhitespaceNone
		if ignore {
			Session.Whitespace = ignoreMode
		}
	}
	action.Checkbox(c, movesToggle, &Session.ShowMoves, action.ChoiceOptions{ID: "diff-show-moves"})
	if e != nil && e.Busy() {
		feedback.Spinner(c, busyLabel)
	}
	if e != nil && e.Notice() != "" {
		token.TextLabel.Apply(ui.Text(c, e.Notice())).TextColor(token.Of(c).Tone(token.ToneWarning)).SingleLine().Shrink(1).MinWidth(0)
	}
}
