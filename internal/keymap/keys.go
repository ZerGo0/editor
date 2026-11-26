package keymap

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	maxStrokes   = 2
	strokeSep    = " "
	modifierSep  = "+"
	letterCount  = 26
	digitCount   = 10
	functionKeys = 12
	shortcutMods = ui.Ctrl | ui.Alt | ui.Super
)

type Stroke struct {
	Mods ui.Modifiers
	Key  ui.Key
}

type Chord []Stroke

type keyInfo struct {
	name  string
	label string
	accel string
}

type modifierInfo struct {
	mod   ui.Modifiers
	name  string
	glyph string
	accel string
}

var stringModifiers = []modifierInfo{
	{ui.Ctrl, "ctrl", "⌃", "Ctrl"},
	{ui.Shift, "shift", "⇧", "Shift"},
	{ui.Alt, "alt", "⌥", "Alt"},
	{ui.Super, "cmd", "⌘", "Cmd"},
}

var labelModifierOrder = []ui.Modifiers{ui.Ctrl, ui.Alt, ui.Shift, ui.Super}

var modifierAliases = map[string]ui.Modifiers{
	"cmd":    ui.Super,
	"meta":   ui.Super,
	"ctrl":   ui.Ctrl,
	"alt":    ui.Alt,
	"option": ui.Alt,
	"shift":  ui.Shift,
	"win":    ui.Super,
}

var keyNameAliases = map[string]string{
	"esc":     "escape",
	"back":    "browserback",
	"forward": "browserforward",
}

var (
	keyInfos   = buildKeyInfos()
	keysByName = buildKeysByName()
)

func buildKeyInfos() map[ui.Key]keyInfo {
	letters := [letterCount]ui.Key{
		ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG, ui.KeyH, ui.KeyI,
		ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM, ui.KeyN, ui.KeyO, ui.KeyP, ui.KeyQ, ui.KeyR,
		ui.KeyS, ui.KeyT, ui.KeyU, ui.KeyV, ui.KeyW, ui.KeyX, ui.KeyY, ui.KeyZ,
	}
	digits := [digitCount]ui.Key{
		ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9,
	}
	functions := [functionKeys]ui.Key{
		ui.KeyF1, ui.KeyF2, ui.KeyF3, ui.KeyF4, ui.KeyF5, ui.KeyF6,
		ui.KeyF7, ui.KeyF8, ui.KeyF9, ui.KeyF10, ui.KeyF11, ui.KeyF12,
	}
	infos := map[ui.Key]keyInfo{
		ui.KeyEnter:        {"enter", "↩", "Enter"},
		ui.KeyEscape:       {"escape", "⎋", "Escape"},
		ui.KeyTab:          {"tab", "⇥", "Tab"},
		ui.KeySpace:        {"space", "␣", "Space"},
		ui.KeyBackspace:    {"backspace", "⌫", "Backspace"},
		ui.KeyDelete:       {"delete", "⌦", "Delete"},
		ui.KeyInsert:       {"insert", "Insert", "Insert"},
		ui.KeyHome:         {"home", "\u2196", "Home"},
		ui.KeyEnd:          {"end", "\u2198", "End"},
		ui.KeyPageUp:       {"pageup", "⇞", "PageUp"},
		ui.KeyPageDown:     {"pagedown", "⇟", "PageDown"},
		ui.KeyLeft:         {"left", "\u2190", "Left"},
		ui.KeyRight:        {"right", "\u2192", "Right"},
		ui.KeyUp:           {"up", "\u2191", "Up"},
		ui.KeyDown:         {"down", "\u2193", "Down"},
		ui.KeyMinus:        {"-", "-", "-"},
		ui.KeyEqual:        {"=", "=", "="},
		ui.KeyComma:        {",", ",", ","},
		ui.KeyPeriod:       {".", ".", "."},
		ui.KeySlash:        {"/", "/", "/"},
		ui.KeySemicolon:    {";", ";", ";"},
		ui.KeyQuote:        {"'", "'", "'"},
		ui.KeyBracketLeft:  {"[", "[", "["},
		ui.KeyBracketRight: {"]", "]", "]"},
		ui.KeyBackslash:    {"\\", "\\", "\\"},
		ui.KeyBackquote:    {"`", "`", "`"},
		ui.KeyBack:         {"browserback", "Back", ""},
		ui.KeyForward:      {"browserforward", "Forward", ""},
	}
	for i, key := range letters {
		name := string(rune('a' + i))
		infos[key] = keyInfo{name, strings.ToUpper(name), strings.ToUpper(name)}
	}
	for i, key := range digits {
		name := fmt.Sprint(i)
		infos[key] = keyInfo{name, name, name}
	}
	for i, key := range functions {
		infos[key] = keyInfo{
			fmt.Sprintf("f%d", i+1),
			fmt.Sprintf("F%d", i+1),
			fmt.Sprintf("F%d", i+1),
		}
	}
	return infos
}

