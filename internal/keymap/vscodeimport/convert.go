package vscodeimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/command"
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

const notFoundReason = "this editor has no such command"

type Skipped struct {
	Entry  keymap.Entry
	Kind   SkipKind
	Reason string
}

type Partial struct {
	Entry   keymap.Entry
	Missing []string
}

type Plan struct {
	Add        []keymap.Entry
	Inactive   []keymap.Entry
	Partial    []Partial
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
		if _, missing := runParts(e, known); len(missing) > 0 {
			p.Partial = append(p.Partial, Partial{Entry: e, Missing: missing})
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
	if _, isSequence := command.ParseSequenceID(resolved); isSequence {
		supported, missing := runParts(e, known)
		if supported == 0 {
			return skip(SkipCommand, "none of its commands exist here: "+strings.Join(missing, ", "))
		}
		return clause, nil
	}
	if !known(resolved) {
		return skip(SkipCommand, notFoundReason)
	}
	return clause, nil
}

func runParts(e keymap.Entry, known func(string) bool) (supported int, missing []string) {
	if keymap.CanonicalID(e.Command) != keymap.RunCommands {
		return 0, nil
	}
	items, err := keymap.RunCommandItems(e.Args)
	if err != nil {
		return 0, nil
	}
	for _, it := range items {
		switch {
		case it.Err != nil:
			missing = append(missing, it.ID+" ("+it.Err.Error()+")")
		case !known(it.Resolved):
			missing = append(missing, it.ID)
		default:
			supported++
		}
	}
	return supported, missing
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
