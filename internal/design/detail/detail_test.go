package detail

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 800
	testHeight = 600
)

func TestPageLayout(t *testing.T) {
	merged := false
	tt := ui.NewTester(func(c *ui.Context) {
		Page(c, func() {
			Header(c, HeaderOptions{Title: "Fix layout", Number: 12, Toolbar: func(c *ui.Context) {
				if action.Button(c, "Merge", action.ButtonOptions{}).Clicked() {
					merged = true
				}
			}})
			Body(c, "Pull request", token.PageMaxWidth, func() {
				Card(c, "Checks", CardOptions{Count: 2}, func() { ui.Text(c, "2 of 2 checks passed") })
				Property(c, "Labels", func(c *ui.Context) { ui.Text(c, "bug") }, nil)
				Comment(c, CommentOptions{Author: "octo", Verb: "commented", Time: "1h ago"}, func() { ui.Text(c, "Looks good") })
				Activity(c, "octo", "closed this pull request", "2h ago")
			})
		})
	}, testWidth, testHeight)
	for _, name := range []string{"Checks", "Labels", "octo commented"} {
		if _, ok := tt.Access(name); !ok {
			t.Errorf("no element named %q", name)
		}
	}
	if err := tt.Click("Merge"); err != nil || !merged {
		t.Fatalf("Merge click: %v, merged %v", err, merged)
	}
}
