package vscodeimport

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/keymap"
)

type Section struct {
	Title string
	Note  string
	Rows  []string
}

var skipSections = []struct {
	kind  SkipKind
	title string
	note  string
}{
	{SkipCommand, "Commands this editor does not have", "Each command is listed once with the number of keybindings that use it."},
	{SkipArguments, "Commands with arguments this editor does not support", ""},
	{SkipKey, "Keys this editor cannot read", ""},
	{SkipWhen, "When clauses this editor cannot read", ""},
	{SkipIncomplete, "Entries without a key or a command", ""},
}

func (r Report) Sections() []Section {
	var out []Section
	if len(r.Plan.Inactive) > 0 {
		out = append(out, Section{
			Title: "Imported with unset context keys",
			Note:  "These use context keys that this editor does not set. As in VS Code, a key that is not set is false.",
			Rows:  entryRows(r.Plan.Inactive),
		})
	}
	for _, s := range skipSections {
		var skipped []Skipped
		for _, sk := range r.Plan.Skipped {
			if sk.Kind == s.kind {
				skipped = append(skipped, sk)
			}
		}
		if len(skipped) == 0 {
			continue
		}
		rows := skippedRows(skipped)
		if s.kind == SkipCommand {
			rows = commandRows(skipped)
		}
		out = append(out, Section{Title: s.title, Note: s.note, Rows: rows})
	}
	return out
}

func entryRows(entries []keymap.Entry) []string {
	rows := make([]string, len(entries))
	for i, e := range entries {
		rows[i] = entryText(e) + ": when " + e.When
	}
	return rows
}

func skippedRows(skipped []Skipped) []string {
	rows := make([]string, len(skipped))
	for i, sk := range skipped {
		rows[i] = entryText(sk.Entry) + ": " + sk.Reason
	}
	return rows
}

func commandRows(skipped []Skipped) []string {
	counts := map[string]int{}
	for _, sk := range skipped {
		counts[strings.TrimPrefix(sk.Entry.Command, keymap.RemovePrefix)]++
	}
	ids := make([]string, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b string) int { return cmp.Compare(a, b) })
	rows := make([]string, len(ids))
	for i, id := range ids {
		rows[i] = fmt.Sprintf("%s: %d", id, counts[id])
	}
	return rows
}

func entryText(e keymap.Entry) string {
	return strings.TrimSpace(e.Key + " " + e.Command)
}
