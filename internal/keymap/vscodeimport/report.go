package vscodeimport

import (
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
	{SkipCommand, "Commands this editor does not have", ""},
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
	if len(r.Plan.Partial) > 0 {
		out = append(out, Section{
			Title: "Imported without some commands",
			Note:  "These keys run the commands this editor has and skip the others.",
			Rows:  partialRows(r.Plan.Partial),
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
		out = append(out, Section{Title: s.title, Note: s.note, Rows: skippedRows(skipped)})
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

func partialRows(partial []Partial) []string {
	rows := make([]string, len(partial))
	for i, p := range partial {
		rows[i] = entryText(p.Entry) + ": skips " + strings.Join(p.Missing, ", ")
	}
	return rows
}

func entryText(e keymap.Entry) string {
	return strings.TrimSpace(e.Key + " " + e.Command)
}