func buildKeysByName() map[string]ui.Key {
	byName := make(map[string]ui.Key, len(keyInfos))
	for key, info := range keyInfos {
		byName[info.name] = key
	}
	return byName
}

func ParseChord(s string) (Chord, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil, errors.New("keymap: empty chord")
	}
	if len(fields) > maxStrokes {
		return nil, fmt.Errorf("keymap: chord %q has more than %d strokes", s, maxStrokes)
	}
	chord := make(Chord, 0, len(fields))
	for _, field := range fields {
		stroke, err := parseStroke(field)
		if err != nil {
			return nil, err
		}
		chord = append(chord, stroke)
	}
	return chord, nil
}

func parseStroke(s string) (Stroke, error) {
	parts := strings.Split(strings.ToLower(s), modifierSep)
	keyName := parts[len(parts)-1]
	if alias, ok := keyNameAliases[keyName]; ok {
		keyName = alias
	}
	key, ok := keysByName[keyName]
	if !ok {
		return Stroke{}, fmt.Errorf("keymap: unknown key %q in %q", parts[len(parts)-1], s)
	}
	var mods ui.Modifiers
	for _, name := range parts[:len(parts)-1] {
		mod, ok := modifierAliases[name]
		if !ok {
			return Stroke{}, fmt.Errorf("keymap: unknown modifier %q in %q", name, s)
		}
		if mods&mod != 0 {
			return Stroke{}, fmt.Errorf("keymap: duplicate modifier %q in %q", name, s)
		}
		mods |= mod
	}
	return Stroke{Mods: mods, Key: key}, nil
}

func StrokeFor(ev ui.InputEvent) Stroke {
	return DispatchCode.Stroke(ev)
}

func (d Dispatch) Stroke(ev ui.InputEvent) Stroke {
	key := ev.Key
	_, known := keyInfos[key]
	byPosition := ev.Mods&shortcutMods != 0 && ev.PhysicalKey != ui.KeyUnknown
	if byPosition && (!known || d == DispatchKeyCode) {
		key = ev.PhysicalKey
	}
	return Stroke{Mods: ev.Mods, Key: key}
}

func (s Stroke) String() string {
	info, ok := keyInfos[s.Key]
	if !ok {
		return ""
	}
	var parts []string
	for _, m := range stringModifiers {
		if s.Mods&m.mod != 0 {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, info.name), modifierSep)
}

func (s Stroke) label() string {
	info, ok := keyInfos[s.Key]
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, mod := range labelModifierOrder {
		if s.Mods&mod != 0 {
			b.WriteString(modifierByMod(mod).glyph)
		}
	}
	b.WriteString(info.label)
	return b.String()
}

func modifierByMod(mod ui.Modifiers) modifierInfo {
	for _, m := range stringModifiers {
		if m.mod == mod {
			return m
		}
	}
	return modifierInfo{}
}

func (s Stroke) Accelerator() string {
	info, ok := keyInfos[s.Key]
	if !ok {
		return ""
	}
	var parts []string
	for _, m := range stringModifiers {
		if s.Mods&m.mod != 0 {
			parts = append(parts, m.accel)
		}
	}
	return strings.Join(append(parts, info.accel), modifierSep)
}

func (c Chord) String() string {
	return c.join(Stroke.String)
}

func (c Chord) Label() string {
	return c.join(Stroke.label)
}

func (c Chord) join(format func(Stroke) string) string {
	parts := make([]string, len(c))
	for i, stroke := range c {
		parts[i] = format(stroke)
	}
	return strings.Join(parts, strokeSep)
}

func (c Chord) Equal(o Chord) bool {
	return slices.Equal(c, o)
}

func (c Chord) HasPrefix(p Chord) bool {
	return len(p) < len(c) && slices.Equal(c[:len(p)], p)
}
