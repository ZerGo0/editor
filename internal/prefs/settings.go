package prefs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
)

const (
	categoryWidth   = 180
	contentPadding  = 20
	rowGap          = 6
	titleFontSize   = 13
	detailFontSize  = 12
	controlWidth    = 280
	numberStep      = 1
	decimalStep     = 0.1
	listSeparator   = ","
	editInJSONLabel = "Edit in settings.json"
	scopeUser       = 0
	scopeFolder     = 1
	overriddenLabel = "Overridden in Folder"
)

type SettingsView struct {
	Store          *settings.Store
	Folder         *settings.Store
	OpenJSON       func()
	OpenFolderJSON func()
	scope          int
	query          string
	category       string
	drafts         map[string]string
	err            string
	list           ui.ListState
}

func (v *SettingsView) ShowFolder() {
	v.scope = scopeFolder
	v.drafts = nil
}

func (v *SettingsView) store() *settings.Store {
	if v.scope == scopeFolder && v.Folder != nil {
		return v.Folder
	}
	return v.Store
}

func (v *SettingsView) openJSON() {
	open := v.OpenJSON
	if v.scope == scopeFolder && v.OpenFolderJSON != nil {
		open = v.OpenFolderJSON
	}
	if open != nil {
		open()
	}
}

func (v *SettingsView) View(c *ui.Context) {
	t := c.Theme()
	if v.category == "" {
		v.category = settings.Categories[0]
	}
	defs := v.visible()
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		toolbar(c, func() {
			if v.Folder != nil && ui.Segmented(c.Key("settings-scope"), &v.scope, "User", "Folder").Label("Settings scope").Changed() {
				v.drafts, v.err = nil, ""
			}
			ui.TextInput(c.Key("settings-search"), &v.query).Placeholder("Search settings").Label("Search settings").Grow(1)
			if ui.Button(c, "Open Settings (JSON)").Clicked() {
				v.openJSON()
			}
		})
		problems(c, t, append(slices.Clone(v.store().Errors), nonEmpty(v.err)...))
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			if v.query == "" {
				v.categories(c, t)
			}
			ui.List(c, &v.list, len(defs), func(i int) { v.row(c, t, defs[i]) }).Grow(1).FillHeight().Padding(8, contentPadding).Children(func() {
				if len(defs) == 0 {
					ui.Text(c, "No settings found").TextColor(t.TextMuted).Padding(12)
				}
			})
		})
	})
}

func (v *SettingsView) visible() []*settings.Def {
	var out []*settings.Def
	q := strings.ToLower(strings.TrimSpace(v.query))
	for i := range settings.Defs {
		d := &settings.Defs[i]
		if q == "" && d.Category != v.category || v.scope == scopeFolder && d.AppWide {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(d.Title+" "+d.Key+" "+d.Description), q) {
			continue
		}
		out = append(out, d)
	}
	return out
}

func (v *SettingsView) categories(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Width(categoryWidth).Shrink(0).FillHeight().Padding(8).Gap(2).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		for _, cat := range settings.Categories {
			b := ui.ButtonBase(c.Key(cat)).FillWidth().Padding(6, 10).Radius(5).Justify(ui.Start).Label(cat)
			switch {
			case cat == v.category:
				b.Background(t.Selection)
			case b.Hovered():
				b.Background(t.SurfaceHover)
			}
			b.Children(func() { ui.Text(c, cat).SingleLine() })
			if b.Clicked() {
				v.category = cat
				v.list.ScrollTo(0, ui.Start)
			}
		}
	})
}

