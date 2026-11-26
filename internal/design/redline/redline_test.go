package redline

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	testScale = 2
	testW     = 200
	testH     = 100
)

var (
	testBg   = color.RGBA{R: 24, G: 24, B: 24, A: 255}
	testInk  = color.RGBA{R: 220, G: 220, B: 220, A: 255}
	testDim  = color.RGBA{R: 60, G: 60, B: 60, A: 255}
	testFill = color.RGBA{R: 90, G: 90, B: 90, A: 255}
)

func canvasOf() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, testW*testScale, testH*testScale))
	fill(img, Rect{W: testW, H: testH}, testBg)
	return img
}

func fill(img *image.RGBA, r Rect, c color.RGBA) {
	px := pxRect(r, testScale)
	for y := px.Min.Y; y < px.Max.Y; y++ {
		for x := px.Min.X; x < px.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func run(t *testing.T, img image.Image, boxes []Box) Result {
	t.Helper()
	res, err := Analyze(Input{Name: "test", Image: img, Boxes: boxes, Scale: testScale})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func findings(res Result, rule Rule) []Finding {
	var out []Finding
	for _, f := range res.Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestAnalyzeRejectsBadInput(t *testing.T) {
	if _, err := Analyze(Input{Scale: testScale}); err == nil {
		t.Error("no image accepted")
	}
	for _, s := range []float64{0, -1, 100} {
		if _, err := Analyze(Input{Image: canvasOf(), Scale: s}); err == nil {
			t.Errorf("scale %v accepted", s)
		}
	}
	res := run(t, canvasOf(), []Box{{Role: "button", Rect: Rect{X: 1, Y: 1, W: -5, H: 3}}, {Role: "button", Rect: Rect{X: 500, Y: 500, W: 5, H: 5}}})
	if len(res.Boxes) != 0 {
		t.Errorf("kept %d invalid or off-image boxes", len(res.Boxes))
	}
}

func TestScaleOf(t *testing.T) {
	s, err := ScaleOf(canvasOf(), testW)
	if err != nil || s != testScale {
		t.Errorf("ScaleOf = %v, %v", s, err)
	}
	if _, err := ScaleOf(canvasOf(), 0); err == nil {
		t.Error("zero width accepted")
	}
}

func TestOpticalCentre(t *testing.T) {
	img := canvasOf()
	fill(img, Rect{X: 5, Y: 5, W: 12, H: 12}, testInk)
	fill(img, Rect{X: 31, Y: 5, W: 12, H: 12}, testInk)
	res := run(t, img, []Box{
		{Role: "button", Label: "Centred", Rect: Rect{X: 0, Y: 0, W: 22, H: 22}},
		{Role: "button", Label: "Off", Rect: Rect{X: 25, Y: 0, W: 22, H: 22}},
	})
	got := findings(res, RuleOptical)
	if len(got) != 1 || res.Boxes[got[0].Box].Label != "Off" || got[0].Check != "ink centre x" || got[0].Measured != 1 {
		t.Fatalf("optical findings = %+v", got)
	}
}

func TestTextBand(t *testing.T) {
	img := canvasOf()
	for i, x := range []float64{10, 18, 26} {
		top := 7.0
		if i == 1 {
			top = 9
		}
		fill(img, Rect{X: x, Y: top + 1, W: 5, H: 16 - top}, testInk)
	}
	res := run(t, img, []Box{{Role: "row", Label: "HxH", Rect: Rect{X: 0, Y: 0, W: 100, H: 22}}})
	got := findings(res, RuleText)
	if len(got) != 1 || got[0].Measured != 1.5 {
		t.Fatalf("text findings = %+v, measures %+v", got, res.Measures)
	}
}

func TestUnevenGaps(t *testing.T) {
	res := run(t, canvasOf(), []Box{
		{ID: "bar", Role: "toolbar", Rect: Rect{X: 0, Y: 0, W: 120, H: 22}},
		{ID: "a", Parent: "bar", Role: "button", Label: "A", Rect: Rect{X: 0, Y: 0, W: 22, H: 22}},
		{ID: "b", Parent: "bar", Role: "button", Label: "B", Rect: Rect{X: 24, Y: 0, W: 22, H: 22}},
		{ID: "c", Parent: "bar", Role: "button", Label: "C", Rect: Rect{X: 48, Y: 0, W: 22, H: 22}},
		{ID: "d", Parent: "bar", Role: "button", Label: "D", Rect: Rect{X: 73, Y: 0, W: 22, H: 22}},
	})
	got := findings(res, RuleGap)
	if len(got) != 1 || res.Boxes[got[0].Box].Label != "C" || got[0].Measured != 3 || got[0].Expected != 2 {
		t.Fatalf("gap findings = %+v", got)
	}
	if !strings.Contains(got[0].Fix, "token.GapTight") {
		t.Errorf("fix %q names no token", got[0].Fix)
	}
}

func TestNearMissAlignment(t *testing.T) {
	res := run(t, canvasOf(), []Box{
		{ID: "view", Role: "group", Rect: Rect{X: 0, Y: 0, W: 120, H: 80}},
		{ID: "a", Parent: "view", Role: "button", Label: "Top", Rect: Rect{X: 20, Y: 0, W: 60, H: 22}},
		{ID: "b", Parent: "view", Role: "button", Label: "Bottom", Rect: Rect{X: 21, Y: 30, W: 70, H: 22}},
		{ID: "c", Parent: "view", Role: "button", Label: "Apart", Rect: Rect{X: 100, Y: 56, W: 10, H: 22}},
	})
	got := findings(res, RuleAlign)
	if len(got) != 1 || got[0].Check != "left edge" || got[0].Measured != 1 {
		t.Fatalf("align findings = %+v", got)
	}
}

func TestPaddingGrid(t *testing.T) {
	res := run(t, canvasOf(), []Box{
		{ID: "panel", Role: "group", Rect: Rect{X: 0, Y: 0, W: 100, H: 38}},
		{ID: "a", Parent: "panel", Role: "button", Label: "A", Rect: Rect{X: 6, Y: 8, W: 22, H: 22}},
		{ID: "box", Role: "group", Rect: Rect{X: 110, Y: 0, W: 80, H: 40}},
		{ID: "b", Parent: "box", Role: "button", Label: "B", Rect: Rect{X: 115, Y: 9, W: 22, H: 22}},
	})
	got := findings(res, RulePadding)
	if len(got) != 1 || got[0].Check != "padding left" || got[0].Measured != 6 || got[0].Expected != 8 {
		t.Fatalf("padding findings = %+v", got)
	}
}

func TestContrast(t *testing.T) {
	img := canvasOf()
	fill(img, Rect{X: 5, Y: 5, W: 12, H: 12}, testDim)
	res := run(t, img, []Box{{Role: "button", Label: "Faint", Rect: Rect{X: 0, Y: 0, W: 22, H: 22}}})
	got := findings(res, RuleContrast)
	if len(got) != 1 || got[0].Expected != contrastIcon || got[0].Measured >= contrastIcon {
		t.Fatalf("contrast findings = %+v", got)
	}
	res = run(t, img, []Box{{Role: "button", Label: "Faint", Disabled: true, Rect: Rect{X: 0, Y: 0, W: 22, H: 22}}})
	if len(findings(res, RuleContrast)) != 0 {
		t.Error("disabled element flagged for contrast")
	}
}

func TestCrispEdges(t *testing.T) {
	img := canvasOf()
	fill(img, Rect{X: 10, Y: 10, W: 60, H: 20}, testFill)
	blend := color.RGBA{R: 57, G: 57, B: 57, A: 255}
	fill(img, Rect{X: 80, Y: 10.5, W: 60, H: 0.5}, blend)
	fill(img, Rect{X: 80, Y: 11, W: 60, H: 20}, testFill)
	res := run(t, img, []Box{
		{Role: "group", Label: "Sharp", Rect: Rect{X: 10, Y: 10, W: 60, H: 20}},
		{Role: "group", Label: "Soft", Rect: Rect{X: 80, Y: 10.75, W: 60, H: 20.25}},
	})
	got := findings(res, RuleCrisp)
	if len(got) != 1 || res.Boxes[got[0].Box].Label != "Soft" || got[0].Check != "top edge blur" {
		t.Fatalf("crisp findings = %+v", got)
	}
}

func TestIconStrokeConsistency(t *testing.T) {
	img := canvasOf()
	var boxes []Box
	boxes = append(boxes, Box{ID: "bar", Role: "toolbar", Rect: Rect{X: 0, Y: 0, W: 120, H: 22}})
	for i, stroke := range []float64{1, 1, 3} {
		x := float64(i) * 24
		fill(img, Rect{X: x + 5, Y: 5, W: stroke, H: 12}, testInk)
		fill(img, Rect{X: x + 5, Y: 5, W: 12, H: stroke}, testInk)
		fill(img, Rect{X: x + 17 - stroke, Y: 5, W: stroke, H: 12}, testInk)
		fill(img, Rect{X: x + 5, Y: 17 - stroke, W: 12, H: stroke}, testInk)
		boxes = append(boxes, Box{ID: string(rune('a' + i)), Parent: "bar", Role: "button", Label: "Icon", Rect: Rect{X: x, Y: 0, W: 22, H: 22}})
	}
	got := findings(run(t, img, boxes), RuleIconSet)
	if len(got) != 1 || got[0].Check != "stroke weight" || got[0].Measured <= got[0].Expected {
		t.Fatalf("icon set findings = %+v", got)
	}
}

func TestDeterministic(t *testing.T) {
	img := canvasOf()
	fill(img, Rect{X: 6, Y: 5, W: 12, H: 12}, testInk)
	boxes := []Box{{Role: "button", Label: "A", Rect: Rect{X: 0, Y: 0, W: 22, H: 22}}, {Role: "button", Label: "B", Rect: Rect{X: 0, Y: 30, W: 23, H: 22}}}
	a, b := run(t, img, boxes), run(t, img, boxes)
	if !reflect.DeepEqual(a.Findings, b.Findings) || a.Report() != b.Report() || !reflect.DeepEqual(a.Image.Pix, b.Image.Pix) {
		t.Error("two runs differ")
	}
}

func TestReportAllowAndWrite(t *testing.T) {
	img := canvasOf()
	fill(img, Rect{X: 6, Y: 5, W: 12, H: 12}, testInk)
	res := run(t, img, []Box{{Role: "button", Label: "Refresh", Rect: Rect{X: 0, Y: 0, W: 22, H: 22}}})
	report := res.Report()
	for _, want := range []string{"element: button \"Refresh\"", "measured: +1 px", "expected: 0 px", "fix:", "allow key: optical button \"Refresh\" ink centre x", "crop: test.redline/01-optical.png"} {
		if !strings.Contains(report, want) {
			t.Errorf("report has no %q:\n%s", want, report)
		}
	}
	if len(res.Unallowed(nil)) != 1 || len(res.Unallowed([]string{"optical button \"Refresh\""})) != 0 {
		t.Error("allow prefix does not match the finding key")
	}
	dir := t.TempDir()
	files, err := res.Write(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range append([]string{files.Image, files.Report}, files.Crops...) {
		if _, err := os.Stat(p); err != nil {
			t.Error(err)
		}
	}
	if len(files.Crops) != 1 || filepath.Base(files.Crops[0]) != "01-optical.png" {
		t.Errorf("crops %v", files.Crops)
	}
}
