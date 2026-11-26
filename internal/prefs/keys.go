package prefs

import (
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/dialog"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	maxStrokes      = 2
	sourceUser      = "User"
	sourceDefault   = "Default"
	keysLabel       = "Keyboard Shortcuts"
	keysSearchLabel = "Search keyboard shortcuts"
	keysJSONOpen    = "Open Keyboard Shortcuts (JSON)"
	recordHint      = "Press Enter to save, or Escape to cancel."
	waitingKeys     = "Waiting for keys..."
	copyCommandID   = "Copy Command ID"
	rowActionSlots  = 3
)

var actionsWidth = rowActionSlots*token.IconButtonSize.Px() + (rowActionSlots-1)*token.GapTight.Px()

type KeysView struct {
	Keys     *keymap.Store
	Commands *command.Registry
	OpenJSON func()
	query    string
	list     ui.ListState
	rec      recorder
	err      string
}

type keyRow struct {
	cmd     *command.Command
	binding *keymap.Binding
}

type recorder struct {
	open    bool
	command string
	old     *keymap.Binding
	chord   keymap.Chord
	focus   ui.Handle
}

func (v *KeysView) rows(f keyFilter) []keyRow {
	km := v.Keys.Current
	var out []keyRow
	for _, cmd := range v.Commands.All() {
		var bound []keymap.Binding
		for _, b := range km.Bindings() {
			if b.Command == cmd.ID {
				if owner, _ := km.Lookup(b.Chord); owner.Command == cmd.ID {
					bound = append(bound, b)
				}
			}
		}
		text := cmd.Label() + " " + cmd.ID
		if len(bound) == 0 {
			if f.match(text, false, false) {
				out = append(out, keyRow{cmd: cmd})
			}
			continue
		}
		for i := range bound {
			b := bound[i]
			if f.match(text+" "+b.Chord.Label()+" "+b.Chord.String(), true, b.User) {
				out = append(out, keyRow{cmd: cmd, binding: &b})
			}
		}
	}
	return out
}

func (v *KeysView) View(c *ui.Context) {
	f := parseKeyFilter(v.query)
	rows := v.rows(f)
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		toolbar(c, keysLabel, func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() {
				field.SearchInput(c, keysSearchLabel, &v.query, field.SearchOptions{Placeholder: "Search by command, ID or key, @source:user", ID: "keys-search"})
			})
			if action.Button(c, keysJSONOpen, action.ButtonOptions{Variant: action.Ghost, Icon: icons.File}).Clicked() {
				v.OpenJSON()
			}
		})
		problems(c, append(slices.Clone(v.Keys.Current.Errors), nonEmpty(v.err)...))
		v.header(c)
		l := list.View(c, &v.list, len(rows), list.ViewOptions{Label: keysLabel, Empty: func(c *ui.Context) {
			feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: "No keyboard shortcuts found", Message: "No command matches the search.",
				Action: "Clear Search", OnAction: func() { v.query = "" }})
		}}, func(i int) list.Item { return v.item(rows[i], i) })
		if sel := v.list.Selected; sel != nil && *sel >= 0 && *sel < len(rows) && l.Shortcut(0, ui.KeyEnter) {
			v.record(rows[*sel])
		}
	})
	v.recorderView(c)
}

func (v *KeysView) header(c *ui.Context) {
	p := token.Of(c)
	ui.Row(c).Height(token.HeaderHeight.Px()).FillWidth().Shrink(0).Padding(0, token.PadRow.Px()).Gap(token.GapControl.Px()).AlignItems(ui.Center).
		BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Section).Children(func() {
		design.Text(c, "Command", token.TextCaption).Grow(1).MinWidth(0)
		v.columns(c, func() { design.Text(c, "Keybinding", token.TextCaption) }, func() { design.Text(c, "When", token.TextCaption) },
			func() { design.Text(c, "Source", token.TextCaption) }, nil)
	})
}

