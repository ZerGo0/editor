package textsearch

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	groupPrefix = '$'
	hyphen      = "-"
	underscore  = "_"
)

type Replacement struct {
	Template     string
	Regex        bool
	PreserveCase bool
}

func (m *Matcher) Replacement(template string, preserveCase bool) Replacement {
	return Replacement{Template: template, Regex: m.regex, PreserveCase: preserveCase}
}

func (r Replacement) Expand(matched string, groups []string) string {
	out := r.Template
	if r.Regex {
		out = expandTemplate(r.Template, matched, groups)
	}
	if r.PreserveCase && matched != "" {
		out = preserveCase(matched, out)
	}
	return out
}

func expandTemplate(tpl, matched string, groups []string) string {
	if !strings.ContainsAny(tpl, "$\\") {
		return tpl
	}
	var b strings.Builder
	for i := 0; i < len(tpl); i++ {
		c := tpl[i]
		switch {
		case c == groupPrefix && i+1 < len(tpl):
			i = expandGroup(&b, tpl, i, matched, groups)
		case c == '\\' && i+1 < len(tpl):
			i++
			switch tpl[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte('\\')
				b.WriteByte(tpl[i])
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func expandGroup(b *strings.Builder, tpl string, i int, matched string, groups []string) int {
	next := tpl[i+1]
	switch {
	case next == groupPrefix:
		b.WriteByte(groupPrefix)
		return i + 1
	case next == '&':
		b.WriteString(matched)
		return i + 1
	case next >= '0' && next <= '9':
		digits := 1
		n := int(next - '0')
		if i+2 < len(tpl) && tpl[i+2] >= '0' && tpl[i+2] <= '9' {
			if two := n*10 + int(tpl[i+2]-'0'); two <= len(groups) {
				n, digits = two, 2
			}
		}
		switch {
		case n == 0:
			b.WriteString(matched)
		case n <= len(groups):
			b.WriteString(groups[n-1])
		default:
			b.WriteString(tpl[i : i+1+digits])
		}
		return i + digits
	}
	b.WriteByte(groupPrefix)
	return i
}

func preserveCase(matched, repl string) string {
	hasHyphen := strings.Contains(matched, hyphen) && strings.Contains(repl, hyphen)
	hasUnderscore := strings.Contains(matched, underscore) && strings.Contains(repl, underscore)
	switch {
	case hasHyphen && !hasUnderscore:
		return preserveCaseParts(matched, repl, hyphen)
	case hasUnderscore && !hasHyphen:
		return preserveCaseParts(matched, repl, underscore)
	}
	return preserveCaseWord(matched, repl)
}

func preserveCaseParts(matched, repl, sep string) string {
	from, to := strings.Split(matched, sep), strings.Split(repl, sep)
	if len(from) != len(to) {
		return preserveCaseWord(matched, repl)
	}
	for i := range to {
		to[i] = preserveCaseWord(from[i], to[i])
	}
	return strings.Join(to, sep)
}

func preserveCaseWord(matched, repl string) string {
	if matched == "" || repl == "" {
		return repl
	}
	first, _ := utf8.DecodeRuneInString(matched)
	head, size := utf8.DecodeRuneInString(repl)
	switch {
	case strings.ToUpper(matched) == matched:
		return strings.ToUpper(repl)
	case strings.ToLower(matched) == matched:
		return strings.ToLower(repl)
	case unicode.IsUpper(first):
		return string(unicode.ToUpper(head)) + repl[size:]
	case unicode.IsLower(first):
		return string(unicode.ToLower(head)) + repl[size:]
	}
	return repl
}

type Edit struct {
	Start, End int
	Text       string
}

type Plan struct {
	Text  string
	Edits []Edit
	CRLF  bool
}

func (m *Matcher) PlanReplace(text string, r Replacement, accept func(Match) bool) Plan {
	p := Plan{Text: text}
	if m.multiline && strings.Contains(text, "\r\n") {
		p.Text, p.CRLF = strings.ReplaceAll(text, "\r\n", "\n"), true
	}
	var rc runeCounter
	m.each(context.Background(), p.Text, 0, func(h hit) bool {
		mt := buildMatch(p.Text, h, &rc)
		if accept != nil && !accept(mt) {
			return true
		}
		matched, groups := p.Text[h.start:h.end], fullGroups(p.Text, h)
		p.Edits = append(p.Edits, Edit{Start: h.start, End: h.end, Text: r.Expand(matched, groups)})
		return true
	})
	return p
}

func fullGroups(text string, h hit) []string {
	if len(h.sub) <= 2 {
		return nil
	}
	groups := make([]string, 0, len(h.sub)/2-1)
	for i := 2; i+1 < len(h.sub); i += 2 {
		if h.sub[i] < 0 {
			groups = append(groups, "")
			continue
		}
		groups = append(groups, text[h.sub[i]:h.sub[i+1]])
	}
	return groups
}

func (p Plan) Apply() string {
	var b strings.Builder
	b.Grow(len(p.Text))
	at := 0
	for _, e := range p.Edits {
		b.WriteString(p.Text[at:e.Start])
		b.WriteString(e.Text)
		at = e.End
	}
	b.WriteString(p.Text[at:])
	if p.CRLF {
		return strings.ReplaceAll(b.String(), "\n", "\r\n")
	}
	return b.String()
}

func ReplaceFile(path string, m *Matcher, r Replacement, accept func(Match) bool, maxSize int64, beforeWrite func()) (int, error) {
	if maxSize <= 0 {
		maxSize = DefaultMaxFileSize
	}
	text, info, err := readText(path, maxSize)
	if err != nil {
		return 0, fmt.Errorf("replace in %s: %w", path, err)
	}
	// TECHDEBT: A file that changes between the read and the rename is overwritten; compare its modification time before the rename if concurrent edits matter.
	plan := m.PlanReplace(text, r, accept)
	if len(plan.Edits) == 0 {
		return 0, nil
	}
	if beforeWrite != nil {
		beforeWrite()
	}
	if err := fsutil.WriteFileAtomic(path, []byte(plan.Apply()), info.Mode().Perm()); err != nil {
		return 0, fmt.Errorf("replace in %s: %w", path, err)
	}
	return len(plan.Edits), nil
}
