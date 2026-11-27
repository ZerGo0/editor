package syntax

import (
	"strings"
	"sync"

	"github.com/egoist/mygo/ui/editor"
)

const (
	mdInnerBits    = markupStateBits
	mdInnerMask    = 1<<mdInnerBits - 1
	mdFenceBit     = 1 << mdInnerBits
	mdTildeBit     = mdFenceBit << 1
	mdLengthShift  = mdInnerBits + 2
	mdLengthMask   = 0xf
	mdLangShift    = mdLengthShift + 4
	mdLangMask     = 0xff
	mdCommentBit   = 1 << (mdLangShift + 8)
	mdFrontBit     = mdCommentBit << 1
	mdFrontDoneBit = mdFrontBit << 1
	mdESMBit       = mdFrontDoneBit << 1
	mdJSXBit       = mdESMBit << 1
	mdBuiltinBit   = 1 << 54
	mdMaxFence     = 15
	mdIndentLimit  = 4
	mdFenceMarks   = "`~"
)

type markdown struct {
	mdx   bool
	block editor.Tokenizer
	jsx   *markup
	esm   *ecma
}

var fenceLanguages = sync.OnceValue(func() map[*editor.Language]int {
	index := map[*editor.Language]int{}
	add := func(l *editor.Language) {
		if _, ok := index[l]; !ok && l.Tokenizer != nil && len(index) < mdLangMask {
			index[l] = len(index) + 1
		}
	}
	for _, e := range entries {
		add(e.lang)
	}
	for _, l := range editor.Languages() {
		add(l)
	}
	return index
})

var fenceByIndex = sync.OnceValue(func() []*editor.Language {
	index := fenceLanguages()
	out := make([]*editor.Language, len(index)+1)
	for l, k := range index {
		out[k] = l
	}
	return out
})

func (m *markdown) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	w := newWriter(tokens)
	n := len(line)
	trimmed := strings.TrimLeft(line, " ")
	indent := n - len(trimmed)
	switch {
	case state&mdFrontBit != 0:
		return m.frontmatter(&w, line, state)
	case state&mdFenceBit != 0:
		return m.fenced(&w, line, state, trimmed, indent)
	case state&mdESMBit != 0:
		return m.module(&w, line, state)
	case state&mdJSXBit != 0:
		return m.component(&w, line, state, trimmed)
	}
	if state&mdFrontDoneBit == 0 && trimmed != "" {
		state |= mdFrontDoneBit
		if line == frontmatterFlag {
			w.emit(n, editor.TokenMeta)
			return w.toks, state | mdFrontBit
		}
	}
	if next, ok := m.fenceOpen(&w, line, state, trimmed, indent); ok {
		return w.toks, next
	}
	if m.mdx && indent == 0 {
		switch {
		case strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "export "):
			return m.module(&w, line, state|mdESMBit)
		case isTagStart(line, 0) && line[0] == '<' || strings.HasPrefix(line, "<>"):
			return m.component(&w, line, state|mdJSXBit, trimmed)
		}
	}
	builtin := uint64(0)
	if state&mdCommentBit != 0 {
		builtin = mdBuiltinBit
	}
	if n == 0 {
		return w.toks, state
	}
	toks, next := m.block.Tokenize(line, builtin, w.toks)
	state &^= mdCommentBit
	if next&mdBuiltinBit != 0 {
		state |= mdCommentBit
	}
	return toks, state
}

func (m *markdown) frontmatter(w *writer, line string, state uint64) ([]editor.Token, uint64) {
	if strings.TrimSpace(line) == frontmatterFlag {
		w.emit(len(line), editor.TokenMeta)
		return w.toks, state &^ (mdFrontBit | mdInnerMask)
	}
	frontmatterLine(w, line)
	return w.toks, state
}

