package feedback

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func TestToastTitleCentresOnCloseButton(t *testing.T) {
	const title = "Pulled main"
	tt := ui.NewTester(func(c *ui.Context) {
		toast(c, token.Of(c), ui.Toast{ID: "pull", Title: title})
	}, 420, 120)
	text, ok := tt.Find(title)
	if !ok {
		t.Fatalf("texts %q", tt.Texts())
	}
	closeButton, ok := tt.Access(closeToastLabel)
	if !ok {
		t.Fatal("no close button")
	}
	textMid := text.Y + text.H/2
	closeMid := closeButton.Bounds.Y + closeButton.Bounds.H/2
	if d := textMid - closeMid; d > 1 || d < -1 {
		t.Fatalf("title centre y %.2f, close button centre y %.2f", textMid, closeMid)
	}
}
