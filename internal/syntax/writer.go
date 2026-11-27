package syntax

import "github.com/egoist/mygo/ui/editor"

type writer struct {
	toks []editor.Token
	base int
	pos  int
}

func newWriter(tokens []editor.Token) writer { return writer{toks: tokens, base: len(tokens)} }

func (w *writer) emit(end int, kind editor.TokenKind) int {
	if end <= w.pos {
		return end
	}
	w.pos = end
	if n := len(w.toks); n > w.base && w.toks[n-1].Kind == kind {
		w.toks[n-1].End = end
		return end
	}
	w.toks = append(w.toks, editor.Token{End: end, Kind: kind})
	return end
}

func (w *writer) nestGuarded(t editor.Tokenizer, line string, offset int, state uint64) (next uint64) {
	n, pos := len(w.toks), w.pos
	var last editor.Token
	if n > w.base {
		last = w.toks[n-1]
	}
	defer func() {
		if recover() == nil {
			return
		}
		w.toks, w.pos = w.toks[:n], pos
		if n > w.base {
			w.toks[n-1] = last
		}
		w.emit(offset+len(line), editor.TokenCode)
		next = 0
	}()
	return w.nest(t, line, offset, state)
}

func (w *writer) nest(t editor.Tokenizer, line string, offset int, state uint64) uint64 {
	toks, next := t.Tokenize(line, state, w.toks[len(w.toks):])
	for _, tk := range toks {
		w.emit(offset+tk.End, tk.Kind)
	}
	return next
}
