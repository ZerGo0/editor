package token

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

type pair struct {
	name   string
	fg, bg ui.Color
	min    float64
}

func palettes() map[string]*Palette {
	return map[string]*Palette{"dark": Dark(), "light": Light(), "dark-hc": For(true, ui.Transparent, true), "light-hc": For(false, ui.Transparent, true)}
}

func layers(p *Palette) map[string]ui.Color {
	return map[string]ui.Color{
		"sidebar": p.Layer.SideBar, "editor": p.Layer.Editor, "panel": p.Layer.Panel, "titlebar": p.Layer.TitleBar,
		"input": p.Layer.Input, "widget": p.Layer.Widget, "menu": p.Layer.Menu, "quickinput": p.Layer.QuickInput,
		"tabbar": p.Layer.TabBar, "statusbar": p.Layer.StatusBar,
	}
}

func rowBackgrounds(p *Palette) map[string]ui.Color {
	return map[string]ui.Color{
		"hover":    Over(p.List.Hover, p.Layer.SideBar),
		"inactive": Over(p.List.InactiveSelection, p.Layer.SideBar),
	}
}

func pairs(p *Palette) []pair {
	var out []pair
	for name, bg := range layers(p) {
		out = append(out,
			pair{"text.primary on " + name, p.Text.Primary, bg, ContrastText},
			pair{"text.secondary on " + name, p.Text.Secondary, bg, ContrastText},
			pair{"icon on " + name, p.Icon, bg, ContrastLarge},
			pair{"focus on " + name, p.Accent.Focus, bg, ContrastLarge},
		)
	}
	main := map[string]ui.Color{"sidebar": p.Layer.SideBar, "editor": p.Layer.Editor}
	for name, bg := range rowBackgrounds(p) {
		main[name] = bg
	}
	for name, bg := range main {
		out = append(out,
			pair{"text.primary on " + name, p.Text.Primary, bg, ContrastText},
			pair{"text.secondary on " + name, p.Text.Secondary, bg, ContrastText},
			pair{"list.highlight on " + name, p.List.Highlight, bg, ContrastText},
			pair{"link on " + name, p.Text.Link, bg, ContrastText},
		)
		for t := ToneInfo; t <= ToneError; t++ {
			out = append(out, pair{t.String() + " on " + name, p.Tone(t), bg, ContrastLarge})
		}
		for s := GitAdded; s <= GitSubmodule; s++ {
			out = append(out, pair{"git " + s.Letter() + " on " + name, p.GitColor(s), bg, ContrastText})
		}
	}
	active := Over(p.List.ActiveSelection, p.Layer.SideBar)
	pick := Over(p.List.PickSelection, p.Layer.QuickInput)
	out = append(out,
		pair{"selection text", p.List.ActiveSelectionText, active, ContrastText},
		pair{"selection secondary", p.List.ActiveSelectionSecondary, active, ContrastText},
		pair{"selection highlight", p.List.FocusHighlight, active, ContrastText},
		pair{"pick text", p.List.PickSelectionText, pick, ContrastText},
		pair{"pick highlight", p.List.PickHighlight, pick, ContrastText},
		pair{"accent text", p.Accent.Text, p.Accent.Base, ContrastText},
		pair{"accent hover text", p.Accent.Text, p.Accent.Hover, ContrastText},
		pair{"secondary button", p.Control.SecondaryText, p.Control.Secondary, ContrastText},
		pair{"badge", p.Badge.Text, p.Badge.Background, ContrastText},
		pair{"accent badge", p.Badge.AccentText, p.Badge.Accent, ContrastText},
		pair{"key cap", p.Key.Text, Over(p.Key.Background, p.Layer.Widget), ContrastText},
		pair{"status text", p.Bar.StatusText, p.Layer.StatusBar, ContrastText},
		pair{"status hover", p.Bar.StatusText, Over(p.Bar.StatusHover, p.Layer.StatusBar), ContrastText},
		pair{"active tab", p.Tab.ActiveText, p.Tab.Active, ContrastText},
		pair{"inactive tab", p.Tab.InactiveText, p.Tab.Inactive, ContrastText},
		pair{"menu selection", p.Menu.SelectionText, Over(p.Menu.Selection, p.Layer.Menu), ContrastText},
		pair{"activity active", p.Bar.ActivityActive, p.Layer.SideBar, ContrastLarge},
		pair{"activity inactive", p.Bar.ActivityInactive, p.Layer.SideBar, ContrastLarge},
		pair{"error notice", p.Text.Primary, Over(p.Severity.ErrorBackground, p.Layer.SideBar), ContrastText},
		pair{"warning notice", p.Text.Primary, Over(p.Severity.WarningBackground, p.Layer.SideBar), ContrastText},
		pair{"info notice", p.Text.Primary, Over(p.Severity.InfoBackground, p.Layer.SideBar), ContrastText},
	)
	for i, c := range p.Avatar {
		out = append(out, pair{"avatar " + string(rune('0'+i)), p.OnAvatar, c, ContrastText})
	}
	return out
}

func TestContrast(t *testing.T) {
	for theme, p := range palettes() {
		for _, pr := range pairs(p) {
			fg := Over(pr.fg, pr.bg)
			if got := Contrast(fg, pr.bg); got < pr.min {
				t.Errorf("%s: %s is %.2f:1, want %.1f:1", theme, pr.name, got, pr.min)
			}
		}
	}
}

func TestSystemAccentKeepsContrast(t *testing.T) {
	for _, accent := range []ui.Color{ui.Hex("#FF9500"), ui.Hex("#34C759"), ui.Hex("#AF52DE"), ui.Hex("#5B7CFF"), ui.Hex("#FFCC00")} {
		for _, dark := range []bool{true, false} {
			p := For(dark, accent, false)
			for _, pr := range []pair{
				{"accent text", p.Accent.Text, p.Accent.Base, ContrastText},
				{"selection text", p.List.ActiveSelectionText, Over(p.List.ActiveSelection, p.Layer.SideBar), ContrastText},
				{"highlight", p.List.Highlight, p.Layer.SideBar, ContrastText},
				{"link", p.Text.Link, p.Layer.Editor, ContrastText},
			} {
				if got := Contrast(Over(pr.fg, pr.bg), pr.bg); got < pr.min {
					t.Errorf("accent %v dark=%v: %s is %.2f:1", accent, dark, pr.name, got)
				}
			}
		}
	}
}

func TestRegistry(t *testing.T) {
	if Dark() != Dark() || Light() != Light() {
		t.Fatal("palettes are not cached")
	}
	if Dark().Theme() != Dark().Theme() {
		t.Fatal("themes are not cached")
	}
	if !Dark().Theme().Dark || Light().Theme().Dark {
		t.Fatal("theme appearance does not match palette")
	}
	var got *Palette
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(Light().Theme())
		got = Of(c)
	}, 100, 100)
	tt.Frame()
	if got != Light() {
		t.Fatal("Of does not find the palette of the theme in use")
	}
}
