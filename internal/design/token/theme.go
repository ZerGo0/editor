package token

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	themeSpacing     float32 = 4
	surfaceHoverMix  float32 = 0.08
	surfacePressMix  float32 = 0.14
	accentPressMix   float32 = 0.1
	accentHoverMix   float32 = 0.1
	darkSelectionMix float32 = 0.7
	darkLinkMix      float32 = 0.3
	darkMatchMix     float32 = 0.35
	toggleDarkAlpha  float32 = 0.5
	toggleLightAlpha float32 = 0.25
	contrastMix      float32 = 0.45
	contrastTextMix  float32 = 0.4
)

type paletteKey struct {
	dark, highContrast bool
	r, g, b, a         uint8
}

var registry = struct {
	sync.Mutex
	palettes map[paletteKey]*Palette
	byTheme  map[*ui.Theme]*Palette
	themes   map[*Palette]*ui.Theme
	code     map[*Palette]*editor.Theme
}{
	palettes: map[paletteKey]*Palette{},
	byTheme:  map[*ui.Theme]*Palette{},
	themes:   map[*Palette]*ui.Theme{},
	code:     map[*Palette]*editor.Theme{},
}

func Dark() *Palette  { return For(true, ui.Transparent, false) }
func Light() *Palette { return For(false, ui.Transparent, false) }

func For(dark bool, accent ui.Color, highContrast bool) *Palette {
	accent = accent.SRGB()
	key := paletteKey{dark: dark, highContrast: highContrast, r: accent.R, g: accent.G, b: accent.B, a: accent.A}
	registry.Lock()
	defer registry.Unlock()
	if p, ok := registry.palettes[key]; ok {
		return p
	}
	p := lightPalette()
	if dark {
		p = darkPalette()
	}
	if accent.A > 0 {
		p.withAccent(accent)
	}
	if highContrast {
		p.withHighContrast()
	}
	p.enforceContrast()
	registry.palettes[key] = &p
	return &p
}

func Of(c *ui.Context) *Palette {
	t := c.Theme()
	registry.Lock()
	p, ok := registry.byTheme[t]
	registry.Unlock()
	if ok {
		return p
	}
	if t != nil && t.Dark {
		return Dark()
	}
	return Light()
}

func (p *Palette) Theme() *ui.Theme {
	registry.Lock()
	defer registry.Unlock()
	if t, ok := registry.themes[p]; ok {
		return t
	}
	t := p.uiTheme()
	registry.themes[p] = t
	registry.byTheme[t] = p
	return t
}

func (p *Palette) uiTheme() *ui.Theme {
	t := ui.LightTheme()
	if p.Dark {
		t = ui.DarkTheme()
	}
	black := ui.RGB(0, 0, 0)
	t.Background = p.Layer.Editor
	t.Surface = p.Layer.Input
	t.SurfaceHover = p.Layer.Input.Mix(p.Text.Primary, surfaceHoverMix)
	t.SurfacePressed = p.Layer.Input.Mix(p.Text.Primary, surfacePressMix)
	t.Border = p.Border.Input
	t.Text = p.Text.Primary
	t.TextMuted = p.Text.Muted
	t.Accent = p.Accent.Base
	t.AccentHover = p.Accent.Hover
	t.AccentPressed = p.Accent.Hover.Mix(black, accentPressMix)
	t.AccentText = p.Accent.Text
	t.InactiveAccent = p.Accent.Base
	t.Danger = p.Severity.Error
	t.Warning = p.Severity.Warning
	t.Success = p.Severity.Success
	t.Selection = p.Text.Selection
	t.Focus = p.Accent.Focus
	t.FocusRingWidth = FocusRingWidth.Px()
	t.FocusRingOffset = FocusRingOffset.Px()
	t.Inverse = p.Layer.Widget
	t.InverseText = p.Text.Primary
	t.Scrollbar = p.Control.Scrollbar
	t.Radius = RadiusControl.Px()
	t.Spacing = themeSpacing
	t.FontSize = TextBody.Px()
	t.TooltipDelay = TooltipDelay
	return t
}

func (p *Palette) withAccent(a ui.Color) {
	a.A = channelMax
	white, black := ui.RGB(channelMax, channelMax, channelMax), ui.RGB(0, 0, 0)
	text := onColor(a)
	a = ensure(a, ContrastText, text)
	p.Accent = Accents{Base: a, Hover: a.Mix(black, accentHoverMix), Text: text, Focus: a}
	p.Badge.Accent, p.Badge.AccentText = a, text
	p.Tab.ActiveBorderTop = a
	p.List.FocusOutline = a
	p.Menu.Selection, p.Menu.SelectionText = a, text
	p.Control.ToggleActiveBorder = a
	p.Diff.GutterModified = a
	if p.Dark {
		selection := a.Mix(p.Layer.SideBar, darkSelectionMix)
		match := a.Mix(white, darkMatchMix)
		p.Control.ToggleActive = a.Alpha(toggleDarkAlpha)
		p.List.ActiveSelection, p.List.PickSelection = selection, selection
		p.List.Highlight, p.List.FocusHighlight, p.List.PickHighlight = match, match, match
		p.Text.Link = a.Mix(white, darkLinkMix)
		return
	}
	p.Control.ToggleActive = a.Alpha(toggleLightAlpha)
	p.List.PickSelection, p.List.PickSelectionText, p.List.PickHighlight = a, text, text
	p.List.Highlight, p.List.FocusHighlight = a, a
	p.Text.Link = a
}

func (p *Palette) enforceContrast() {
	rows := []ui.Color{p.Layer.SideBar, p.Layer.Editor, Over(p.List.Hover, p.Layer.SideBar), Over(p.List.InactiveSelection, p.Layer.SideBar)}
	surfaces := append([]ui.Color{p.Layer.Input, p.Layer.Widget, p.Layer.Menu, p.Layer.Panel, p.Layer.SectionHeader, p.Layer.QuickInput}, rows...)
	p.Text.Secondary = ensure(p.Text.Secondary, ContrastText, surfaces...)
	p.Text.Link = ensure(p.Text.Link, ContrastText, rows...)
	p.enforceSyntax()
	p.List.Highlight = ensure(p.List.Highlight, ContrastText, rows...)
	p.Accent.Focus = ensure(p.Accent.Focus, ContrastLarge, surfaces...)
	p.Accent.Hover = ensure(p.Accent.Hover, ContrastText, p.Accent.Text)
	for _, c := range []*ui.Color{&p.Git.Added, &p.Git.Modified, &p.Git.Deleted, &p.Git.Renamed, &p.Git.Untracked, &p.Git.Ignored, &p.Git.Conflict, &p.Git.Submodule} {
		*c = ensure(*c, ContrastText, rows...)
	}
}

func (p *Palette) withHighContrast() {
	strong := func(c ui.Color) ui.Color { return c.Mix(p.Text.Primary, contrastMix) }
	p.Border = Borders{Default: strong(p.Border.Default), Input: strong(p.Border.Input), Widget: strong(p.Border.Widget), Section: strong(p.Border.Section)}
	p.Tab.Border = strong(p.Tab.Border)
	p.List.IndentGuide = strong(p.List.IndentGuide)
	p.Text.Secondary = p.Text.Secondary.Mix(p.Text.Primary, contrastTextMix)
	p.Text.Muted = p.Text.Muted.Mix(p.Text.Primary, contrastTextMix)
	p.Tab.InactiveText = p.Tab.InactiveText.Mix(p.Text.Primary, contrastTextMix)
	p.strengthenSyntax()
}
