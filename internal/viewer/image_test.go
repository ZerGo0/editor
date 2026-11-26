package viewer

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

const svgSample = `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><rect width="40" height="20" fill="red"/></svg>`

func openImage(t *testing.T, path string, kind Kind) (*State, *imagePane, *fakeHost, *ui.Tester) {
	t.Helper()
	h := newFakeHost()
	s, err := Open(h, path, kind, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	p := s.pane.(*imagePane)
	tt := viewTester(s)
	return s, p, h, tt
}

func TestImageLoadsAndShowsInfo(t *testing.T) {
	path := writePNG(t, t.TempDir(), "a.png", 4, 3)
	_, p, h, tt := openImage(t, path, Image)
	if !tt.HasText("Loading image") {
		t.Errorf("no loading state before the decode finished: %q", tt.Texts())
	}
	settle(t, h, tt, func() bool { return p.pic != nil })
	for _, want := range []string{"4 x 3 px", "PNG", "100%"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	if _, ok := tt.Find("Image viewer"); !ok {
		t.Error("the canvas has no accessible label")
	}
	if tt.FramePending() {
		t.Error("an idle image view keeps asking for frames")
	}
}

func TestImageZoomControls(t *testing.T) {
	path := writePNG(t, t.TempDir(), "a.png", 8, 6)
	s, p, h, tt := openImage(t, path, Image)
	settle(t, h, tt, func() bool { return p.pic != nil })
	if err := tt.Click("Zoom In"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("150%") {
		t.Fatalf("zoom in: %q", tt.Texts())
	}
	if err := tt.Click("Zoom Out"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("100%") {
		t.Fatalf("zoom out: %q", tt.Texts())
	}
	s.Handle().Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Key(ui.Cmd, ui.KeyEqual)
	if !tt.HasText("200%") {
		t.Fatalf("Cmd+=: %q", tt.Texts())
	}
	tt.Key(ui.Cmd, ui.KeyMinus)
	if !tt.HasText("150%") {
		t.Fatalf("Cmd+-: %q", tt.Texts())
	}
	tt.Key(ui.Cmd, ui.Key0)
	if !tt.HasText("100%") || p.zoom.fit {
		t.Fatalf("Cmd+0: %q fit=%v", tt.Texts(), p.zoom.fit)
	}
	if err := tt.Click("Fit"); err != nil {
		t.Fatal(err)
	}
	if !p.zoom.fit {
		t.Error("Fit did not return to fit mode")
	}
}

func TestImageFitShrinksLargeImages(t *testing.T) {
	path := writePNG(t, t.TempDir(), "big.png", 1800, 1200)
	_, p, h, tt := openImage(t, path, Image)
	settle(t, h, tt, func() bool { return p.pic != nil && p.viewW > 0 })
	if got := p.effective(); got >= 1 {
		t.Fatalf("fit scale of a large image = %v, want below 1", got)
	}
	if err := tt.Click("Actual Size"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("100%") || !p.pannable() {
		t.Fatalf("actual size: %q pannable=%v", tt.Texts(), p.pannable())
	}
}

func TestImagePanAndPointerZoom(t *testing.T) {
	path := writePNG(t, t.TempDir(), "big.png", 1800, 1200)
	_, p, h, tt := openImage(t, path, Image)
	settle(t, h, tt, func() bool { return p.pic != nil && p.viewW > 0 })
	if err := tt.Click("Actual Size"); err != nil {
		t.Fatal(err)
	}
	box, ok := tt.Find("Image viewer")
	if !ok {
		t.Fatal("no canvas")
	}
	cx, cy := box.X+box.W/2, box.Y+box.H/2
	tt.Scroll(cx, cy, 0, 30)
	if p.zoom.panY >= 0 {
		t.Errorf("scrolling down moved the image to pan %v, want negative", p.zoom.panY)
	}
	before := p.zoom.panX
	tt.Press(cx, cy)
	tt.Move(cx+25, cy)
	tt.Release(cx+25, cy)
	if p.zoom.panX-before != 25 {
		t.Errorf("drag moved the image by %v, want 25", p.zoom.panX-before)
	}
	tt.Pinch(cx, cy, 2)
	if p.zoom.scale != 2 {
		t.Errorf("pinch scale = %v, want 2", p.zoom.scale)
	}
	p.input(ui.InputEvent{Kind: ui.InputScroll, DY: -100, X: cx, Y: cy, Mods: ui.Cmd})
	if p.zoom.scale <= 2 {
		t.Errorf("Cmd+scroll up did not zoom in: %v", p.zoom.scale)
	}
}

func TestZoomedPictureStaysCentered(t *testing.T) {
	path := writePNG(t, t.TempDir(), "a.png", 600, 400)
	_, p, h, tt := openImage(t, path, Image)
	settle(t, h, tt, func() bool { return p.pic != nil && p.viewW > 0 })
	for _, scale := range []float32{0.5, 1, 4} {
		p.zoomTo(scale)
		tt.Frame()
		tt.Frame()
		box, ok := tt.Find("Image viewer")
		pic, found := tt.Find("a.png")
		if !ok || !found {
			t.Fatalf("scale %v: canvas %v picture %v", scale, ok, found)
		}
		if dx, dy := pic.X+pic.W/2-(box.X+box.W/2), pic.Y+pic.H/2-(box.Y+box.H/2); dx < -1 || dx > 1 || dy < -1 || dy > 1 {
			t.Errorf("scale %v: picture center is off by %v, %v", scale, dx, dy)
		}
		if want := min(600*scale, box.W); pic.W < want-1 || pic.W > want+1 {
			t.Errorf("scale %v: picture shows %v wide, want %v", scale, pic.W, want)
		}
	}
}

func TestImageTooLargeOffersPreview(t *testing.T) {
	path := writePNG(t, t.TempDir(), "a.png", 10, 10)
	s, p, h, tt := openImage(t, path, Image)
	calls := recordCommands(s)
	settle(t, h, tt, func() bool { return p.pic != nil })
	p.limit = 50
	s.Reload()
	settle(t, h, tt, func() bool { return p.big != nil })
	if !tt.HasText("This image is too large to show") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	if err := tt.Click("Open in Preview"); err != nil {
		t.Fatal(err)
	}
	got := nextCommand(t, calls)
	want := []string{"open", "-a", "Preview", s.path}
	if len(got) != len(want) || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Errorf("command = %v, want %v", got, want)
	}
}

func TestImageFallsBackToConverter(t *testing.T) {
	dir := t.TempDir()
	pngPath := writePNG(t, dir, "src.png", 5, 2)
	pngData, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	heic := writeFile(t, dir, "photo.heic", []byte("not a format Go knows"))
	h := newFakeHost()
	s, err := Open(h, heic, Image, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	p := s.pane.(*imagePane)
	p.convert = func(path string, limit int64) ([]byte, error) { return pngData, nil }
	tt := viewTester(s)
	s.Reload()
	settle(t, h, tt, func() bool { return p.pic != nil })
	if !tt.HasText("5 x 2 px") || !tt.HasText("HEIC") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

func TestImageErrorOffersRetry(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "broken.png", []byte("\x89PNG\r\n\x1a\nnope"))
	s, p, h, tt := openImage(t, path, Image)
	p.convert = nil
	s.Reload()
	settle(t, h, tt, func() bool { return p.err != nil })
	if !tt.HasText("Cannot show this image") {
		t.Fatalf("texts: %q", tt.Texts())
	}
	writePNG(t, dir, "broken.png", 2, 2)
	if err := tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	settle(t, h, tt, func() bool { return p.pic != nil && p.err == nil })
	if !tt.HasText("2 x 2 px") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

func TestImageReloadShowsTheNewFile(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, "a.png", 4, 4)
	s, p, h, tt := openImage(t, path, Image)
	settle(t, h, tt, func() bool { return p.pic != nil })
	writePNG(t, dir, "a.png", 7, 5)
	s.Reload()
	settle(t, h, tt, func() bool { return p.pic != nil && p.pic.width == 7 })
	if !tt.HasText("7 x 5 px") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

func TestImageIgnoresResultsAfterClose(t *testing.T) {
	path := writePNG(t, t.TempDir(), "a.png", 4, 4)
	s, p, h, _ := openImage(t, path, Image)
	s.Close()
	s.Close()
	h.drain()
	if p.pic != nil {
		t.Error("a result arrived after Close")
	}
}

func TestSVGRendersWithZoomToolbar(t *testing.T) {
	path := writeFile(t, t.TempDir(), "a.svg", []byte(svgSample))
	_, p, h, tt := openImage(t, path, SVG)
	settle(t, h, tt, func() bool { return p.pic != nil })
	if !tt.HasText("40 x 20 px") || !tt.HasText("SVG") {
		t.Errorf("texts: %q", tt.Texts())
	}
	if _, ok := tt.Find("Zoom In"); !ok {
		t.Error("no zoom toolbar")
	}
	if !p.zoom.enlargeToFit {
		t.Error("SVG does not enlarge to fit")
	}
}

func TestAnimatedGIFPlaysAndPauses(t *testing.T) {
	path := writeGIF(t, t.TempDir(), "a.gif", 3)
	_, p, h, tt := openImage(t, path, Image)
	settle(t, h, tt, func() bool { return p.pic != nil })
	if p.player == nil {
		t.Fatal("an animated GIF has no player")
	}
	if err := tt.Click("Pause"); err != nil {
		t.Fatal(err)
	}
	if p.player.Playing() {
		t.Error("Pause did not pause")
	}
	if err := tt.Click("Play"); err != nil {
		t.Fatal(err)
	}
	if !p.player.Playing() {
		t.Error("Play did not play")
	}
}

func TestSingleFrameGIFIsAPicture(t *testing.T) {
	path := writeGIF(t, t.TempDir(), "a.gif", 1)
	_, p, h, tt := openImage(t, path, Image)
	settle(t, h, tt, func() bool { return p.pic != nil })
	if p.player != nil {
		t.Error("a one frame GIF got a player")
	}
}

func TestDecodeRasterRejectsOversizedPixels(t *testing.T) {
	var buf bytes.Buffer
	img := writePNGImage(20, 20)
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	_, err := decodeRaster(buf.Bytes(), "x.png", 100, nil)
	var big *tooLargeError
	if !errors.As(err, &big) || big.width != 20 {
		t.Fatalf("error = %v, want tooLargeError", err)
	}
}

func TestParseSipsSize(t *testing.T) {
	out := "/tmp/a.heic\n  pixelWidth: 4032\n  pixelHeight: 3024\n"
	w, h, err := parseSipsSize(out)
	if err != nil || w != 4032 || h != 3024 {
		t.Fatalf("got %d, %d, %v", w, h, err)
	}
	if _, _, err := parseSipsSize("garbage"); err == nil {
		t.Fatal("no error for missing size")
	}
}
