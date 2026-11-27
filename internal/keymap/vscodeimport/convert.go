package vscodeimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/keymap/when"
)

type SkipKind int

const (
	SkipCommand SkipKind = iota
	SkipArguments
	SkipKey
	SkipWhen
	SkipIncomplete
)

type Skipped struct {
	Entry  keymap.Entry
	Kind   SkipKind
	Reason string
}

type Plan struct {
	Add        []keymap.Entry
	Inactive   []keymap.Entry
	Skipped    []Skipped
	Duplicates int
}

type entryKey struct {
	key, command, when, args string
}

func Convert(entries, ours []keymap.Entry, known func(id string) bool) Plan {
	var p Plan
	seen := map[entryKey]bool{}
	for _, e := range ours {
		if k, err := keyOf(e); err == nil {
			seen[k] = true
		}
	}
	ctx := keymap.DefaultContext()
	for _, e := range entries {
		clause, skip := check(e, known)
		if skip != nil {
			p.Skipped = append(p.Skipped, *skip)
			continue
		}
		k, err := keyOf(e)
		if err != nil {
			p.Skipped = append(p.Skipped, Skipped{Entry: e, Kind: SkipArguments, Reason: err.Error()})
			continue
		}
		if seen[k] {
			p.Duplicates++
			continue
		}
		seen[k] = true
		p.Add = append(p.Add, e)
		if len(clause.Unknown(ctx)) > 0 {
			p.Inactive = append(p.Inactive, e)
		}
	}
	return p
}

func check(e keymap.Entry, known func(string) bool) (*when.Clause, *Skipped) {
	skip := func(kind SkipKind, reason string) (*when.Clause, *Skipped) {
		return nil, &Skipped{Entry: e, Kind: kind, Reason: reason}
	}
	id, removal := strings.CutPrefix(e.Command, keymap.RemovePrefix)
	if id == "" {
		return skip(SkipIncomplete, "no command")
	}
	if e.Key == "" && !removal {
		return skip(SkipIncomplete, "no key")
	}
	if e.Key != "" {
		if _, err := keymap.ParseChord(e.Key); err != nil {
			return skip(SkipKey, "unsupported key "+e.Key)
		}
	}
	clause, err := when.Parse(e.When)
	if err != nil {
		return skip(SkipWhen, "unsupported when clause: "+err.Error())
	}
	resolved := keymap.CanonicalID(id)
	if !removal {
		if resolved, err = keymap.ResolveCommand(id, e.Args); err != nil {
			return skip(SkipArguments, err.Error())
		}
	}
	if !known(resolved) {
		return skip(SkipCommand, "unsupported command "+id)
	}
	return clause, nil
}

func keyOf(e keymap.Entry) (entryKey, error) {
	k := entryKey{command: e.Command, when: when.Normalize(e.When)}
	if e.Key != "" {
		chord, err := keymap.ParseChord(e.Key)
		if err != nil {
			return entryKey{}, err
		}
		k.key = chord.String()
	}
	if len(bytes.TrimSpace(e.Args)) > 0 {
		var buf bytes.Buffer
		if err := json.Compact(&buf, e.Args); err != nil {
			return entryKey{}, fmt.Errorf("invalid args: %w", err)
		}
		k.args = buf.String()
	}
	return k, nil
}
