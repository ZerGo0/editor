package diffmode

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
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
	if action.ToggleIconButton(c, icons.Whitespace, whitespaceLabel, &ignore, action.IconOptions{ID: "diff-whitespace"}).Changed() {
		Session.Whitespace = git.WhitespaceNone
		if ignore {
			Session.Whitespace = ignoreMode
		}
	}
	action.ToggleIconButton(c, icons.MovedCode, movesToggle, &Session.ShowMoves, action.IconOptions{ID: "diff-show-moves"})
	if e != nil && e.Busy() {
		feedback.Spinner(c, busyLabel)
	}
	if e != nil && e.Notice() != "" {
		token.TextLabel.Apply(ui.Text(c, e.Notice())).TextColor(token.Of(c).Tone(token.ToneWarning)).SingleLine().Shrink(1).MinWidth(0)
	}
}