func (v *KeysView) columns(c *ui.Context, keys, when, source, actions func()) {
	ui.Row(c).Width(token.KeyColumnWidth.Px()).Shrink(0).AlignItems(ui.Center).Clip().Children(keys)
	ui.Row(c).Width(token.WhenColumnWidth.Px()).Shrink(0).AlignItems(ui.Center).Clip().Children(when)
	ui.Row(c).Width(token.SourceColumnWidth.Px()).Shrink(0).AlignItems(ui.Center).Children(source)
	ui.Row(c).Width(actionsWidth).Shrink(0).Gap(token.GapTight.Px()).Justify(ui.End).AlignItems(ui.Center).Children(func() {
		if actions != nil {
			actions()
		}
	})
}

func (v *KeysView) item(r keyRow, i int) list.Item {
	label := r.cmd.Label()
	return list.Item{
		Tooltip:       r.cmd.ID,
		Menu:          func(m *ui.Menu) { v.menu(m, r) },
		OnDoubleClick: func() { v.record(r) },
		Content: func(c *ui.Context) {
			p := token.Of(c)
			row := ui.Row(c).Grow(1).MinWidth(0).FillHeight().Gap(token.GapControl.Px()).AlignItems(ui.Center)
			selected := v.list.Selected != nil && *v.list.Selected == i
			show := row.Hovered() || selected
			keys, when, source := "", "", ""
			if r.binding != nil {
				keys, source = r.binding.Chord.Label(), sourceDefault
				if r.binding.User {
					source = sourceUser
				}
				if r.binding.When != nil {
					when = r.binding.When.String()
				}
			}
			row.Label(label).Description(strings.Join(slices.DeleteFunc([]string{r.cmd.ID, keys, when, source}, func(s string) bool { return s == "" }), ", "))
			row.Children(func() {
				ui.RichText(c, ui.Span{Text: label + "  ", Color: p.Text.Primary}, ui.Span{Text: r.cmd.ID, Color: p.Text.Secondary}).
					FontSize(token.TextBody.Px()).SingleLine().Grow(1).Shrink(1).MinWidth(0).AccessHidden(true)
				v.columns(c, func() {
					if keys != "" {
						badge.Keys(c, keys)
					}
				}, func() {
					design.Text(c, when, token.TextMono).SingleLine().Shrink(1).MinWidth(0).AccessHidden(true)
				}, func() {
					design.Text(c, source, token.TextCaption).SingleLine().AccessHidden(true)
				}, func() {
					if show {
						v.actions(c, r)
					}
				})
			})
		},
	}
}

func (v *KeysView) actions(c *ui.Context, r keyRow) {
	label := r.cmd.Label()
	if r.binding == nil {
		if action.IconButton(c, icons.Plus, "Add keybinding of "+label, action.IconOptions{Tooltip: "Add Keybinding", ID: "add"}).Clicked() {
			v.record(r)
		}
	} else {
		if action.IconButton(c, icons.Edit, "Change keybinding of "+label, action.IconOptions{Tooltip: "Change Keybinding", ID: "change"}).Clicked() {
			v.record(r)
		}
		if action.IconButton(c, icons.Minus, "Remove keybinding of "+label, action.IconOptions{Tooltip: "Remove Keybinding", ID: "remove"}).Clicked() {
			v.remove(r.cmd.ID, r.binding)
		}
	}
	if v.customized(r.cmd.ID) && action.IconButton(c, icons.Undo, "Reset keybindings of "+label, action.IconOptions{Tooltip: "Reset Keybinding", ID: "reset"}).Clicked() {
		v.reset(r.cmd.ID)
	}
}

func (v *KeysView) menu(m *ui.Menu, r keyRow) {
	change := "Add Keybinding..."
	if r.binding != nil {
		change = "Change Keybinding..."
	}
	if m.Item(change).Chosen() {
		v.record(r)
	}
	if r.binding != nil && m.Item("Remove Keybinding").Chosen() {
		v.remove(r.cmd.ID, r.binding)
	}
	if m.Item("Reset Keybinding").Disabled(!v.customized(r.cmd.ID)).Chosen() {
		v.reset(r.cmd.ID)
	}
	m.Separator()
	if m.Item(copyCommandID).Chosen() {
		mygo.Clipboard.WriteText(r.cmd.ID)
	}
}

