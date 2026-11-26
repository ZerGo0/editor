package keymap

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestParseChord(t *testing.T) {
	tests := []struct {
		in   string
		want Chord
	}{
		{"cmd+p", Chord{{ui.Super, ui.KeyP}}},
		{"Meta+Shift+P", Chord{{ui.Super | ui.Shift, ui.KeyP}}},
		{"Option+Esc", Chord{{ui.Alt, ui.KeyEscape}}},
		{"cmd+k cmd+s", Chord{{ui.Super, ui.KeyK}, {ui.Super, ui.KeyS}}},
		{"  ctrl+`  ", Chord{{ui.Ctrl, ui.KeyBackquote}}},
		{"cmd+\\", Chord{{ui.Super, ui.KeyBackslash}}},
		{"F12", Chord{{0, ui.KeyF12}}},
		{"cmd+1", Chord{{ui.Super, ui.Key1}}},
		{"win+p", Chord{{ui.Super, ui.KeyP}}},
		{"back", Chord{{0, ui.KeyBack}}},
		{"forward", Chord{{0, ui.KeyForward}}},
		{"BrowserBack", Chord{{0, ui.KeyBack}}},
	}
	for _, tt := range tests {
		got, err := ParseChord(tt.in)
		if err != nil {
			t.Errorf("ParseChord(%q) error: %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("ParseChord(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseChordInvalid(t *testing.T) {
	for _, in := range []string{
		"", "   ", "cmd", "cmd+", "+p", "cmd++p", "foo+p", "cmd+foo", "cmd+cmd+p",
		"cmd+meta+p", "a b c", "f13", "hyper+a",
	} {
		if got, err := ParseChord(in); err == nil {
			t.Errorf("ParseChord(%q) = %v, want error", in, got)
		}
	}
}

func TestChordRoundTrip(t *testing.T) {
	for _, in := range []string{
		"cmd+k cmd+s", "ctrl+shift+alt+cmd+p", "escape", "f1", "ctrl+pageup",
		"cmd+[", "cmd+]", "-", "=", ",", ".", "/", ";", "'", "`", "\\",
		"enter", "tab", "space", "backspace", "delete", "insert", "home", "end",
		"left", "right", "up", "down", "0", "9", "z", "browserback", "browserforward",
	} {
		chord, err := ParseChord(in)
		if err != nil {
			t.Fatalf("ParseChord(%q) error: %v", in, err)
		}
		if got := chord.String(); got != in {
			t.Errorf("String() = %q, want %q", got, in)
		}
		again, err := ParseChord(chord.String())
		if err != nil || !again.Equal(chord) {
			t.Errorf("round trip of %q gave %v, %v", in, again, err)
		}
	}
}

func TestStringCanonicalOrder(t *testing.T) {
	chord, err := ParseChord("Cmd+Alt+Shift+Ctrl+P")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := chord.String(), "ctrl+shift+alt+cmd+p"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLabel(t *testing.T) {
	tests := []struct{ in, want string }{
		{"cmd+k cmd+s", "⌘K ⌘S"},
		{"cmd+shift+p", "⇧⌘P"},
		{"ctrl+alt+shift+cmd+a", "⌃⌥⇧⌘A"},
		{"enter", "↩"},
		{"escape", "⎋"},
		{"tab", "⇥"},
		{"space", "␣"},
		{"backspace", "⌫"},
		{"delete", "⌦"},
		{"home", "\u2196"},
		{"end", "\u2198"},
		{"pageup", "⇞"},
		{"pagedown", "⇟"},
		{"left", "\u2190"},
		{"right", "\u2192"},
		{"up", "\u2191"},
		{"down", "\u2193"},
		{"cmd+f5", "⌘F5"},
		{"cmd+/", "⌘/"},
		{"cmd+1", "⌘1"},
	}
	for _, tt := range tests {
		chord, err := ParseChord(tt.in)
		if err != nil {
			t.Fatalf("ParseChord(%q) error: %v", tt.in, err)
		}
		if got := chord.Label(); got != tt.want {
			t.Errorf("Label(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAccelerator(t *testing.T) {
	tests := []struct{ in, want string }{
		{"cmd+shift+p", "Shift+Cmd+P"},
		{"ctrl+shift+alt+cmd+a", "Ctrl+Shift+Alt+Cmd+A"},
		{"cmd+1", "Cmd+1"},
		{"cmd+,", "Cmd+,"},
		{"cmd+/", "Cmd+/"},
		{"cmd+\\", "Cmd+\\"},
		{"cmd+`", "Cmd+`"},
		{"cmd+[", "Cmd+["},
		{"cmd+left", "Cmd+Left"},
		{"alt+down", "Alt+Down"},
		{"f9", "F9"},
		{"cmd+f12", "Cmd+F12"},
		{"cmd+pageup", "Cmd+PageUp"},
		{"escape", "Escape"},
		{"enter", "Enter"},
	}
	for _, tt := range tests {
		chord, err := ParseChord(tt.in)
		if err != nil {
			t.Fatalf("ParseChord(%q) error: %v", tt.in, err)
		}
		if got := chord[0].Accelerator(); got != tt.want {
			t.Errorf("Accelerator(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if got := (Stroke{Mods: ui.Super, Key: ui.KeyUnknown}).Accelerator(); got != "" {
		t.Errorf("Accelerator of unknown key = %q, want empty", got)
	}
}

func TestEqualAndHasPrefix(t *testing.T) {
	single := mustParse(t, "cmd+k")
	double := mustParse(t, "cmd+k cmd+s")
	other := mustParse(t, "cmd+j cmd+s")

	if !double.Equal(mustParse(t, "cmd+k cmd+s")) || double.Equal(single) || double.Equal(other) {
		t.Error("Equal gave a wrong result")
	}
	if !double.HasPrefix(single) {
		t.Error("cmd+k should be a prefix of cmd+k cmd+s")
	}
	if double.HasPrefix(double) || single.HasPrefix(double) || single.HasPrefix(single) {
		t.Error("HasPrefix must be strict")
	}
	if other.HasPrefix(single) {
		t.Error("cmd+k must not prefix cmd+j cmd+s")
	}
}

func mustParse(t *testing.T, s string) Chord {
	t.Helper()
	chord, err := ParseChord(s)
	if err != nil {
		t.Fatalf("ParseChord(%q) error: %v", s, err)
	}
	return chord
}

func TestStrokeForFallsBackToPhysicalKey(t *testing.T) {
	tests := []struct {
		name string
		ev   ui.InputEvent
		want Stroke
	}{
		{"cyrillic s with cmd", ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUnknown, PhysicalKey: ui.KeyS, Mods: ui.Super}, Stroke{ui.Super, ui.KeyS}},
		{"greek p with cmd and shift", ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUnknown, PhysicalKey: ui.KeyP, Mods: ui.Super | ui.Shift}, Stroke{ui.Super | ui.Shift, ui.KeyP}},
		{"hebrew c with ctrl", ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUnknown, PhysicalKey: ui.KeyC, Mods: ui.Ctrl}, Stroke{ui.Ctrl, ui.KeyC}},
		{"arabic k with alt", ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUnknown, PhysicalKey: ui.KeyK, Mods: ui.Alt}, Stroke{ui.Alt, ui.KeyK}},
		{"typing without modifiers stays text", ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUnknown, PhysicalKey: ui.KeyS}, Stroke{0, ui.KeyUnknown}},
		{"shift alone stays text", ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUnknown, PhysicalKey: ui.KeyS, Mods: ui.Shift}, Stroke{ui.Shift, ui.KeyUnknown}},
		{"latin layout keeps the typed key", ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyZ, PhysicalKey: ui.KeyY, Mods: ui.Super}, Stroke{ui.Super, ui.KeyZ}},
		{"no physical key reported", ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUnknown, Mods: ui.Super}, Stroke{ui.Super, ui.KeyUnknown}},
	}
	for _, tt := range tests {
		if got := StrokeFor(tt.ev); got != tt.want {
			t.Errorf("%s: StrokeFor = %v, want %v", tt.name, got, tt.want)
		}
	}
	km := New(Defaults(PresetVSCode), nil)
	b, ok := km.Lookup(Chord{StrokeFor(ui.InputEvent{Key: ui.KeyUnknown, PhysicalKey: ui.KeyS, Mods: ui.Super})})
	if !ok || b.Command != "workbench.action.files.save" {
		t.Fatalf("Cmd+ы = %v %v, want Save", b, ok)
	}
}

func TestKeyCodeDispatchUsesThePhysicalKey(t *testing.T) {
	azerty := ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyZ, PhysicalKey: ui.KeyW, Mods: ui.Super}
	if got, want := DispatchKeyCode.Stroke(azerty), (Stroke{ui.Super, ui.KeyW}); got != want {
		t.Errorf("keyCode: Stroke = %v, want %v", got, want)
	}
	if got, want := DispatchCode.Stroke(azerty), (Stroke{ui.Super, ui.KeyZ}); got != want {
		t.Errorf("code: Stroke = %v, want %v", got, want)
	}
	plain := ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyZ, PhysicalKey: ui.KeyW}
	if got, want := DispatchKeyCode.Stroke(plain), (Stroke{0, ui.KeyZ}); got != want {
		t.Errorf("keyCode without modifiers: Stroke = %v, want %v", got, want)
	}
}

func TestBrowserKeysHaveNoAccelerator(t *testing.T) {
	for _, key := range []string{"browserback", "browserforward"} {
		chord, err := ParseChord(key)
		if err != nil {
			t.Fatal(err)
		}
		if got := chord[0].Accelerator(); got != "" {
			t.Errorf("%s accelerator = %q, want none", key, got)
		}
	}
}
