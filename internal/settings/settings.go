package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/jsonc"
)

const (
	filePerm = 0o644
	dirPerm  = 0o755
)

type Store struct {
	Path   string
	Base   *Store
	values map[string]any
	set    map[string]bool
	Errors []jsonc.Problem
	Broken bool
}

func Load(path string) *Store {
	s := &Store{Path: path}
	s.Reload()
	return s
}

func LoadFolder(path string, base *Store) *Store {
	s := &Store{Path: path, Base: base}
	s.Reload()
	return s
}

func (s *Store) Reload() {
	s.values, s.set, s.Errors, s.Broken = map[string]any{}, map[string]bool{}, nil, false
	if s.Base == nil {
		for _, d := range Defs {
			s.values[d.Key] = d.Default
		}
	}
	raw, data, bad := s.read()
	if bad != nil {
		s.Broken = true
		s.Errors = append(s.Errors, *bad)
		return
	}
	offsets := jsonc.KeyOffsets(data)
	problem := func(key, text string) jsonc.Problem {
		if at, ok := offsets[key]; ok {
			return jsonc.At(data, at, text)
		}
		return jsonc.Unlocated(text)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, key := range keys {
		d := DefFor(key)
		if d == nil {
			s.Errors = append(s.Errors, problem(key, key+": unknown setting."))
			continue
		}
		if s.Base != nil && d.AppWide {
			s.Errors = append(s.Errors, problem(key, key+": applies to all windows; set it in the user settings."))
			continue
		}
		v, err := decode(d, raw[key])
		if err != nil {
			s.Errors = append(s.Errors, problem(key, fmt.Sprintf("%s: %v.", key, err)))
			continue
		}
		s.values[key], s.set[key] = v, true
	}
}

func (s *Store) read() (map[string]json.RawMessage, []byte, *jsonc.Problem) {
	data, err := jsonc.ReadFile(s.Path)
	if err != nil {
		p := jsonc.Unlocated(fmt.Sprintf("%s: %v.", filepath.Base(s.Path), err))
		return nil, nil, &p
	}
	if data == nil {
		return nil, nil, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		f := jsonc.Describe(err)
		p := jsonc.At(data, f.Offset, f.Sentence())
		return nil, nil, &p
	}
	return raw, data, nil
}

func unmarshal(raw json.RawMessage, v any) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return errors.New(jsonc.Describe(err).Message)
	}
	return nil
}

func decode(d *Def, raw json.RawMessage) (any, error) {
	switch d.Kind {
	case Bool:
		var v bool
		err := unmarshal(raw, &v)
		return v, err
	case Int:
		var v float64
		if err := unmarshal(raw, &v); err != nil {
			return nil, err
		}
		if v != math.Trunc(v) {
			return nil, errors.New("must be a whole number")
		}
		return int(clamp(d, v)), nil
	case Number:
		var v float64
		if err := unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return clamp(d, v), nil
	case String:
		var v string
		err := unmarshal(raw, &v)
		return v, err
	case Enum:
		var v string
		if err := unmarshal(raw, &v); err != nil {
			var b bool
			if unmarshal(raw, &b) != nil {
				return nil, err
			}
			v = strconv.FormatBool(b)
		}
		if !slices.Contains(d.Options, v) {
			return nil, fmt.Errorf("must be one of %s", strings.Join(d.Options, ", "))
		}
		return v, nil
	case IntList:
		var v []int
		err := unmarshal(raw, &v)
		return v, err
	case StringList:
		var v []string
		err := unmarshal(raw, &v)
		return v, err
	case JSON:
		if !json.Valid(raw) {
			return nil, errors.New("must be valid JSON")
		}
		return json.RawMessage(slices.Clone(raw)), nil
	}
	return nil, errors.New("unsupported kind")
}

func clamp(d *Def, v float64) float64 {
	if d.Min == 0 && d.Max == 0 {
		return v
	}
	return min(max(v, d.Min), d.Max)
}

func (s *Store) Value(key string) any {
	if v, ok := s.values[key]; ok {
		return v
	}
	if s.Base != nil {
		return s.Base.Value(key)
	}
	return nil
}

func (s *Store) Bool(key string) bool {
	v, _ := s.Value(key).(bool)
	return v
}

func (s *Store) Int(key string) int {
	v, _ := s.Value(key).(int)
	return v
}

func (s *Store) Number(key string) float64 {
	v, _ := s.Value(key).(float64)
	return v
}

func (s *Store) String(key string) string {
	v, _ := s.Value(key).(string)
	return v
}

func (s *Store) Ints(key string) []int {
	v, _ := s.Value(key).([]int)
	return v
}

func (s *Store) Strings(key string) []string {
	v, _ := s.Value(key).([]string)
	return v
}

func (s *Store) JSON(key string) json.RawMessage {
	v, _ := s.Value(key).(json.RawMessage)
	return v
}

func (s *Store) IsSet(key string) bool {
	return s.set[key]
}

func (s *Store) Set(key string, value any) error {
	d := DefFor(key)
	if d == nil {
		return fmt.Errorf("%s: unknown setting", key)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := decode(d, encoded); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if s.Base != nil && d.AppWide {
		return fmt.Errorf("%s: applies to all windows; set it in the user settings", key)
	}
	return s.edit(func(data []byte) ([]byte, error) { return jsonc.SetKey(data, key, encoded) })
}

func (s *Store) Reset(key string) error {
	return s.edit(func(data []byte) ([]byte, error) { return jsonc.DeleteKey(data, key) })
}

func (s *Store) edit(change func([]byte) ([]byte, error)) error {
	data, err := os.ReadFile(s.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	out, err := change(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), dirPerm); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(s.Path, out, filePerm); err != nil {
		return err
	}
	s.Reload()
	return nil
}
