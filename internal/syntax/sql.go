package syntax

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

const (
	sqlModeCode = iota
	sqlModeComment
	sqlModeString
	sqlModeEString
	sqlModeDollarString
	sqlModeDollarCode
	sqlModeQuotedName
	sqlModeMask     = 0xf
	sqlTagShift     = 8
	sqlTagMask      = 0xff
	sqlDepthShift   = 16
	sqlDepthMask    = 0xff
	sqlMaxWordBytes = 40
	sqlTagMix       = 31
)

var (
	sqlKeywords  = set("abort action add after all alter always analyze and any array as asc asymmetric at authorization backward before begin between by cascade case cast check checkpoint close cluster collate column comment commit committed concurrently conflict connection constraint constraints copy cost create cross csv cube current cursor cycle data database deallocate declare default deferrable deferred definer delete delimiter desc detach distinct do domain drop each else elsif enable encoding end enum escape event except exception exclude exclusive execute exists explain extension external false fetch filter first following for force foreign forward from full function functions generated global grant group grouping handler having header hold if ilike immediate immutable in include increment index inherit inherits initially inline inner inout input insert instead intersect into invoker is isolation join key language large last lateral leading leakproof left level like limit listen load local lock locked logged loop mapping match materialized maxvalue merge minvalue mode move name names natural new next no none not nothing notify nowait null nulls of off offset oids old on only operator option options or order ordinality others out outer over overlaps overriding owned owner parallel partition passing password placing plans policy preceding prepare prepared primary prior privileges procedural procedure publication quote range read reassign recheck recursive reference references refresh reindex relative release rename repeatable replace replica reset restart restrict return returning returns revoke right role rollback rollup routine routines row rows rule savepoint schema schemas scroll search security select sequence sequences serializable server session set setof sets share show similar simple skip snapshot some stable standalone start statement statistics stdin stdout storage strict strip subscription substring symmetric sysid system table tables tablespace temp template temporary then ties to trailing transaction transform trigger truncate trusted type types unbounded uncommitted unencrypted union unique unknown unlisten unlogged until update user using vacuum valid validate validator value values variadic verbose version view views volatile when where while window with within without work wrapper write zone raise perform notice using format")
	sqlTypes     = set("trigger event_trigger bigint bigserial bit bool boolean box bpchar bytea char character cidr circle date daterange decimal double float float4 float8 inet int int2 int4 int8 int4range int8range integer interval json jsonb line lseg macaddr money numeric numrange oid path point polygon precision real record regclass serial serial2 serial4 serial8 smallint smallserial text time timestamp timestamptz timetz tsquery tsrange tstzrange tsvector uuid varbit varchar void xml citext hstore ltree vector")
	sqlConstants = set("true false null current_date current_time current_timestamp current_user session_user localtime localtimestamp")
	sqlFunctions = set("abs avg array_agg bool_and bool_or btrim ceil ceiling coalesce concat concat_ws count date_part date_trunc extract exists floor generate_series greatest gen_random_uuid initcap jsonb_agg jsonb_array_elements jsonb_build_array jsonb_build_object jsonb_each jsonb_set json_agg json_build_object least left length lower lpad ltrim max md5 min mod now nullif position random regexp_replace replace right round row_number rank dense_rank lag lead first_value last_value rpad rtrim split_part string_agg substr substring sum to_char to_date to_jsonb to_timestamp trim trunc unnest upper uuid_generate_v4 version coalesce cast convert")
)

type sqlLexer struct{}

type sqlScan struct {
	writer
	line  string
	mode  int
	tag   int
	depth int
	asDo  bool
	cast  bool
	dot   bool
}

