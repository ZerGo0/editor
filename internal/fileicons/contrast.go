package fileicons

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"sync"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	iconContrastTarget = 3.2
	iconContrastFloor  = 3.0
	lightKnockout      = 0.85
	darkKnockout       = 0.03
	detailID           = "motive"
	shadeStep          = 0.01
	channelMax         = 255
	hexDigits          = 6
	shortHexDigits     = 3
	hexBase            = 16
	shortHexRepeat     = 17
	srgbLow            = 0.03928
	srgbLowDivisor     = 12.92
	srgbOffset         = 0.055
	srgbScale          = 1.055
	srgbGamma          = 2.4
	redWeight          = 0.2126
	greenWeight        = 0.7152
	blueWeight         = 0.0722
)

var (
	colorAttr = regexp.MustCompile(`\b(fill|stroke|stop-color)="#([0-9a-fA-F]{6}|[0-9a-fA-F]{3})"`)
	tagPlain  = regexp.MustCompile(`<[a-zA-Z][^>]*>`)
)

type rgb struct{ r, g, b uint8 }

func (c rgb) hex() string { return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b) }

func (c rgb) ui() ui.Color { return ui.RGB(c.r, c.g, c.b) }

func parseHex(digits string) (rgb, bool) {
	if len(digits) == shortHexDigits {
		v, err := strconv.ParseUint(digits, hexBase, 16)
		if err != nil {
			return rgb{}, false
		}
		return rgb{uint8(v >> 8 & 0xf * shortHexRepeat), uint8(v >> 4 & 0xf * shortHexRepeat), uint8(v & 0xf * shortHexRepeat)}, true
	}
	if len(digits) != hexDigits {
		return rgb{}, false
	}
	v, err := strconv.ParseUint(digits, hexBase, 32)
	if err != nil {
		return rgb{}, false
	}
	return rgb{uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
}

func linear(v uint8) float64 {
	s := float64(v) / channelMax
	if s <= srgbLow {
		return s / srgbLowDivisor
	}
	return pow((s+srgbOffset)/srgbScale, srgbGamma)
}

func luminance(c rgb) float64 {
	return redWeight*linear(c.r) + greenWeight*linear(c.g) + blueWeight*linear(c.b)
}

var (
	surfacesOnce  sync.Once
	darkSurfaces  []ui.Color
	lightSurfaces []ui.Color
)

func surfacesOf(p *token.Palette, selected ...ui.Color) []ui.Color {
	return append([]ui.Color{
		p.Layer.Editor, p.Layer.SideBar, p.Layer.Panel, p.Layer.TabBar, p.Layer.Menu,
		p.Tab.Active, p.Tab.Inactive, p.Tab.Hover, p.List.Hover,
	}, selected...)
}

func surfaces(dark bool) []ui.Color {
	surfacesOnce.Do(func() {
		dark, light := token.Dark(), token.Light()
		darkSurfaces = surfacesOf(dark)
		lightSurfaces = surfacesOf(light, light.List.ActiveSelection, light.List.InactiveSelection)
	})
	if dark {
		return darkSurfaces
	}
	return lightSurfaces
}

func worstContrast(c rgb, dark bool) float64 {
	worst := -1.0
	for _, s := range surfaces(dark) {
		if v := token.Contrast(c.ui(), s); worst < 0 || v < worst {
			worst = v
		}
	}
	return worst
}

func knockout(c rgb, dark bool) bool {
	if dark {
		return luminance(c) < darkKnockout
	}
	return luminance(c) >= lightKnockout
}

func collectColors(svg []byte, dark bool) (required, all []rgb) {
	seen := map[rgb]bool{}
	for _, tag := range tagPlain.FindAll(svg, -1) {
		detail := bytes.Contains(tag, []byte(`id="`+detailID+`"`))
		for _, m := range colorAttr.FindAllSubmatch(tag, -1) {
			c, ok := parseHex(string(m[2]))
			if !ok {
				continue
			}
			if !seen[c] {
				seen[c] = true
				all = append(all, c)
			}
			if !detail && !knockout(c, dark) {
				required = append(required, c)
			}
		}
	}
	if len(required) == 0 {
		required = all
	}
	return required, all
}

func shade(c rgb, dark bool, t float64) rgb {
	mix := func(v uint8) uint8 {
		if dark {
			return uint8(float64(v) + (channelMax-float64(v))*t + 0.5)
		}
		return uint8(float64(v)*(1-t) + 0.5)
	}
	return rgb{mix(c.r), mix(c.g), mix(c.b)}
}

func shadeFor(colors []rgb, dark bool) float64 {
	for t := 0.0; t <= 1; t += shadeStep {
		ok := true
		for _, c := range colors {
			if worstContrast(shade(c, dark, t), dark) < iconContrastTarget {
				ok = false
				break
			}
		}
		if ok {
			return t
		}
	}
	return 1
}

func adapt(svg []byte, dark bool) []byte {
	out, _ := adaptWithColors(svg, dark)
	return out
}

func adaptWithColors(svg []byte, dark bool) ([]byte, []rgb) {
	required, _ := collectColors(svg, dark)
	t := shadeFor(required, dark)
	shown := make([]rgb, len(required))
	for i, c := range required {
		shown[i] = shade(c, dark, t)
	}
	if t == 0 {
		return svg, shown
	}
	return colorAttr.ReplaceAllFunc(svg, func(attr []byte) []byte {
		m := colorAttr.FindSubmatch(attr)
		c, ok := parseHex(string(m[2]))
		if !ok {
			return attr
		}
		return []byte(string(m[1]) + `="` + shade(c, dark, t).hex() + `"`)
	}), shown
}

func pow(x, y float64) float64 { return math.Pow(x, y) }
