package keymap

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/jsonc"
	"github.com/ZerGo0/editor/internal/keymap/when"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui/editor"
)

const (
	RemovePrefix = "-"
	filePerm     = 0o644
)

type Entry struct {
	Key     string          `json:"key"`
	Command string          `json:"command"`
	When    string          `json:"when,omitempty"`
	Args    json.RawMessage `json:"args,omitempty"`
}

type Binding struct {
	Chord   Chord
	Command string
	When    *when.Clause
	User    bool
}

type chordKey [maxStrokes]Stroke

type Keymap struct {
	*Active
	bindings    []Binding
	ctx         when.Context
	byCommand   map[string][]Chord
	editor      []editor.Binding
	editorReady bool
	Errors      []jsonc.Problem
}

type Active struct {
	bindings    []Binding
	owners      map[chordKey]int
	singles     []Binding
	prefixes    []Stroke
	completions map[Stroke][]Stroke
}

func keyOf(c Chord) chordKey {
	var k chordKey
	copy(k[:], c)
	return k
}

func newBinding(e Entry) (Binding, error) {
	chord, err := ParseChord(e.Key)
	if err != nil {
		return Binding{}, err
	}
	if e.Command == "" {
		return Binding{}, fmt.Errorf("keymap: %s needs a command", e.Key)
	}
	command, err := ResolveCommand(e.Command, e.Args)
	if err != nil {
		return Binding{}, err
	}
	clause, err := when.Parse(e.When)
	if err != nil {
		return Binding{}, err
	}
	return Binding{Chord: chord, Command: command, When: clause}, nil
}

func New(defaults []Binding, user []Entry) *Keymap {
	return newLocated(defaults, user, nil)
}

func newLocated(defaults []Binding, user []Entry, locate func(int) (int, int)) *Keymap {
	k := &Keymap{ctx: DefaultContext()}
	fail := func(i int, format string, args ...any) {
		p := jsonc.Unlocated(fmt.Sprintf(format, args...))
		if locate != nil {
			p.Line, p.Col = locate(i)
		}
		k.Errors = append(k.Errors, p)
	}
	base := slices.Clone(defaults)
	var added []Binding
	for i, e := range user {
		if id, ok := strings.CutPrefix(e.Command, RemovePrefix); ok {
			if err := removeDefaults(&base, id, e); err != nil {
				fail(i, "entry %d: %v", i+1, err)
			}
			continue
		}
		if e.Key == "" || e.Command == "" {
			fail(i, "entry %d: needs a key and a command", i+1)
			continue
		}
		b, err := newBinding(e)
		if err != nil {
			fail(i, "entry %d: %v", i+1, err)
			continue
		}
		b.User = true
		added = append(added, b)
	}
	k.bindings = append(base, added...)
	k.index()
	return k
}

func removeDefaults(base *[]Binding, id string, e Entry) error {
	var chord Chord
	if e.Key != "" {
		c, err := ParseChord(e.Key)
		if err != nil {
			return err
		}
		chord = c
	}
	if _, err := when.Parse(e.When); err != nil {
		return err
	}
	id = CanonicalID(id)
	*base = slices.DeleteFunc(*base, func(b Binding) bool {
		return b.Command == id && (chord == nil || b.Chord.Equal(chord)) &&
			(e.When == "" || when.Normalize(b.When.String()) == when.Normalize(e.When))
	})
	return nil
}

func (k *Keymap) index() {
	k.Active = k.In(k.ctx)
	labels := k.In(LabelContext())
	k.byCommand = map[string][]Chord{}
	for i := len(k.bindings) - 1; i >= 0; i-- {
		if b := k.bindings[i]; labels.owner(b.Chord) == i {
			k.byCommand[b.Command] = append(k.byCommand[b.Command], b.Chord)
		}
	}
	for i := len(k.bindings) - 1; i >= 0; i-- {
		b := k.bindings[i]
		if _, labeled := k.byCommand[b.Command]; !labeled && !k.overridden(i) && !otherPlatform(b) {
			k.byCommand[b.Command] = []Chord{b.Chord}
		}
	}
}

var otherPlatformKeys = []string{"isWeb", "isWindows", "isLinux"}

func otherPlatform(b Binding) bool {
	return slices.ContainsFunc(b.When.Keys(), func(key string) bool { return slices.Contains(otherPlatformKeys, key) })
}

func (k *Keymap) overridden(i int) bool {
	return slices.ContainsFunc(k.bindings[i+1:], func(b Binding) bool { return b.When == nil && b.Chord.Equal(k.bindings[i].Chord) })
}

