//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ZerGo0/editor/internal/workspace"
)

const (
	previewOutEnv         = "EDITOR_PREVIEW_OUT"
	previewZoomLevel      = 4
	previewWidth          = 1280
	previewHeight         = 820
	previewStateKeyPrefix = "project:"
	previewSplitTop       = 0.6
	previewSplitBottom    = 0.4
	previewDividerWidth   = 2.0
	previewCornerRadius   = 16.0
	previewMarginSide     = 56.0
	previewMarginTop      = 38.0
	previewMarginBottom   = 74.0
	previewShadowOffset   = 18.0
	previewShadowBlur     = 10.0
	previewShadowOpacity  = 0.35
	previewBlurPasses     = 3
	previewLightsScript   = "testdata/preview/lights.swift"
	previewOpenFile       = "internal/workspace/editordrop.go"
	previewSideFile       = "tabmove.go"
)

var (
	previewFiles = []string{
		"main.go", "go.mod", "docs/design.md",
		"internal/workspace/editordrop.go", "internal/workspace/tabmove.go", "internal/workspace/keys.go", "internal/workspace/search.go",
		"internal/layout/node.go", "internal/layout/group.go", "internal/keymap/keymap.go", "internal/session/session.go",
	}
	previewThemes  = []struct{ setting, appearance string }{{"Light", "light"}, {"Dark", "dark"}}
	previewDivider = color.NRGBA{R: 128, G: 128, B: 128, A: 153}
)

func TestReadmePreview(t *testing.T) {
	out := os.Getenv(previewOutEnv)
	if out == "" {
		t.Skip("set " + previewOutEnv + " to write the README preview")
	}
	root := previewProject(t)
	var shots []*image.NRGBA
	var scale float64
	for _, theme := range previewThemes {
		shot := previewCapture(t, root, theme.setting)
		scale = float64(shot.Bounds().Dx()) / previewWidth
		lights := previewLights(t, theme.appearance, scale)
		at := workspace.TrafficLightPosition()
		origin := image.Pt(int(math.Round(float64(at.X)*scale)), int(math.Round(float64(at.Y)*scale)))
		draw.Draw(shot, lights.Bounds().Add(origin), lights, lights.Bounds().Min, draw.Over)
		shots = append(shots, shot)
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, previewCompose(shots[0], shots[1], scale)); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", out)
}

func previewProject(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "editor")
	for _, rel := range previewFiles {
		data, err := os.ReadFile(filepath.Join("..", rel))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, string(data))
	}
	return root
}

func previewCapture(t *testing.T, root, theme string) *image.NRGBA {
	t.Helper()
	userData := t.TempDir()
	zoom := math.Pow(workspace.ZoomFactor, previewZoomLevel)
	writeFile(t, filepath.Join(userData, "settings.json"), fmt.Sprintf(`{"workbench.colorTheme": %q, "window.zoomLevel": %d}`, theme, previewZoomLevel))
	state, err := json.Marshal(map[string]map[string]int{previewStateKeyPrefix + root: {
		"width": int(math.Round(previewWidth * zoom)), "height": int(math.Round(previewHeight * zoom)),
	}})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(userData, "window-state.json"), string(state))
	app := LaunchWith(t, userData, root, filepath.Join(root, previewOpenFile))
	defer app.Quit()
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.Key(win, "Cmd+\\")
	app.Key(win, "Cmd+P")
	app.Type(win, previewSideFile)
	app.Key(win, "Enter")
	app.WaitNode(win, previewSideFile+" tab", RoleLabel("tab", previewSideFile))
	data, err := app.Capture()
	if err != nil {
		t.Fatal(err)
	}
	return previewDecode(t, data)
}

func previewLights(t *testing.T, appearance string, scale float64) *image.NRGBA {
	t.Helper()
	out := filepath.Join(t.TempDir(), "lights.png")
	cmd := exec.Command("swift", previewLightsScript, appearance, strconv.FormatFloat(scale, 'f', -1, 64), out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render the traffic lights: %v\n%s", err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return previewDecode(t, data)
}

func previewDecode(t *testing.T, data []byte) *image.NRGBA {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	out := image.NewNRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}

func previewCompose(light, dark *image.NRGBA, scale float64) *image.NRGBA {
	w, h := light.Bounds().Dx(), light.Bounds().Dy()
	radius := previewCornerRadius * scale
	window := image.NewNRGBA(light.Bounds())
	top, bottom := float64(w)*previewSplitTop, float64(w)*previewSplitBottom
	half := previewDividerWidth * scale / 2
	for y := range h {
		fy := float64(y) + 0.5
		split := top + (bottom-top)*fy/float64(h)
		for x := range w {
			fx := float64(x) + 0.5
			c := light.NRGBAAt(x, y)
			if fx > split {
				c = dark.NRGBAAt(x, y)
			}
			if math.Abs(fx-split) <= half {
				c = previewBlend(c, previewDivider)
			}
			c.A = uint8(math.Round(float64(c.A) * previewCoverage(fx, fy, float64(w), float64(h), radius)))
			window.SetNRGBA(x, y, c)
		}
	}
	side, above, below := int(previewMarginSide*scale), int(previewMarginTop*scale), int(previewMarginBottom*scale)
	cw, ch := w+2*side, h+above+below
	shadow := make([]float64, cw*ch)
	offset := int(previewShadowOffset * scale)
	for y := range h {
		for x := range w {
			if sy := y + above + offset; sy < ch {
				shadow[sy*cw+x+side] = previewShadowOpacity * previewCoverage(float64(x)+0.5, float64(y)+0.5, float64(w), float64(h), radius)
			}
		}
	}
	for range previewBlurPasses {
		previewBoxBlur(shadow, cw, ch, int(previewShadowBlur*scale))
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, cw, ch))
	for i, a := range shadow {
		canvas.Pix[i*4+3] = uint8(math.Round(math.Min(a, 1) * 255))
	}
	draw.Draw(canvas, window.Bounds().Add(image.Pt(side, above)), window, image.Point{}, draw.Over)
	return canvas
}

func previewBlend(c, over color.NRGBA) color.NRGBA {
	a := float64(over.A) / 255
	mix := func(b, o uint8) uint8 { return uint8(math.Round(float64(b)*(1-a) + float64(o)*a)) }
	return color.NRGBA{R: mix(c.R, over.R), G: mix(c.G, over.G), B: mix(c.B, over.B), A: c.A}
}

func previewCoverage(x, y, w, h, radius float64) float64 {
	cx, cy := math.Min(math.Max(x, radius), w-radius), math.Min(math.Max(y, radius), h-radius)
	return math.Min(math.Max(radius-math.Hypot(x-cx, y-cy)+0.5, 0), 1)
}

func previewBoxBlur(a []float64, w, h, r int) {
	tmp := make([]float64, len(a))
	previewBlurLines(a, tmp, w, h, r, 1, w)
	previewBlurLines(tmp, a, h, w, r, w, 1)
}

func previewBlurLines(src, dst []float64, length, lines, r, stride, step int) {
	n := float64(2*r + 1)
	for l := range lines {
		base := l * step
		at := func(i int) float64 {
			if i < 0 || i >= length {
				return 0
			}
			return src[base+i*stride]
		}
		sum := 0.0
		for i := -r; i <= r; i++ {
			sum += at(i)
		}
		for i := range length {
			dst[base+i*stride] = sum / n
			sum += at(i+r+1) - at(i-r)
		}
	}
}
