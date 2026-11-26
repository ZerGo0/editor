package problems

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

var bracketlessLanguages = map[string]bool{
	"Markdown": true, "Diff": true, "CSV": true, "TSV": true, "Log": true, "reStructuredText": true,
	"LaTeX": true, "Batch": true, "Assembly": true, "Perl": true, "Shell": true, "Dotenv": true,
	"Ignore": true, "Git Commit": true, "Git Rebase Todo": true, "HTML": true, "ERB": true,
	"Svelte": true, "XML": true, "Jinja": true,
}

type openBracket struct {
	pair      int
	line, col int
}

func bracketsMeaningful(lang *editor.Language) bool {
	return lang != nil && lang.Tokenizer != nil && len(lang.Brackets) > 0 && !bracketlessLanguages[lang.Name]
}

func isCode(kind editor.TokenKind) bool {
	switch kind {
	case editor.TokenString, editor.TokenComment, editor.TokenRegexp, editor.TokenEscape:
		return false
	}
	return true
}

func checkBrackets(lang *editor.Language, text string) []Problem {
	if !bracketsMeaningful(lang) {
		return nil
	}
	var pairs []editor.BracketPair
	for _, p := range lang.Brackets {
		if p.Open != "" && p.Close != "" && p.Open != p.Close {
			pairs = append(pairs, p)
		}
	}
	var (
		stack    []openBracket
		out      []Problem
		toks     []editor.Token
		state    uint64
		overflow bool
	)
	eachLine(text, func(no int, line string) bool {
		toks, state = lang.Tokenizer.Tokenize(line, state, toks[:0])
		start := 0
		for _, tk := range toks {
			if isCode(tk.Kind) {
				for col := start; col < tk.End; {
					width := 1
					for k, p := range pairs {
						if strings.HasPrefix(line[col:tk.End], p.Open) {
							stack = append(stack, openBracket{k, no, col})
							width = len(p.Open)
							break
						}
						if strings.HasPrefix(line[col:tk.End], p.Close) {
							width = len(p.Close)
							if n := len(stack); n > 0 && stack[n-1].pair == k {
								stack = stack[:n-1]
								break
							}
							out = append(out, Problem{Line: no, Col: col, EndCol: col + width, Severity: Error, Message: "Unmatched '" + p.Close + "'", Source: sourceBrackets})
							break
						}
					}
					if len(out) >= MaxProblemsPerKind {
						overflow = true
						return false
					}
					col += width
				}
			}
			start = tk.End
		}
		return true
	})
	if overflow {
		return out
	}
	for _, o := range stack {
		if len(out) >= MaxProblemsPerKind {
			break
		}
		open := pairs[o.pair].Open
		out = append(out, Problem{Line: o.line, Col: o.col, EndCol: o.col + len(open), Severity: Error, Message: "'" + open + "' is not closed", Source: sourceBrackets})
	}
	return out
}
