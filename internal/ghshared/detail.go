package ghshared

import (
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/egoist/mygo/ui"
)

const commentLines = 6

func DetailLoading(c *ui.Context, number int) {
	detail.Loading(c, "Loading #"+strconv.Itoa(number)+"...")
}

func DetailLoadFailed(c *ui.Context, number int, err error, retry func()) {
	detail.Centered(c, func() {
		feedback.Error(c, "Cannot load #"+strconv.Itoa(number), err, retry)
	})
}

func CommentBox(c *ui.Context, draft *string, busy string, post func()) {
	detail.Card(c, "Add a comment", detail.CardOptions{}, func() {
		field.TextArea(c, "Comment", draft, field.AreaOptions{Placeholder: "Leave a comment (Markdown is supported)", MinLines: commentLines, ID: "comment"})
		blank := strings.TrimSpace(*draft) == ""
		ui.Row(c).Justify(ui.End).FillWidth().Children(func() {
			action.Button(c, "Add Comment", action.ButtonOptions{Variant: action.Primary, Disabled: blank || busy != ""}).OnClick(post)
		})
	})
}
