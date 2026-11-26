package badge

import (
	"math"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 600
	testHeight = 200
)

func TestHexColor(t *testing.T) {
	if _, ok := hexColor("zzzzzz"); ok {
		t.Error("invalid hex accepted")
	}
	if _, ok := hexColor(""); ok {
		t.Error("empty hex accepted")
	}
	if _, ok := hexColor("fff"); ok {
		t.Error("short hex accepted")
	}
	if c, ok := hexColor("d73a4a"); !ok || c != ui.RGB(0xd7, 0x3a, 0x4a) {
		t.Errorf("hexColor = %v %v", c, ok)
	}
}

func TestLongTagEndsWithEllipsis(t *testing.T) {
	long := strings.Repeat("feature/very-long-branch-name-", 4)
	tt := ui.NewTester(func(c *ui.Context) { Tag(c, long, TagOptions{}) }, testWidth, testHeight)
	var tag ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Name == long {
			tag = a
		}
	}
	if tag.Name == "" || tag.Bounds.W > token.TagMaxWidth.Px() {
		t.Fatalf("tag bounds %v; want a width of at most %v px", tag.Bounds, token.TagMaxWidth.Px())
	}
	if tag.Bounds.W != float32(math.Round(float64(tag.Bounds.W))) {
		t.Fatalf("tag width %v is not a whole px", tag.Bounds.W)
	}
	if tag.Description != long {
		t.Fatalf("cut tag tooltip %q; want the full text", tag.Description)
	}
}

func TestTagShowsTextAndCount(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			Tag(c, "bug", TagOptions{Hex: "d73a4a"})
			Count(c, 1200, CountOptions{})
		})
	}, 200, 60)
	if !tt.HasText("bug") || !tt.HasText("999+") {
		t.Errorf("texts %q", tt.Texts())
	}
}
