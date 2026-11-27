package prefs

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	listSeparator    = ","
	editInJSONLabel  = "Edit in settings.json"
	scopeUser        = 0
	scopeFolder      = 1
	overriddenLabel  = "Also modified in Folder"
	searchLabel      = "Search settings"
	settingsLabel    = "Settings"
	categoriesLabel  = "Setting Categories"
	modifiedOnly     = "Show Modified Settings Only"
	resetLabel       = "Reset"
	resetSetting     = "Reset Setting"
	copySettingID    = "Copy Setting ID"
	copySettingJSON  = "Copy Setting as JSON"
	settingsJSONOpen = "Open Settings (JSON)"
)

type SettingsView struct {
	Store          *settings.Store
	Folder         *settings.Store
	OpenJSON       func()
	OpenFolderJSON func()
	scope          int
	query          string
	modified       bool
	category       int
	drafts         map[string]string
	err            string
	list           ui.ListState
	nav            ui.ListState
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

func (v *SettingsView) filter() settingFilter {
	f := parseSettingFilter(v.query)
	f.modified = f.modified || v.modified
	return f
}

func (v *SettingsView) View(c *ui.Context) {
	f := v.filter()
	defs := v.visible(f)
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		toolbar(c, settingsLabel, func() {
			if v.Folder != nil && action.Segmented(c, "Settings scope", &v.scope, []string{"User", "Folder"}, action.SegmentedOptions{}).Changed() {
				v.drafts, v.err = nil, ""
			}
			ui.Box(c).Grow(1).MinWidth(0).Children(func() {
				field.SearchInput(c, searchLabel, &v.query, field.SearchOptions{
					Placeholder: "Search settings by name or ID, @modified, @id:",
					Toggles:     []field.Toggle{{Icon: icons.Filter, Label: modifiedOnly, On: &v.modified}},
					ID:          "settings-search",
				})
			})
			if action.Button(c, settingsJSONOpen, action.ButtonOptions{Variant: action.Ghost, Icon: icons.File}).Clicked() {
				v.openJSON()
			}
		})
		problems(c, append(slices.Clone(v.store().Errors), nonEmpty(v.err)...))
		ui.Row(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
			if !f.active() {
				v.categories(c)
			}
			l := ui.List(c, &v.list, len(defs)).Grow(1).FillHeight().MinWidth(0).Padding(token.GapStack.Px(), token.PadView.Px()).Label(settingsLabel)
			l.Rows(func(r ui.ListRow) { v.row(r.Context, defs[r.Index]) })
			if len(defs) == 0 {
				l.Children(func() { v.empty(c, f) })
			}
		})
	})
}

func (v *SettingsView) empty(c *ui.Context, f settingFilter) {
	o := feedback.EmptyOptions{Icon: icons.Search, Title: "No settings found", Message: "No setting matches the search."}
	if f.active() {
		o.Action, o.OnAction = "Clear Search", func() { v.query, v.modified = "", false }
	}
	feedback.Empty(c, o)
}

func (v *SettingsView) visible(f settingFilter) []*settings.Def {
	var out []*settings.Def
	s := v.store()
	category := settings.Categories[min(max(v.category, 0), len(settings.Categories)-1)]
	for i := range settings.Defs {
		d := &settings.Defs[i]
		if v.scope == scopeFolder && d.AppWide {
			continue
		}
		if !f.active() && d.Category != category {
			continue
		}
		if f.match(d, s.IsSet(d.Key)) {
			out = append(out, d)
		}
	}
	return out
}

func (v *SettingsView) categories(c *ui.Context) {
	p := token.Of(c)
	if v.nav.Selected == nil {
		v.nav.Selected = &v.category
	}
	ui.Column(c).Width(token.SettingsNavWidth.Px()).Shrink(0).FillHeight().Padding(token.GapStack.Px(), 0).
		BorderWidth(0, token.BorderWidth.Px(), 0, 0).BorderColor(p.Border.Section).Children(func() {
		list.View(c, &v.nav, len(settings.Categories), list.ViewOptions{Label: categoriesLabel}, func(i int) list.Item {
			label := settings.Categories[i]
			return list.Item{Label: label, OnClick: func() { v.list.ScrollTo(0, ui.Start) }, Rich: func(s list.Style) []ui.Span {
				if s.Text == p.Text.Primary {
					s.Text = p.Text.Strong
				}
				return list.Spans(label, nil, s.Text, s.Highlight)
			}}
		})
	})
}