func (sqlLexer) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	s := sqlScan{writer: newWriter(tokens), line: line, mode: int(state & sqlModeMask), tag: int(state >> sqlTagShift & sqlTagMask), depth: int(state >> sqlDepthShift & sqlDepthMask)}
	for i := 0; i < len(line); {
		switch s.mode {
		case sqlModeComment:
			i = s.blockComment(i)
		case sqlModeString, sqlModeEString:
			i = s.stringBody(i)
		case sqlModeQuotedName:
			i = s.quotedName(i)
		case sqlModeDollarString:
			i = s.dollarBody(i, editor.TokenString)
		default:
			i = s.code(i)
		}
	}
	return s.toks, uint64(s.mode) | uint64(s.tag)<<sqlTagShift | uint64(s.depth)<<sqlDepthShift
}

func (s *sqlScan) blockComment(i int) int {
	line := s.line
	for j := i; j < len(line); j++ {
		switch {
		case strings.HasPrefix(line[j:], "/*"):
			s.depth++
			j++
		case strings.HasPrefix(line[j:], "*/"):
			j++
			if s.depth == 0 {
				s.mode = sqlModeCode
				return s.emit(j+1, editor.TokenComment)
			}
			s.depth--
		}
	}
	return s.emit(len(line), editor.TokenComment)
}

func (s *sqlScan) stringBody(i int) int {
	line := s.line
	for j := i; j < len(line); j++ {
		switch {
		case line[j] == '\\' && s.mode == sqlModeEString:
			s.emit(j, editor.TokenString)
			j = s.emit(min(len(line), escapeEnd(line, j)), editor.TokenEscape) - 1
		case line[j] == '\'':
			if j+1 < len(line) && line[j+1] == '\'' {
				j++
				continue
			}
			s.mode = sqlModeCode
			return s.emit(j+1, editor.TokenString)
		}
	}
	return s.emit(len(line), editor.TokenString)
}

func (s *sqlScan) quotedName(i int) int {
	end := strings.IndexByte(s.line[i:], '"')
	if end < 0 {
		return s.emit(len(s.line), editor.TokenVariable)
	}
	s.mode = sqlModeCode
	return s.emit(i+end+1, editor.TokenVariable)
}

func dollarTag(line string, i int) (end, tag int) {
	j := i + 1
	h := 0
	for j < len(line) && (isIdentPartByte(line[j]) && line[j] != '$') {
		h = (h*sqlTagMix + int(line[j])) & sqlTagMask
		j++
	}
	if j < len(line) && line[j] == '$' && (j == i+1 || !isDigit(line[i+1])) {
		return j + 1, h + 1
	}
	return 0, 0
}

func (s *sqlScan) dollarBody(i int, kind editor.TokenKind) int {
	line := s.line
	for j := i; j < len(line); j++ {
		if line[j] != '$' {
			continue
		}
		if end, tag := dollarTag(line, j); end > 0 && tag == s.tag {
			s.emit(j, kind)
			s.mode, s.tag = sqlModeCode, 0
			return s.emit(end, editor.TokenMeta)
		}
	}
	return s.emit(len(line), kind)
}

func (s *sqlScan) code(i int) int {
	line := s.line
	c := line[i]
	switch {
	case c == ' ' || c == '\t' || c == '\r':
		return s.emit(i+1, editor.TokenText)
	case c == '-' && i+1 < len(line) && line[i+1] == '-':
		return s.emit(len(line), editor.TokenComment)
	case c == '/' && i+1 < len(line) && line[i+1] == '*':
		s.mode, s.depth = sqlModeComment, 0
		return s.blockComment(i + 2)
	case c == '\'':
		s.mode = sqlModeString
		s.asDo, s.cast, s.dot = false, false, false
		return s.stringBody(i + 1)
	case c == '"':
		s.mode = sqlModeQuotedName
		return s.quotedName(i + 1)
	case c == '$':
		return s.dollar(i)
	case isDigit(c) || c == '.' && i+1 < len(line) && isDigit(line[i+1]):
		s.asDo, s.cast, s.dot = false, false, false
		return s.emit(s.numberEnd(i), editor.TokenNumber)
	case identStartAt(line, i):
		return s.word(i)
	case c >= utf8.RuneSelf:
		_, n := utf8.DecodeRuneInString(line[i:])
		return s.emit(i+n, editor.TokenText)
	}
	return s.operator(i)
}

