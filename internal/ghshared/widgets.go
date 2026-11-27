package ghshared

import (
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const RetryLabel = "Retry"

func MutedText(c *ui.Context, text string) ui.Element {
	return design.Text(c, text, token.TextCaption)
}

func ErrorNotice(c *ui.Context, err error, retry func()) {
	feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneError, Text: ErrorText(err), Action: RetryLabel, OnAction: retry})
}

func SmallButton(c *ui.Context, label string, disabled bool) ui.Element {
	return action.Button(c, label, action.ButtonOptions{Small: true, Disabled: disabled})
}

func GhostButton(c *ui.Context, label, id string) ui.Element {
	return action.Button(c, label, action.ButtonOptions{Variant: action.Ghost, Small: true, ID: id})
}

func Options(values []string) []action.Option[string] {
	out := make([]action.Option[string], len(values))
	for i, v := range values {
		out[i] = action.Option[string]{Value: v, Label: v}
	}
	return out
}

func ErrorItem(err error, retry func()) list.Item {
	return list.Item{Depth: 1, Tone: token.ToneError, Label: ErrorText(err), Tooltip: ErrorText(err), Trailing: func(c *ui.Context) {
		GhostButton(c, RetryLabel, "retry").OnClick(retry)
	}}
}

func ToggleSectionWithArrows(c *ui.Context, open *bool) {
	switch {
	case !*open && c.Shortcut(0, ui.KeyRight):
		*open = true
	case *open && c.Shortcut(0, ui.KeyLeft):
		*open = false
	}
}

func CreateFooter(c *ui.Context, busy, blocked string) ui.Element {
	if busy != "" {
		feedback.Spinner(c, busy)
		MutedText(c, busy+"...")
	}
	if blocked != "" {
		MutedText(c, blocked)
	}
	return action.Button(c, "Create", action.ButtonOptions{Variant: action.Primary, Disabled: blocked != "" || busy != ""})
}