func (v *SettingsView) row(c *ui.Context, d *settings.Def) {
	s := v.store()
	modified := s.IsSet(d.Key)
	o := design.SettingOptions{Category: d.Category, Modified: modified, Menu: func(m *ui.Menu) { v.menu(m, d, modified) }}
	if v.scope == scopeUser && v.Folder != nil && v.Folder.IsSet(d.Key) {
		o.Note = overriddenLabel
	}
	design.SettingRow(c.Key(d.Key), d.Title, d.Description, o, func() {
		design.Inline(c, token.GapControl, func() {
			v.control(c, d)
			if modified && action.Button(c, resetLabel, action.ButtonOptions{Variant: action.Ghost, Small: true, Icon: icons.Undo, ID: "reset"}).Label(resetLabel+" "+d.Title).Clicked() {
				v.reset(d)
			}
		})
	})
}

func (v *SettingsView) menu(m *ui.Menu, d *settings.Def, modified bool) {
	if m.Item(resetSetting).Disabled(!modified).Chosen() {
		v.reset(d)
	}
	m.Separator()
	if m.Item(copySettingID).Chosen() {
		mygo.Clipboard.WriteText(d.Key)
	}
	if m.Item(copySettingJSON).Chosen() {
		v.copyJSON(d)
	}
	m.Separator()
	if m.Item(editInJSONLabel).Chosen() {
		v.openJSON()
	}
}

func (v *SettingsView) copyJSON(d *settings.Def) {
	data, err := json.Marshal(v.store().Value(d.Key))
	if err != nil {
		v.err = err.Error()
		return
	}
	mygo.Clipboard.WriteText(strconv.Quote(d.Key) + ": " + string(data))
}

func (v *SettingsView) reset(d *settings.Def) {
	v.apply(v.store().Reset(d.Key))
	delete(v.drafts, d.Key)
}

func (v *SettingsView) control(c *ui.Context, d *settings.Def) {
	s := v.store()
	switch d.Kind {
	case settings.Bool:
		on := s.Bool(d.Key)
		if action.Switch(c, "", &on, action.ChoiceOptions{ID: "control"}).Label(d.Title).Changed() {
			v.apply(s.Set(d.Key, on))
		}
	case settings.Enum:
		sel := s.String(d.Key)
		opts := make([]action.Option[string], len(d.Options))
		for i, o := range d.Options {
			opts[i] = action.Option[string]{Value: o, Label: o}
		}
		if action.Select(c, d.Title, &sel, opts, action.SelectOptions{ID: "control", Width: token.SettingsControl}).Changed() {
			v.apply(s.Set(d.Key, sel))
		}
	case settings.JSON:
		if action.Button(c, editInJSONLabel, action.ButtonOptions{ID: "control"}).Label(editInJSONLabel + ": " + d.Title).Clicked() {
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
	ui.Box(c).Width(token.SettingsControl.Px()).Shrink(0).Children(func() {
		input := field.TextInput(c, d.Title, &draft, field.InputOptions{ID: "control"})
		if input.Focused() {
			v.drafts[d.Key] = draft
		}
		if !input.Submitted() && !input.FocusLeft() {
			return
		}
		delete(v.drafts, d.Key)
		value, err := parse(d, draft)
		if err != nil {
			v.err = err.Error()
			return
		}
		if format(v.store().Value(d.Key)) != draft {
			v.apply(v.store().Set(d.Key, value))
		}
	})
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
	switch d.Kind {
	case settings.Int:
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a whole number", d.Title, text)
		}
		return n, inRange(d, float64(n))
	case settings.Number:
		n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a number", d.Title, text)
		}
		return n, inRange(d, n)
	}
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

func inRange(d *settings.Def, n float64) error {
	if d.Min == 0 && d.Max == 0 || n >= d.Min && n <= d.Max {
		return nil
	}
	return fmt.Errorf("%s: use a value from %v to %v", d.Title, d.Min, d.Max)
}
