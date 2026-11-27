package ghtest

import (
	"context"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	SideWidth    = 360
	SideHeight   = 640
	DetailWidth  = 1000
	DetailHeight = 1400
	StatusHeight = 40
)

func MustHaveText(t *testing.T, tt *ui.Tester, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !tt.HasText(s) {
			t.Errorf("missing text %q in %q", s, tt.Texts())
		}
	}
}

func MustClick(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("click %q: %v (texts %q)", label, err, tt.Texts())
	}
}

func NaturalWidth(text string, width, height int) float32 {
	var natural float32
	probe := ui.NewTester(func(c *ui.Context) {
		natural, _ = c.MeasureText(0, ui.Span{Text: text, Size: token.TextBody.Px(), Weight: token.TextBody.Weight()})
	}, width, height)
	probe.Frame()
	return natural
}

func SideTester(actions, view, overlay func(*ui.Context)) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).Children(func() { actions(c) })
			view(c)
		})
		if overlay != nil {
			overlay(c)
		}
	}, SideWidth, SideHeight)
}

func StatusTester(items func(*ui.Context)) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) { ui.Row(c).Children(func() { items(c) }) }, SideWidth, StatusHeight)
}

func (h *FakeHost) ViewTester(id string, overlay func(*ui.Context)) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { h.Views[id](c) })
		if overlay != nil {
			overlay(c)
		}
	}, DetailWidth, DetailHeight)
}

func OpenLocal[L any](failure *error, local func() L) func(context.Context, string) (L, error) {
	return func(context.Context, string) (L, error) {
		if *failure != nil {
			var none L
			return none, *failure
		}
		return local(), nil
	}
}
