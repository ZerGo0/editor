package qa

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

const (
	RuleTitleCase     = "title-case"
	RuleEllipsis      = "ellipsis"
	RuleDoubleSpace   = "double-space"
	RuleTrailingPunct = "trailing-punctuation"
	RuleVSCodeWording = "vscode-wording"
	RuleForeignWord   = "foreign-word"
	RuleEdgeSpace     = "edge-space"

	Ellipsis        = "..."
	unicodeEllipsis = "\u2026"
)

var smallWords = map[string]bool{
	"a": true, "an": true, "and": true, "as": true, "at": true, "but": true, "by": true, "for": true,
	"from": true, "in": true, "of": true, "on": true, "or": true, "the": true, "to": true, "vs": true,
	"with": true, "via": true, "into": true, "per": true, "than": true,
}

var trailingPunct = ".,;:!?"

type Issue struct {
	Rule    string
	Message string
}

func (i Issue) String() string { return i.Rule + ": " + i.Message }

func CheckTitle(title string, opensInput bool, classified bool) []Issue {
	var issues []Issue
	add := func(rule, format string, args ...any) {
		issues = append(issues, Issue{rule, fmt.Sprintf(format, args...)})
	}
	if title != strings.TrimSpace(title) {
		add(RuleEdgeSpace, "%q starts or ends with a space", title)
	}
	if strings.Contains(title, "  ") {
		add(RuleDoubleSpace, "%q has a double space", title)
	}
	body := strings.TrimSpace(title)
	if strings.Contains(body, unicodeEllipsis) {
		add(RuleEllipsis, "%q uses the Unicode ellipsis, VS Code writes %q", title, Ellipsis)
		body = strings.ReplaceAll(body, unicodeEllipsis, Ellipsis)
	}
	hasEllipsis := strings.HasSuffix(body, Ellipsis)
	body = strings.TrimSuffix(body, Ellipsis)
	if body != "" && strings.ContainsRune(trailingPunct, rune(body[len(body)-1])) {
		add(RuleTrailingPunct, "%q ends with %q", title, body[len(body)-1:])
	}
	if classified {
		switch {
		case opensInput && !hasEllipsis:
			add(RuleEllipsis, "%q opens a dialog or picker, so it ends with %q", title, Ellipsis)
		case !opensInput && hasEllipsis:
			add(RuleEllipsis, "%q opens no dialog or picker, so it has no %q", title, Ellipsis)
		}
	}
	if word, ok := notTitleCase(body); !ok {
		add(RuleTitleCase, "%q: %q is not in Title Case (capitalize it, except a, an, and, as, at, but, by, for, from, in, of, on, or, the, to, with)", title, word)
	}
	return issues
}

var particleVerbs = map[string]bool{"Zoom": true, "Log": true, "Sign": true, "Check": true, "Turn": true}

func notTitleCase(s string) (string, bool) {
	words := strings.Fields(s)
	for i, word := range words {
		if strings.HasPrefix(word, "'") || strings.HasPrefix(word, "\"") || strings.HasPrefix(word, "`") || strings.HasPrefix(word, ".") {
			continue
		}
		if i > 0 && particleVerbs[words[i-1]] && (word == "In" || word == "Out" || word == "Off" || word == "On" || word == "Up") {
			continue
		}
		for _, part := range splitWord(word) {
			if !partOK(part, i == 0) {
				return word, false
			}
		}
	}
	return "", true
}

func splitWord(word string) []string {
	return strings.FieldsFunc(word, func(r rune) bool { return r == '/' || r == '-' || r == '(' || r == ')' || r == ',' })
}

func partOK(part string, first bool) bool {
	runes := []rune(part)
	letter := -1
	for i, r := range runes {
		if unicode.IsLetter(r) {
			letter = i
			break
		}
	}
	if letter < 0 {
		return true
	}
	if len(runes) > 1 && part == strings.ToUpper(part) {
		return true
	}
	if low := strings.ToLower(part); smallWords[low] {
		if first {
			return unicode.IsUpper(runes[letter])
		}
		return part == low
	}
	if unicode.IsUpper(runes[letter]) {
		return true
	}
	for _, r := range runes[letter:] {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

var foreignWords = map[string]string{
	"shelve":   "Stash",
	"shelf":    "Stashes",
	"rollback": "Discard Changes",
	"annotate": "Toggle Git Blame",
	"sidebar":  "Side Bar",
	"unstash":  "Pop Stash",
}

func CheckVSCodeWords(title string) []Issue {
	var issues []Issue
	for _, w := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if vs, ok := foreignWords[w]; ok {
			issues = append(issues, Issue{RuleForeignWord, fmt.Sprintf("%q uses %q, VS Code says %q", title, w, vs)})
		}
	}
	return issues
}

func CompareVSCode(id, title string) *Issue {
	want, ok := VSCodeTitles[id]
	if !ok || slices.Contains(want, title) {
		return nil
	}
	return &Issue{RuleVSCodeWording, fmt.Sprintf("%s is %q, VS Code says %q", id, title, want[0])}
}