func (s *sqlScan) numberEnd(i int) int {
	line := s.line
	j := i
	for j < len(line) && (isDigit(line[j]) || line[j] == '_' || line[j] == '.' && j+1 < len(line) && isDigit(line[j+1]) ||
		(line[j] == 'e' || line[j] == 'E') && j+1 < len(line) && (isDigit(line[j+1]) || line[j+1] == '-' || line[j+1] == '+')) {
		if line[j] == 'e' || line[j] == 'E' {
			j++
		}
		j++
	}
	return j
}

func (s *sqlScan) dollar(i int) int {
	line := s.line
	if i+1 < len(line) && isDigit(line[i+1]) {
		j := i + 1
		for j < len(line) && isDigit(line[j]) {
			j++
		}
		return s.emit(j, editor.TokenVariable)
	}
	end, tag := dollarTag(line, i)
	if end == 0 {
		return s.emit(i+1, editor.TokenOperator)
	}
	if s.mode == sqlModeDollarCode {
		if tag == s.tag {
			s.mode, s.tag = sqlModeCode, 0
		}
		return s.emit(end, editor.TokenMeta)
	}
	s.tag = tag
	s.emit(end, editor.TokenMeta)
	if s.asDo {
		s.mode = sqlModeDollarCode
		return end
	}
	s.mode = sqlModeDollarString
	return end
}

func (s *sqlScan) operator(i int) int {
	line := s.line
	c := line[i]
	s.dot, s.asDo, s.cast = false, false, false
	switch c {
	case '(', ')', ',', ';', '[', ']':
		return s.emit(i+1, editor.TokenPunctuation)
	case '.':
		s.dot = true
		return s.emit(i+1, editor.TokenPunctuation)
	case ':':
		if strings.HasPrefix(line[i:], "::") {
			s.cast = true
			return s.emit(i+2, editor.TokenOperator)
		}
	}
	j := i + 1
	for j < len(line) && strings.IndexByte("+-*/%<>=!~|&^@#?:", line[j]) >= 0 && !strings.HasPrefix(line[j:], "--") && !strings.HasPrefix(line[j:], "/*") {
		j++
	}
	return s.emit(j, editor.TokenOperator)
}

func (s *sqlScan) word(i int) int {
	line := s.line
	end := scanIdent(line, i)
	w := line[i:end]
	dot, cast := s.dot, s.cast
	s.dot, s.cast = false, false
	var buf [sqlMaxWordBytes]byte
	n := len(w)
	if n > sqlMaxWordBytes {
		s.asDo = false
		return s.plainWord(end, dot)
	}
	for k := 0; k < n; k++ {
		c := w[k]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[k] = c
	}
	lower := buf[:n]
	s.asDo = string(lower) == "as" || string(lower) == "do"
	nx := byte(0)
	if end < len(line) {
		nx = line[end]
	}
	switch {
	case string(lower) == "e" && nx == '\'':
		s.mode = sqlModeEString
		s.asDo = false
		return s.stringBody(end + 1)
	case dot:
		if nx == '(' {
			return s.emit(end, editor.TokenFunction)
		}
		return s.emit(end, editor.TokenProperty)
	case cast || sqlTypes[string(lower)]:
		return s.emit(end, editor.TokenType)
	case sqlConstants[string(lower)]:
		return s.emit(end, editor.TokenConstant)
	case nx == '(' && (sqlFunctions[string(lower)] || !sqlKeywords[string(lower)]):
		return s.emit(end, editor.TokenFunction)
	case sqlKeywords[string(lower)]:
		return s.emit(end, editor.TokenKeyword)
	}
	return s.emit(end, editor.TokenVariable)
}

func (s *sqlScan) plainWord(end int, dot bool) int {
	switch {
	case dot:
		return s.emit(end, editor.TokenProperty)
	case end < len(s.line) && s.line[end] == '(':
		return s.emit(end, editor.TokenFunction)
	}
	return s.emit(end, editor.TokenVariable)
}