func (v *SettingsView) row(c *ui.Context, t *ui.Theme, d *settings.Def) {
	modified := v.store().IsSet(d.Key)
	overridden := v.scope == scopeUser && v.Folder != nil && v.Folder.IsSet(d.Key)
	ui.Column(c.Key(d.Key)).Gap(rowGap).Padding(10, 0).FillWidth().Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.RichText(c, ui.Span{Text: d.Category + ": ", Color: t.TextMuted}, ui.Span{Text: d.Title, Weight: 600}).FontSize(titleFontSize)
			if modified {
				ui.Text(c, "Modified").FontSize(detailFontSize).TextColor(t.Accent)
				if ui.ButtonBase(c.Key("reset")).Label("Reset " + d.Title).Children(func() {
					ui.Text(c, "Reset").FontSize(detailFontSize).TextColor(t.TextMuted).Underline()
				}).Clicked() {
					v.apply(v.store().Reset(d.Key))
					delete(v.drafts, d.Key)
				}
			}
			if overridden {
				ui.Text(c, overriddenLabel).FontSize(detailFontSize).TextColor(t.TextMuted)
			}
		})
		ui.Text(c, d.Description).FontSize(detailFontSize).TextColor(t.TextMuted)
		ui.Text(c, d.Key).FontSize(detailFontSize).TextColor(t.TextMuted).Font("monospace")
		v.control(c, d)
	})
}

func (v *SettingsView) control(c *ui.Context, d *settings.Def) {
	s := v.store()
	switch d.Kind {
	case settings.Bool:
		on := s.Bool(d.Key)
		if ui.Switch(c.Key("control"), &on).Label(d.Title).Changed() {
			v.apply(s.Set(d.Key, on))
		}
	case settings.Enum:
		sel := s.String(d.Key)
		if ui.Select(c.Key("control"), &sel, d.Options).Width(controlWidth).Label(d.Title).Changed() {
			v.apply(s.Set(d.Key, sel))
		}
	case settings.Int, settings.Number:
		value, step := float64(s.Int(d.Key)), float64(numberStep)
		if d.Kind == settings.Number {
			value, step = s.Number(d.Key), decimalStep
		}
		if ui.NumberInput(c.Key("control"), &value, d.Min, d.Max, step).Width(controlWidth).Label(d.Title).Changed() {
			if d.Kind == settings.Int {
				v.apply(s.Set(d.Key, int(value)))
			} else {
				v.apply(s.Set(d.Key, value))
			}
		}
	case settings.JSON:
		if ui.Button(c.Key("control"), editInJSONLabel).Label(editInJSONLabel + ": " + d.Title).Clicked() {
			v.openJSON()
		}
	default:
		v.textControl(c, d)
	}
}

func (v *SettingsView) textControl(c *ui.Context, d *settings.Def) {
	if v.drafts == nil {
		v.drafts = map[string]string{}
	}
	draft, editing := v.drafts[d.Key]
	if !editing {
		draft = format(v.store().Value(d.Key))
	}
	input := ui.TextInput(c.Key("control"), &draft).Width(controlWidth).Label(d.Title)
	if input.Focused() {
		v.drafts[d.Key] = draft
	}
	if input.Submitted() || input.FocusLeft() {
		delete(v.drafts, d.Key)
		value, err := parse(d, draft)
		if err != nil {
			v.err = err.Error()
			return
		}
		if format(v.store().Value(d.Key)) != draft {
			v.apply(v.store().Set(d.Key, value))
		}
	}
}

func (v *SettingsView) apply(err error) {
	v.err = ""
	if err != nil {
		v.err = err.Error()
	}
}

func format(value any) string {
	switch x := value.(type) {
	case []int:
		parts := make([]string, len(x))
		for i, n := range x {
			parts[i] = strconv.Itoa(n)
		}
		return strings.Join(parts, listSeparator+" ")
	case []string:
		return strings.Join(x, listSeparator+" ")
	case string:
		return x
	}
	return fmt.Sprint(value)
}

func parse(d *settings.Def, text string) (any, error) {
	var parts []string
	for _, p := range strings.Split(text, listSeparator) {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	switch d.Kind {
	case settings.IntList:
		out := make([]int, 0, len(parts))
		for _, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return nil, fmt.Errorf("%s: %q is not a number", d.Title, p)
			}
			out = append(out, n)
		}
		return out, nil
	case settings.StringList:
		if parts == nil {
			parts = []string{}
		}
		return parts, nil
	}
	return text, nil
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
