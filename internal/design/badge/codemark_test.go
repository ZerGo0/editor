package badge

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func TestCodeMarkShowsItsTextAndDescription(t *testing.T) {
	font := ui.Font{Family: token.FontMono, Size: token.TextMono.Px()}
	const tip = "The character U+202E is a control character"
	tt := ui.NewTester(func(c *ui.Context) {
		w := CodeMarkWidth(c, "[U+202E]", font)
		if w <= CodeMarkWidth(c, "c", font) {
			t.Errorf("width %v is not wider than one character", w)
		}
		e := ui.Row(c).Size(w, token.TextMono.LineHeight()).Role(ui.RoleImage).Label("mark")
		CodeMark(c, e, "[U+202E]", CodeMarkOptions{Font: font, Tone: token.ToneError, Tooltip: tip})
	}, testWidth, testHeight)
	if !tt.HasText("[U+202E]") {
		t.Fatal("the mark does not show its text")
	}
	if a, ok := tt.AccessByRole(ui.RoleImage, "mark"); !ok || a.Description != tip {
		t.Fatalf("mark access %+v, want description %q", a, tip)
	}
}
