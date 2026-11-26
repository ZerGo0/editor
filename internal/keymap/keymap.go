package keymap

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/jsonc"
	"github.com/egoist/mygo/ui/editor"
)

const (
	EditorPrefix = "editor."
	removePrefix = "-"
	filePerm     = 0o644
)

type Entry struct {
	Key     string `json:"key"`
	Command string `json:"command"`
}

type Binding struct {
	Chord   Chord
	Command string
	User    bool
}

type Keymap struct {
	bindings     []Binding
	editor       []editor.Binding
	editorCached bool
	Errors       []string
}

func EditorCommand(id string) (editor.Command, bool) {
	name, ok := strings.CutPrefix(id, EditorPrefix)
	if !ok {
		return editor.NoCommand, false
	}
	return editor.CommandNamed(name)
}

func EditorCommandID(cmd editor.Command) string {
	return EditorPrefix + cmd.String()
}

func Defaults() []Binding {
	var out []Binding
	for _, e := range workbenchDefaults {
		chord, err := ParseChord(e.Key)
		if err != nil {
			panic("keymap: default " + e.Key + ": " + err.Error())
		}
		out = append(out, Binding{Chord: chord, Command: e.Command})
	}
	for _, b := range editor.DefaultBindings() {
		chord := Chord{{Mods: b.Mods, Key: b.Key}}
		if b.ThenKey != 0 {
			chord = append(chord, Stroke{Mods: b.ThenMods, Key: b.ThenKey})
		}
		out = append(out, Binding{Chord: chord, Command: EditorCommandID(b.Command)})
	}
	return out
}

func New(defaults []Binding, user []Entry) *Keymap {
	k := &Keymap{bindings: slices.Clone(defaults)}
	for i, e := range user {
		var chord Chord
		if e.Key != "" {
			c, err := ParseChord(e.Key)
			if err != nil {
				k.Errors = append(k.Errors, fmt.Sprintf("entry %d: %v", i+1, err))
				continue
			}
			chord = c
		}
		if id, ok := strings.CutPrefix(e.Command, removePrefix); ok {
			k.bindings = slices.DeleteFunc(k.bindings, func(b Binding) bool {
				return b.Command == id && (chord == nil || b.Chord.Equal(chord))
			})
			continue
		}
		if chord == nil || e.Command == "" {
			k.Errors = append(k.Errors, fmt.Sprintf("entry %d: needs a key and a command", i+1))
			continue
		}
		k.bindings = append(k.bindings, Binding{Chord: chord, Command: e.Command, User: true})
	}
	return k
}

func (k *Keymap) Bindings() []Binding {
	return k.bindings
}

func (k *Keymap) For(command string) []Chord {
	var out []Chord
	for i := len(k.bindings) - 1; i >= 0; i-- {
		if b := k.bindings[i]; b.Command == command && k.owner(b.Chord) == i {
			out = append(out, b.Chord)
		}
	}
	return out
}

func (k *Keymap) Lookup(c Chord) (Binding, bool) {
	if i := k.owner(c); i >= 0 {
		return k.bindings[i], true
	}
	return Binding{}, false
}

func (k *Keymap) owner(c Chord) int {
	for i := len(k.bindings) - 1; i >= 0; i-- {
		if k.bindings[i].Chord.Equal(c) {
			return i
		}
	}
	return -1
}

func (k *Keymap) Prefixes() []Stroke {
	var out []Stroke
	for _, b := range k.bindings {
		if len(b.Chord) > 1 && !slices.Contains(out, b.Chord[0]) {
			out = append(out, b.Chord[0])
		}
	}
	return out
}

func (k *Keymap) Completions(first Stroke) []Stroke {
	var out []Stroke
	for _, b := range k.bindings {
		if len(b.Chord) > 1 && b.Chord[0] == first && !slices.Contains(out, b.Chord[1]) {
			out = append(out, b.Chord[1])
		}
	}
	return out
}

func (k *Keymap) Singles() []Binding {
	var out []Binding
	for i, b := range k.bindings {
		if len(b.Chord) == 1 && k.owner(b.Chord) == i {
			out = append(out, b)
		}
	}
	return out
}

func (k *Keymap) EditorBindings() []editor.Binding {
	if !k.editorCached {
		k.editor, k.editorCached = k.editorBindings(), true
	}
	return k.editor
}

func (k *Keymap) MenuStroke(command string) (Stroke, bool) {
	for _, c := range k.For(command) {
		if len(c) == 1 && c[0].Accelerator() != "" {
			return c[0], true
		}
	}
	return Stroke{}, false
}

func (k *Keymap) editorBindings() []editor.Binding {
	defaults := map[Stroke]editor.Command{}
	for _, b := range editor.DefaultBindings() {
		if b.ThenKey == 0 {
			defaults[Stroke{Mods: b.Mods, Key: b.Key}] = b.Command
		}
	}
	var out []editor.Binding
	seen := map[Stroke]bool{}
	check := func(s Stroke) {
		if seen[s] {
			return
		}
		seen[s] = true
		want := editor.NoCommand
		if b, ok := k.Lookup(Chord{s}); ok {
			cmd, isEditor := EditorCommand(b.Command)
			if !isEditor {
				return
			}
			want = cmd
		}
		if have, ok := defaults[s]; !ok && want != editor.NoCommand || ok && have != want {
			out = append(out, editor.Binding{Mods: s.Mods, Key: s.Key, Command: want})
		}
	}
	for s := range defaults {
		check(s)
	}
	for _, b := range k.bindings {
		if len(b.Chord) == 1 {
			check(b.Chord[0])
		}
	}
	return out
}

type Store struct {
	Path    string
	User    []Entry
	Current *Keymap
}

func Load(path string) *Store {
	user, err := readUser(path)
	current := New(Defaults(), user)
	if err != nil {
		current.Errors = append([]string{err.Error()}, current.Errors...)
	}
	return &Store{Path: path, User: user, Current: current}
}

func (s *Store) Reload() {
	*s = *Load(s.Path)
}

func readUser(path string) ([]Entry, error) {
	data, err := jsonc.ReadFile(path)
	if err != nil || data == nil {
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return entries, nil
}

func (s *Store) SetUser(entries []Entry) error {
	data, err := json.MarshalIndent(entries, "", "\t")
	if err != nil {
		return err
	}
	// TECHDEBT: Rewrites the whole file, so comments in keybindings.json are lost on edits from the UI; edit the array in place with jsonc once it supports arrays.
	if err := fsutil.WriteFileAtomic(s.Path, append(data, '\n'), filePerm); err != nil {
		return err
	}
	s.Reload()
	return nil
}
