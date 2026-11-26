package prefs

import (
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo/ui"
)

const (
	keysColumnWidth   = 140
	sourceColumnWidth = 64
	actionColumnWidth = 150
	commandMinWidth   = 160
	keyRowHeight      = 32
	recorderWidth     = 420
	maxStrokes        = 2
	sourceUser        = "User"
	sourceDefault     = "Default"
)

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

func (v *KeysView) rows() []keyRow {
	km := v.Keys.Current
	q := strings.ToLower(strings.TrimSpace(v.query))
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
		match := func(keys string) bool {
			return q == "" || strings.Contains(strings.ToLower(cmd.Label()+" "+cmd.ID+" "+keys), q)
		}
		if len(bound) == 0 {
			if match("") {
				out = append(out, keyRow{cmd: cmd})
			}
			continue
		}
		for i := range bound {
			b := bound[i]
			if match(b.Chord.Label() + " " + b.Chord.String()) {
				out = append(out, keyRow{cmd: cmd, binding: &b})
			}
		}
	}
	return out
}

func (v *KeysView) View(c *ui.Context) {
	t := c.Theme()
	rows := v.rows()
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		toolbar(c, func() {
			ui.TextInput(c.Key("keys-search"), &v.query).Placeholder("Search by command or key").Label("Search keyboard shortcuts").Grow(1)
			if ui.Button(c, "Open Keyboard Shortcuts (JSON)").Clicked() {
				v.OpenJSON()
			}
		})
		problems(c, t, append(slices.Clone(v.Keys.Current.Errors), nonEmpty(v.err)...))
		ui.Row(c).Padding(6, contentPadding).Gap(12).FillWidth().Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			header(c, t, "Command").Grow(1).MinWidth(commandMinWidth)
			header(c, t, "Keybinding").Width(keysColumnWidth)
			header(c, t, "Source").Width(sourceColumnWidth)
			ui.Box(c).Width(actionColumnWidth)
		})
		ui.List(c, &v.list, len(rows), func(i int) { v.row(c, t, rows[i]) }).Grow(1).FillWidth().Padding(0, contentPadding)
	})
	v.recorderView(c, t)
}

func header(c *ui.Context, t *ui.Theme, text string) ui.Element {
	return ui.Text(c, text).FontSize(detailFontSize).Bold().TextColor(t.TextMuted).SingleLine()
}

func (v *KeysView) row(c *ui.Context, t *ui.Theme, r keyRow) {
	key := r.cmd.ID
	if r.binding != nil {
		key += " " + r.binding.Chord.String()
	}
	row := ui.Row(c.Key(key)).Height(keyRowHeight).Gap(12).AlignItems(ui.Center).FillWidth().BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	hovered := row.Hovered() || row.FocusWithin()
	if hovered {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Row(c).Gap(8).Grow(1).MinWidth(commandMinWidth).AlignItems(ui.Center).Clip().Children(func() {
			ui.Text(c, r.cmd.Label()).SingleLine().Shrink(1).MinWidth(0)
			ui.Text(c, r.cmd.ID).FontSize(detailFontSize).TextColor(t.TextMuted).SingleLine().Shrink(1).MinWidth(0)
		})
		keys, source := "", ""
		if r.binding != nil {
			keys, source = r.binding.Chord.Label(), sourceDefault
			if r.binding.User {
				source = sourceUser
			}
		}
		ui.Text(c, keys).SingleLine().Width(keysColumnWidth)
		ui.Text(c, source).FontSize(detailFontSize).TextColor(t.TextMuted).SingleLine().Width(sourceColumnWidth)
		ui.Row(c).Gap(4).Width(actionColumnWidth).Shrink(0).Justify(ui.End).Children(func() {
			if !hovered {
				return
			}
			label := "Add"
			if r.binding != nil {
				label = "Change"
			}
			if smallButton(c, t, label, label+" keybinding of "+r.cmd.Label()) {
				v.rec = recorder{open: true, command: r.cmd.ID, old: r.binding}
			}
			if r.binding != nil && smallButton(c, t, "Remove", "Remove keybinding of "+r.cmd.Label()) {
				v.remove(r.cmd.ID, r.binding)
			}
			if v.customized(r.cmd.ID) && smallButton(c, t, "Reset", "Reset keybindings of "+r.cmd.Label()) {
				v.reset(r.cmd.ID)
			}
		})
	})
	if row.DoubleClicked() {
		v.rec = recorder{open: true, command: r.cmd.ID, old: r.binding}
	}
}

func smallButton(c *ui.Context, t *ui.Theme, text, label string) bool {
	b := ui.ButtonBase(c.Key(text)).Padding(2, 8).Radius(4).Label(label)
	if b.Hovered() {
		b.Background(t.SurfacePressed)
	}
	b.Children(func() { ui.Text(c, text).FontSize(detailFontSize).TextColor(t.Accent) })
	return b.Clicked()
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

func (v *KeysView) recorderView(c *ui.Context, t *ui.Theme) {
	r := &v.rec
	if !r.open {
		return
	}
	ui.Modal(c, &r.open, func() {
		ui.Column(c).Gap(10).Width(recorderWidth).Children(func() {
			title := "Press the keys for " + v.title(r.command)
			ui.Text(c, title).Bold()
			ui.Text(c, "Press Enter to save, or Escape to cancel.").FontSize(detailFontSize).TextColor(t.TextMuted)
			box := ui.Row(c.Key("capture")).Height(keyRowHeight).FillWidth().Center().Radius(6).Background(t.Surface).
				BorderWidth(1).BorderColor(t.Focus).Focusable().AutoFocus().Label(title).Bind(&r.focus)
			box.HandleInput(func(ev ui.InputEvent) bool { return v.capture(ev) })
			box.Children(func() {
				text := r.chord.Label()
				if text == "" {
					text = "Waiting for keys..."
				}
				ui.Text(c, text)
			})
			if len(r.chord) > 0 {
				if b, ok := v.Keys.Current.Lookup(r.chord); ok && b.Command != r.command {
					ui.Text(c, "Already used by "+v.title(b.Command)+".").FontSize(detailFontSize).TextColor(t.Warning)
				}
			}
		})
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
	if ev.Kind != ui.InputKeyDown || stroke.Key == ui.KeyUnknown {
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
