package feedback

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func titleAndAction(t *testing.T, title, label string) (ui.Rect, ui.AccessInfo) {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		toast(c, token.Of(c), ui.Toast{ID: "discard", Title: title, Action: label}, false)
	}, 420, 160)
	text, ok := tt.Find(title)
	if !ok {
		t.Fatalf("texts %q", tt.Texts())
	}
	button, ok := tt.Access(label)
	if !ok {
		t.Fatal("no action button")
	}
	return text, button
}

func TestToastKeepsAShortActionOnTheTitleRow(t *testing.T) {
	text, button := titleAndAction(t, "Review discarded", "Undo")
	if button.Bounds.Y+button.Bounds.H/2 > text.Y+text.H {
		t.Fatalf("action at y %.2f is below the title at y %.2f", button.Bounds.Y, text.Y)
	}
}

func TestToastPutsALongActionBelowTheTitle(t *testing.T) {
	text, button := titleAndAction(t, "Review discarded for this session", "Open the review again")
	if button.Bounds.Y < text.Y+text.H {
		t.Fatalf("action at y %.2f shares the row of the title at y %.2f", button.Bounds.Y, text.Y)
	}
}

func TestToastTitleCentresOnCloseButton(t *testing.T) {
	const title = "Pulled main"
	tt := ui.NewTester(func(c *ui.Context) {
		toast(c, token.Of(c), ui.Toast{ID: "pull", Title: title}, false)
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

func TestToastShowsASecondaryActionThatClosesIt(t *testing.T) {
	const title, primary, secondary = "Cannot save a.txt", "Save As...", "Show in Finder"
	ran, added := false, false
	tt := ui.NewTester(func(c *ui.Context) {
		if !added {
			added = true
			Toast(c, ToastOptions{ID: "save", Title: title, Action: primary, Secondary: secondary, OnSecondary: func() { ran = true }, Sticky: true})
		}
		ToastHost(c, ToastHostOptions{})
	}, 420, 200)
	if _, ok := tt.Access(primary); !ok {
		t.Fatal("no primary action")
	}
	tt.Click(secondary)
	if !ran {
		t.Fatal("the secondary action did not run")
	}
	if _, ok := tt.Find(title); ok {
		t.Fatal("the toast stays after its secondary action")
	}
}
