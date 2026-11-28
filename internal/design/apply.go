package design

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type Appearance uint8

const (
	AppearanceSystem Appearance = iota
	AppearanceDark
	AppearanceLight
)

type Settings struct {
	Appearance         Appearance
	HighContrast       bool
	FollowHighContrast bool
	SystemAccent       bool
}

func Apply(c *ui.Context, s Settings) *token.Palette {
	dark := c.Theme().Dark
	switch s.Appearance {
	case AppearanceDark:
		dark = true
	case AppearanceLight:
		dark = false
	}
	prefs := c.Preferences()
	accent := ui.Transparent
	if s.SystemAccent {
		accent = prefs.Accent
	}
	p := token.For(dark, accent, s.HighContrast || s.FollowHighContrast && prefs.HighContrast)
	c.SetTheme(p.Theme())
	c.DisableAnimations()
	return p
}