func (v *KeysView) record(r keyRow) {
	v.rec = recorder{open: true, command: r.cmd.ID, old: r.binding}
}

func (v *KeysView) customized(id string) bool {
	return slices.ContainsFunc(v.Keys.User, func(e keymap.Entry) bool {
		return e.Command == id || e.Command == "-"+id
	})
}

func (v *KeysView) write(entries []keymap.Entry) {
	v.err = ""
	if err := v.Keys.SetUser(entries); err != nil {
		v.err = err.Error()
	}
}

func (v *KeysView) withoutUserBinding(id string, b *keymap.Binding) []keymap.Entry {
	return slices.DeleteFunc(slices.Clone(v.Keys.User), func(e keymap.Entry) bool {
		chord, err := keymap.ParseChord(e.Key)
		return e.Command == id && err == nil && chord.Equal(b.Chord)
	})
}

func (v *KeysView) remove(id string, b *keymap.Binding) {
	if b.User {
		v.write(v.withoutUserBinding(id, b))
		return
	}
	v.write(append(slices.Clone(v.Keys.User), keymap.Entry{Key: b.Chord.String(), Command: "-" + id}))
}

func (v *KeysView) reset(id string) {
	v.write(slices.DeleteFunc(slices.Clone(v.Keys.User), func(e keymap.Entry) bool {
		return e.Command == id || e.Command == "-"+id
	}))
}

func (v *KeysView) assign(id string, old *keymap.Binding, chord keymap.Chord) {
	entries := slices.Clone(v.Keys.User)
	switch {
	case old != nil && old.User:
		entries = v.withoutUserBinding(id, old)
	case old != nil:
		entries = append(entries, keymap.Entry{Key: old.Chord.String(), Command: "-" + id})
	}
	v.write(append(entries, keymap.Entry{Key: chord.String(), Command: id}))
}

func (v *KeysView) recorderView(c *ui.Context) {
	r := &v.rec
	if !r.open {
		return
	}
	title := "Press the keys for " + v.title(r.command)
	dialog.Modal(c, &r.open, title, dialog.Options{Hint: recordHint}, func() {
		p := token.Of(c)
		box := ui.Row(c.Key("capture")).Height(token.ControlHeight.Px()).FillWidth().Center().Radius(token.RadiusControl.Px()).
			Background(p.Layer.Input).Border(token.BorderWidth.Px(), p.Accent.Focus).Focusable().AutoFocus().Label(title).Bind(&r.focus)
		box.HandleInput(func(ev ui.InputEvent) bool { return v.capture(ev) })
		box.Children(func() {
			if len(r.chord) == 0 {
				design.Text(c, waitingKeys, token.TextCaption)
				return
			}
			badge.Keys(c, r.chord.Label())
		})
		if len(r.chord) > 0 {
			if b, ok := v.Keys.Current.Lookup(r.chord); ok && b.Command != r.command {
				design.Inline(c, token.GapRowIcon, func() {
					ui.Icon(c, feedback.ToneIcon(token.ToneWarning)).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Severity.Warning)
					design.Text(c, "Already used by "+v.title(b.Command)+".", token.TextBody)
				})
			}
		}
	})
}

func (v *KeysView) title(id string) string {
	if cmd := v.Commands.Get(id); cmd != nil {
		return cmd.Label()
	}
	return id
}

func (v *KeysView) capture(ev ui.InputEvent) bool {
	r := &v.rec
	if ev.Kind == ui.InputText {
		return true
	}
	stroke := keymap.StrokeFor(ev)
	if ev.Kind != ui.InputKeyDown || stroke.Key == ui.KeyUnknown && !stroke.ByCode() {
		return false
	}
	switch {
	case stroke.Mods == 0 && stroke.Key == ui.KeyEscape:
		r.open = false
	case stroke.Mods == 0 && stroke.Key == ui.KeyEnter && len(r.chord) > 0:
		r.open = false
		v.assign(r.command, r.old, r.chord)
	default:
		if len(r.chord) == maxStrokes {
			r.chord = nil
		}
		r.chord = append(r.chord, stroke)
	}
	return true
}
