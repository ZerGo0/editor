package token

import (
	"math"

	"github.com/egoist/mygo/ui"
)

const (
	ContrastText  = 4.5
	ContrastLarge = 3.0
)

const (
	channelMax       = 255
	linearThreshold  = 0.03928
	linearDivisor    = 12.92
	gammaOffset      = 0.055
	gammaDivisor     = 1.055
	gamma            = 2.4
	luminanceRed     = 0.2126
	luminanceGreen   = 0.7152
	luminanceBlue    = 0.0722
	luminanceFlare   = 0.05
	avatarColors     = 8
	avatarHashPrime  = 31
	onAccentFallback = "#1F1F1F"
	luminanceMid     = 0.179
)

const contrastStep float32 = 0.05

func Contrast(fg, bg ui.Color) float64 {
	a, b := luminance(Over(fg, bg)), luminance(bg)
	return (max(a, b) + luminanceFlare) / (min(a, b) + luminanceFlare)
}

func Over(fg, bg ui.Color) ui.Color {
	fg, bg = fg.SRGB(), bg.SRGB()
	alpha := float32(fg.A) / channelMax
	opaque := fg
	opaque.A = channelMax
	out := bg.Mix(opaque, alpha)
	out.A = channelMax
	return out
}

func luminance(c ui.Color) float64 {
	c = c.SRGB()
	return Luminance(c.R, c.G, c.B)
}

func Luminance(r, g, b uint8) float64 {
	return luminanceRed*linearChannel(r) + luminanceGreen*linearChannel(g) + luminanceBlue*linearChannel(b)
}

func linearChannel(v uint8) float64 {
	s := float64(v) / channelMax
	if s <= linearThreshold {
		return s / linearDivisor
	}
	return math.Pow((s+gammaOffset)/gammaDivisor, gamma)
}

func ensure(fg ui.Color, want float64, bgs ...ui.Color) ui.Color {
	if len(bgs) == 0 {
		return fg
	}
	toward := ui.RGB(0, 0, 0)
	if luminance(bgs[0]) < luminanceMid {
		toward = ui.RGB(channelMax, channelMax, channelMax)
	}
	for t := float32(0); t < 1; t += contrastStep {
		c := fg.Mix(toward, t)
		if passes(c, want, bgs) {
			return c
		}
	}
	return toward
}

func passes(fg ui.Color, want float64, bgs []ui.Color) bool {
	for _, bg := range bgs {
		if Contrast(fg, bg) < want {
			return false
		}
	}
	return true
}

func onColor(bg ui.Color) ui.Color {
	white := ui.RGB(channelMax, channelMax, channelMax)
	if Contrast(white, bg) >= ContrastText {
		return white
	}
	return ui.Hex(onAccentFallback)
}