func (k *Keymap) In(ctx when.Context) *Active {
	a := &Active{bindings: k.bindings, owners: map[chordKey]int{}, completions: map[Stroke][]Stroke{}}
	for i, b := range k.bindings {
		if b.When.Eval(ctx) {
			a.owners[keyOf(b.Chord)] = i
		}
	}
	for i, b := range k.bindings {
		if a.owner(b.Chord) != i {
			continue
		}
		switch len(b.Chord) {
		case 1:
			a.singles = append(a.singles, b)
		default:
			first, second := b.Chord[0], b.Chord[1]
			if !slices.Contains(a.prefixes, first) {
				a.prefixes = append(a.prefixes, first)
			}
			if !slices.Contains(a.completions[first], second) {
				a.completions[first] = append(a.completions[first], second)
			}
		}
	}
	return a
}

func (a *Active) owner(c Chord) int {
	if i, ok := a.owners[keyOf(c)]; ok && len(c) <= maxStrokes {
		return i
	}
	return -1
}

func (k *Keymap) Bindings() []Binding {
	return k.bindings
}

func (k *Keymap) For(command string) []Chord {
	return k.byCommand[CanonicalID(command)]
}

func (k *Keymap) Label(command string) string {
	if chords := k.For(command); len(chords) > 0 {
		return chords[0].Label()
	}
	if CanonicalID(command) == SearchEverywhereCommand {
		return doubleShiftLabel
	}
	return ""
}

func (a *Active) Lookup(c Chord) (Binding, bool) {
	if i := a.owner(c); i >= 0 {
		return a.bindings[i], true
	}
	return Binding{}, false
}

func (k *Keymap) Resolve(c Chord, ctx when.Context) (Binding, bool) {
	for i := len(k.bindings) - 1; i >= 0; i-- {
		if b := k.bindings[i]; b.Chord.Equal(c) && b.When.Eval(ctx) {
			return b, true
		}
	}
	return Binding{}, false
}

func (a *Active) Prefixes() []Stroke {
	return a.prefixes
}

func (a *Active) Completions(first Stroke) []Stroke {
	return a.completions[first]
}

func (a *Active) Singles() []Binding {
	return a.singles
}

func (k *Keymap) EditorBindings() []editor.Binding {
	if !k.editorReady {
		k.editor, k.editorReady = k.editorBindings(), true
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
	var strokes []Stroke
	for _, b := range editor.DefaultBindings() {
		s := Stroke{Mods: b.Mods, Key: b.Key}
		if b.ThenKey == 0 {
			defaults[s] = b.Command
		}
		strokes = append(strokes, s)
	}
	for _, b := range k.Singles() {
		if !b.Chord[0].ByCode() {
			strokes = append(strokes, b.Chord[0])
		}
	}
	var out []editor.Binding
	seen := map[Stroke]bool{}
	for _, s := range strokes {
		if seen[s] {
			continue
		}
		seen[s] = true
		want := editor.NoCommand
		if b, ok := k.Lookup(Chord{s}); ok {
			if cmd, isEditor := EditorCommand(b.Command); isEditor {
				want = cmd
			}
		}
		if have, ok := defaults[s]; !ok && want != editor.NoCommand || ok && have != want || !ok && k.isEditorPrefix(s) {
			out = append(out, editor.Binding{Mods: s.Mods, Key: s.Key, Command: want})
		}
	}
	return out
}

func (k *Keymap) isEditorPrefix(s Stroke) bool {
	for _, b := range editor.DefaultBindings() {
		if b.ThenKey != 0 && b.Mods == s.Mods && b.Key == s.Key {
			return true
		}
	}
	return false
}

type Store struct {
	Path     string
	Settings *settings.Store
	Preset   string
	Dispatch Dispatch
	User     []Entry
	Current  *Keymap
}

func Load(path string) *Store {
	s := &Store{Path: path}
	s.Reload()
	return s
}

func (s *Store) Reload() {
	s.Preset, s.Dispatch = PresetVSCode, DispatchCode
	if s.Settings != nil {
		s.Preset, s.Dispatch = s.Settings.String(SettingKeymap), Dispatch(s.Settings.String(SettingDispatch))
	}
	user, data, err := readUser(s.Path)
	s.User = user
	s.Current = newLocated(Defaults(s.Preset), user, entryLocator(data))
	if err != nil {
		p, ok := err.(jsonc.Problem)
		if !ok {
			p = jsonc.Unlocated(err.Error())
		}
		s.Current.Errors = append([]jsonc.Problem{p}, s.Current.Errors...)
	}
}

func readUser(path string) ([]Entry, []byte, error) {
	data, err := jsonc.ReadFile(path)
	if err != nil || data == nil {
		return nil, nil, err
	}
	entries, err := ParseEntries(data, path)
	return entries, data, err
}

func ParseEntries(data []byte, name string) ([]Entry, error) {
	var entries []Entry
	if err := json.Unmarshal(jsonc.Standard(data), &entries); err != nil {
		return nil, describeEntries(data, name, err)
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
