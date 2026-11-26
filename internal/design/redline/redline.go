package redline

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
)

type Rect struct {
	X, Y, W, H float64
}

func (r Rect) Right() float64   { return r.X + r.W }
func (r Rect) Bottom() float64  { return r.Y + r.H }
func (r Rect) CenterX() float64 { return r.X + r.W/2 }
func (r Rect) CenterY() float64 { return r.Y + r.H/2 }

func (r Rect) String() string {
	return fmt.Sprintf("x %s y %s w %s h %s", num(r.X), num(r.Y), num(r.W), num(r.H))
}

type Box struct {
	ID       string
	Parent   string
	Role     string
	Label    string
	Rect     Rect
	Disabled bool
}

type Input struct {
	Name   string
	Image  image.Image
	Boxes  []Box
	Scale  float64
	Detail bool
}

type Result struct {
	Name     string
	Image    *image.RGBA
	Boxes    []Box
	Parents  []int
	Scale    float64
	Findings []Finding
	Measures []Measure
	Guides   int
	detail   bool
}

const (
	minScale = 0.5
	maxScale = 8
)

var (
	errNoImage  = errors.New("redline: no image")
	errBadScale = errors.New("redline: scale must be between 0.5 and 8")
)

func Analyze(in Input) (Result, error) {
	if in.Image == nil || in.Image.Bounds().Empty() {
		return Result{}, errNoImage
	}
	if math.IsNaN(in.Scale) || in.Scale < minScale || in.Scale > maxScale {
		return Result{}, fmt.Errorf("%w: got %v", errBadScale, in.Scale)
	}
	shot := toRGBA(in.Image)
	boxes, parents := visible(in.Boxes, shot.Bounds(), in.Scale)
	a := &analysis{shot: shot, boxes: boxes, parents: parents, scale: in.Scale}
	a.children = childrenOf(parents)
	a.layout()
	a.optical()
	a.iconSets()
	a.crisp()
	sortFindings(a.findings)
	out := Result{Name: in.Name, Boxes: boxes, Parents: parents, Scale: in.Scale, Findings: a.findings,
		Measures: a.measures, detail: in.Detail}
	out.Image, out.Guides = a.annotate()
	return out, nil
}

func ScaleOf(img image.Image, contentWidth float64) (float64, error) {
	if img == nil || img.Bounds().Empty() {
		return 0, errNoImage
	}
	if !(contentWidth > 0) || math.IsInf(contentWidth, 0) {
		return 0, fmt.Errorf("redline: content width must be positive, got %v", contentWidth)
	}
	s := float64(img.Bounds().Dx()) / contentWidth
	if s < minScale || s > maxScale {
		return 0, fmt.Errorf("%w: image %d px for %v px of content", errBadScale, img.Bounds().Dx(), contentWidth)
	}
	return s, nil
}

func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

type analysis struct {
	shot     *image.RGBA
	boxes    []Box
	parents  []int
	children map[int][]int
	scale    float64
	findings []Finding
	measures []Measure
	links    []link
	pads     []padding
}

func (a *analysis) add(f Finding) {
	a.findings = append(a.findings, f)
}

func (a *analysis) px(r Rect) image.Rectangle {
	return pxRect(r, a.scale)
}