func frontmatterLine(w *writer, line string) {
	trimmed := strings.TrimLeft(line, " \t-")
	indent := len(line) - len(trimmed)
	w.emit(indent, editor.TokenPunctuation)
	if strings.HasPrefix(trimmed, "#") {
		w.emit(len(line), editor.TokenComment)
		return
	}
	value := indent
	if colon := strings.Index(trimmed, ":"); colon > 0 && (colon+1 == len(trimmed) || trimmed[colon+1] == ' ') {
		w.emit(indent+colon, editor.TokenProperty)
		value = w.emit(indent+colon+1, editor.TokenPunctuation)
	}
	rest := strings.TrimSpace(line[value:])
	kind := editor.TokenString
	switch {
	case rest == "":
		kind = editor.TokenText
	case rest == "true" || rest == "false" || rest == "null" || rest == "~":
		kind = editor.TokenConstant
	case isDigit(rest[0]):
		kind = editor.TokenNumber
	case strings.HasPrefix(rest, "#"):
		kind = editor.TokenComment
	}
	w.emit(len(line), kind)
}

func clampInner(next uint64) uint64 {
	if next > mdInnerMask {
		return 0
	}
	return next
}

func (m *markdown) module(w *writer, line string, state uint64) ([]editor.Token, uint64) {
	next := w.nest(m.esm, line, 0, state&mdInnerMask)
	state &^= mdInnerMask
	if next&ecmaStackMask != 0 || next&ecmaCommentBit != 0 || next>>ecmaPrevShift&ecmaPrevMask == pvOperand {
		return w.toks, state | clampInner(next)
	}
	return w.toks, state &^ mdESMBit
}

func (m *markdown) component(w *writer, line string, state uint64, trimmed string) ([]editor.Token, uint64) {
	inner := state & mdInnerMask
	if trimmed == "" && inner>>mkModeShift&mkModeMask == mkText {
		w.emit(len(line), editor.TokenText)
		return w.toks, state &^ (mdJSXBit | mdInnerMask)
	}
	next := w.nest(m.jsx, line, 0, inner)
	return w.toks, state&^mdInnerMask | clampInner(next)
}

func (m *markdown) fenceOpen(w *writer, line string, state uint64, trimmed string, indent int) (uint64, bool) {
	if indent >= mdIndentLimit || trimmed == "" || strings.IndexByte(mdFenceMarks, trimmed[0]) < 0 {
		return 0, false
	}
	ch := trimmed[0]
	run := 0
	for run < len(trimmed) && trimmed[run] == ch {
		run++
	}
	info := strings.TrimSpace(trimmed[run:])
	if run < 3 || ch == '`' && strings.Contains(info, "`") {
		return 0, false
	}
	w.emit(indent+run, editor.TokenPunctuation)
	w.emit(len(line), editor.TokenType)
	next := state&^(mdInnerMask|mdLangMask<<mdLangShift|mdLengthMask<<mdLengthShift) | mdFenceBit | uint64(min(run, mdMaxFence))<<mdLengthShift
	if ch == '~' {
		next |= mdTildeBit
	}
	name := info
	if end := strings.IndexAny(name, " \t{:"); end >= 0 {
		name = name[:end]
	}
	if l := Named(name); l != nil {
		next |= uint64(fenceLanguages()[l]) << mdLangShift
	}
	return next, true
}

func (m *markdown) fenced(w *writer, line string, state uint64, trimmed string, indent int) ([]editor.Token, uint64) {
	ch := byte('`')
	if state&mdTildeBit != 0 {
		ch = '~'
	}
	length := int(state >> mdLengthShift & mdLengthMask)
	run := 0
	for run < len(trimmed) && trimmed[run] == ch {
		run++
	}
	if indent < mdIndentLimit && run >= length && strings.TrimSpace(trimmed[run:]) == "" {
		w.emit(len(line), editor.TokenPunctuation)
		return w.toks, state &^ (mdFenceBit | mdTildeBit | mdLengthMask<<mdLengthShift | mdLangMask<<mdLangShift | mdInnerMask)
	}
	k := int(state >> mdLangShift & mdLangMask)
	list := fenceByIndex()
	if k == 0 || k >= len(list) || list[k] == nil {
		w.emit(len(line), editor.TokenCode)
		return w.toks, state
	}
	next := w.nestGuarded(list[k].Tokenizer, line, 0, state&mdInnerMask)
	return w.toks, state&^mdInnerMask | clampInner(next)
}
