package viewer

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type previewFixture struct {
	pv   *Preview
	h    *fakeHost
	tt   *ui.Tester
	text string
}

func newPreviewFixture(t *testing.T, path, text string) *previewFixture {
	t.Helper()
	fx := &previewFixture{h: newFakeHost(), text: text}
	fx.pv = NewPreview(fx.h, path, func() string { return fx.text })
	if fx.pv == nil {
		t.Fatalf("NewPreview(%q) = nil", path)
	}
	t.Cleanup(fx.pv.Close)
	fx.tt = ui.NewTester(func(c *ui.Context) { fx.pv.View(c) }, testWidth, testHeight)
	return fx
}

func TestNewPreviewOnlyForPreviewableFiles(t *testing.T) {
	h := newFakeHost()
	for _, path := range []string{"main.go", "a.png", "a.bin", "a.pdf"} {
		if NewPreview(h, path, func() string { return "" }) != nil {
			t.Errorf("NewPreview(%q) returned a preview", path)
		}
	}
	if NewPreview(h, "a.md", nil) != nil {
		t.Error("NewPreview without a text source returned a preview")
	}
}

func TestMarkdownPreviewFollowsTheBufferAfterADebounce(t *testing.T) {
	fx := newPreviewFixture(t, "/work/readme.md", "# Hello")
	if !fx.tt.HasText("Hello") {
		t.Fatalf("texts: %q", fx.tt.Texts())
	}
	fx.text = "# Changed"
	fx.tt.Frame()
	fx.tt.Advance(100 * time.Millisecond)
	if !fx.tt.HasText("Hello") {
		t.Error("the preview changed before the debounce ended")
	}
	fx.tt.Advance(2 * previewDebounce)
	if !fx.tt.HasText("Changed") || fx.tt.HasText("Hello") {
		t.Errorf("texts: %q", fx.tt.Texts())
	}
	if fx.tt.FramePending() {
		t.Error("the preview keeps asking for frames while idle")
	}
}

func TestMarkdownPreviewWaitsWhileTheTextKeepsChanging(t *testing.T) {
	fx := newPreviewFixture(t, "/work/readme.md", "# Start")
	for i := range 8 {
		fx.text = "# Typing " + string(rune('a'+i))
		fx.tt.Frame()
		fx.tt.Advance(100 * time.Millisecond)
	}
	if !fx.tt.HasText("Start") {
		t.Errorf("the preview updated while typing: %q", fx.tt.Texts())
	}
	fx.tt.Advance(4 * previewDebounce)
	if !fx.tt.HasText("Typing h") {
		t.Errorf("the final text did not show: %q", fx.tt.Texts())
	}
}

func TestMarkdownPreviewEmptyState(t *testing.T) {
	fx := newPreviewFixture(t, "/work/readme.md", "  \n")
	if !fx.tt.HasText("Nothing to preview") {
		t.Errorf("texts: %q", fx.tt.Texts())
	}
}

func TestSVGPreviewRendersAndReportsErrors(t *testing.T) {
	fx := newPreviewFixture(t, "/work/a.svg", svgSample)
	p := fx.pv.pane.(*imagePane)
	settle(t, fx.h, fx.tt, func() bool { return p.pic != nil })
	if !fx.tt.HasText("40 x 20 px") {
		t.Fatalf("texts: %q", fx.tt.Texts())
	}
	if _, ok := fx.tt.Find(svgPreviewLabel); !ok {
		t.Error("the preview has no accessible label")
	}
	fx.text = "<svg"
	fx.tt.Frame()
	fx.tt.Advance(3 * previewDebounce)
	settle(t, fx.h, fx.tt, func() bool { return p.err != nil })
	if !fx.tt.HasText("Cannot show this image") {
		t.Errorf("texts: %q", fx.tt.Texts())
	}
	fx.text = svgSample
	fx.tt.Frame()
	fx.tt.Advance(3 * previewDebounce)
	settle(t, fx.h, fx.tt, func() bool { return p.pic != nil && p.err == nil })
}

func TestSVGPreviewEmptyText(t *testing.T) {
	fx := newPreviewFixture(t, "/work/a.svg", "")
	p := fx.pv.pane.(*imagePane)
	settle(t, fx.h, fx.tt, func() bool { return p.empty })
	if !fx.tt.HasText("Nothing to preview") {
		t.Errorf("texts: %q", fx.tt.Texts())
	}
}

func TestPreviewHandleAndClose(t *testing.T) {
	fx := newPreviewFixture(t, "/work/readme.md", "# Hi")
	if fx.pv.Handle() == nil {
		t.Fatal("no handle")
	}
	fx.pv.Close()
	fx.pv.Close()
}
