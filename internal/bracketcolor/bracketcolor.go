package bracketcolor

import (
	"context"

	"github.com/egoist/mygo/ui/editor"
)

const (
	cancelCheckLines = 1024
	pairable         = "()[]{}"
)

type Lines interface {
	LineCount() int
	Line(i int) string
	LineStart(i int) int
}

type Bracket struct {
	Offset     int
	Depth      int
	Char       byte
	Unexpected bool
}

type pairs struct {
	opens, closes string
}

func pairsOf(lang *editor.Language) pairs {
	var p pairs
	if lang == nil {
		return p
	}
	for _, b := range lang.Brackets {
		if len(b.Open) == 1 && len(b.Close) == 1 && containsByte(pairable, b.Open[0]) && containsByte(pairable, b.Close[0]) {
			p.opens += b.Open
			p.closes += b.Close
		}
	}
	return p
}

func containsByte(s string, b byte) bool {
	for i := range len(s) {
		if s[i] == b {
			return true
		}
	}
	return false
}

func indexByte(s string, b byte) int {
	for i := range len(s) {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func lastIndex(stack []int, k int) int {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == k {
			return i
		}
	}
	return -1
}

func Supports(lang *editor.Language) bool {
	return pairsOf(lang).opens != ""
}

func Scan(ctx context.Context, text Lines, lang *editor.Language) ([]Bracket, error) {
	p := pairsOf(lang)
	if p.opens == "" {
		return nil, nil
	}
	var out []Bracket
	var stack []int
	var tokens []editor.Token
	var state uint64
	for i := range text.LineCount() {
		if i%cancelCheckLines == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		line := text.Line(i)
		start := text.LineStart(i)
		tokens = tokens[:0]
		if lang.Tokenizer != nil {
			tokens, state = lang.Tokenizer.Tokenize(line, state, tokens)
		}
		tok := 0
		for j := range len(line) {
			for tok < len(tokens) && tokens[tok].End <= j {
				tok++
			}
			if tok < len(tokens) && !code(tokens[tok].Kind) {
				continue
			}
			if k := indexByte(p.opens, line[j]); k >= 0 {
				out = append(out, Bracket{Offset: start + j, Depth: len(stack), Char: line[j]})
				stack = append(stack, k)
				continue
			}
			k := indexByte(p.closes, line[j])
			if k < 0 {
				continue
			}
			if at := lastIndex(stack, k); at >= 0 {
				stack = stack[:at]
				out = append(out, Bracket{Offset: start + j, Depth: at, Char: line[j]})
				continue
			}
			out = append(out, Bracket{Offset: start + j, Depth: len(stack), Char: line[j], Unexpected: true})
		}
	}
	return out, nil
}

func code(k editor.TokenKind) bool {
	return k != editor.TokenString && k != editor.TokenComment && k != editor.TokenRegexp && k != editor.TokenEscape && k != editor.TokenCode
}
