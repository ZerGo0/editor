package syntax

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

const (
	fuzzLines            = 300
	fuzzMaxLen           = 160
	fuzzSeed             = 20260101
	fuzzRandomStateEvery = 7
	fuzzAlphabet         = "\"'`#/*-(){}[]\\ \t;%!<>=|+.:@$,\n0aZ_é"
)

func tokenize(t *testing.T, l *editor.Language, line string, state uint64) ([]editor.Token, uint64) {
	t.Helper()
	toks, next := l.Tokenizer.Tokenize(line, state, nil)
	checkTokens(t, l.Name, line, toks)
	return toks, next
}

func checkTokens(t *testing.T, name, line string, toks []editor.Token) {
	t.Helper()
	prev := 0
	for _, tk := range toks {
		if tk.End <= prev || tk.End > len(line) {
			t.Fatalf("%s: bad token end %d after %d in %q (%v)", name, tk.End, prev, line, toks)
		}
		prev = tk.End
	}
	if prev != len(line) {
		t.Fatalf("%s: tokens end at %d, line %q has %d bytes", name, prev, line, len(line))
	}
}

func kindOf(toks []editor.Token, line, sub string) (editor.TokenKind, bool) {
	at := strings.Index(line, sub)
	if at < 0 {
		return 0, false
	}
	start := 0
	for _, tk := range toks {
		if at < tk.End {
			return tk.Kind, start <= at
		}
		start = tk.End
	}
	return 0, false
}

func TestTokenizersMarkKeywordStringComment(t *testing.T) {
	tests := []struct {
		file, line, keyword, str, comment string
	}{
		{"a.ex", `def foo(x) do "hi" end # note`, "def", `"hi"`, "# note"},
		{"a.erl", `case X of "hi" -> ok end. % note`, "case", `"hi"`, "% note"},
		{"a.hs", `import Foo "hi" -- note`, "import", `"hi"`, "-- note"},
		{"a.ml", `let s = "hi" (* note *)`, "let", `"hi"`, "(* note *)"},
		{"a.fs", `let s = "hi" // note`, "let", `"hi"`, "// note"},
		{"a.clj", `(defn foo "hi") ; note`, "defn", `"hi"`, "; note"},
		{"a.lisp", `(define x "hi") ; note`, "define", `"hi"`, "; note"},
		{"a.jl", `function f() "hi" end # note`, "function", `"hi"`, "# note"},
		{"a.pl", `my $x = "hi"; # note`, "my", `"hi"`, "# note"},
		{"a.ps1", `if ($x) { "hi" } # note`, "if", `"hi"`, "# note"},
		{"a.groovy", `def x = "hi" // note`, "def", `"hi"`, "// note"},
		{"a.elm", `import X "hi" -- note`, "import", `"hi"`, "-- note"},
		{"a.nim", `proc f() = "hi" # note`, "proc", `"hi"`, "# note"},
		{"a.cr", `def f "hi" end # note`, "def", `"hi"`, "# note"},
		{"a.d", `foreach (x; "hi") // note`, "foreach", `"hi"`, "// note"},
		{"a.f90", `program p 'hi' ! note`, "program", "'hi'", "! note"},
		{"a.pas", `begin 'hi' // note`, "begin", "'hi'", "// note"},
		{"a.asm", `mov rax, "hi" ; note`, "mov", `"hi"`, "; note"},
		{"CMakeLists.txt", `if(X) "hi" # note`, "if", `"hi"`, "# note"},
		{"BUILD", `def f(): "hi" # note`, "def", `"hi"`, "# note"},
		{"a.gleam", `fn f() { "hi" } // note`, "fn", `"hi"`, "// note"},
		{"a.sol", `contract C { "hi" } // note`, "contract", `"hi"`, "// note"},
		{"a.prisma", `model User { "hi" } // note`, "model", `"hi"`, "// note"},
		{"a.v", `module m; "hi" // note`, "module", `"hi"`, "// note"},
		{"a.vhd", `entity e is "hi" -- note`, "entity", `"hi"`, "-- note"},
		{"a.tex", `\section{a} % note`, `\section`, "", "% note"},
		{"a.bat", `if "hi" == x :: note`, "if", `"hi"`, ":: note"},
		{"a.tcl", `proc f "hi" # note`, "proc", `"hi"`, "# note"},
		{"a.awk", `BEGIN { "hi" } # note`, "BEGIN", `"hi"`, "# note"},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			l := ForName(tc.file)
			if l == nil {
				t.Fatalf("no language for %s", tc.file)
			}
			toks, state := tokenize(t, l, tc.line, 0)
			if state != 0 {
				t.Errorf("state after line = %d, want 0", state)
			}
			for _, w := range []struct {
				sub  string
				kind editor.TokenKind
			}{{tc.keyword, editor.TokenKeyword}, {tc.str, editor.TokenString}, {tc.comment, editor.TokenComment}} {
				if w.sub == "" {
					continue
				}
				if got, ok := kindOf(toks, tc.line, w.sub); !ok || got != w.kind {
					t.Errorf("%q in %q has kind %d, want %d (%v)", w.sub, tc.line, got, w.kind, toks)
				}
			}
		})
	}
}

func TestBlockCommentState(t *testing.T) {
	tests := []struct{ file, open, close string }{
		{"a.hs", "{-", "-}"}, {"a.ml", "(*", "*)"}, {"a.fs", "(*", "*)"}, {"a.lisp", "#|", "|#"},
		{"a.jl", "#=", "=#"}, {"a.ps1", "<#", "#>"}, {"a.groovy", "/*", "*/"}, {"a.nim", "#[", "]#"},
		{"a.d", "/+", "+/"}, {"a.pas", "{", "}"}, {"a.asm", "/*", "*/"}, {"CMakeLists.txt", "#[[", "]]"},
		{"a.sol", "/*", "*/"}, {"a.v", "/*", "*/"}, {"a.elm", "{-", "-}"},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			l := ForName(tc.file)
			open := "x " + tc.open + " start"
			mid := "def let end"
			closing := "tail " + tc.close + " y"

			toks, state := tokenize(t, l, open, 0)
			if state == 0 {
				t.Fatalf("state after %q is 0", open)
			}
			if kind, _ := kindOf(toks, open, tc.open); kind != editor.TokenComment {
				t.Errorf("opener is not a comment: %v", toks)
			}
			toks, state2 := tokenize(t, l, mid, state)
			if state2 != state || len(toks) != 1 || toks[0].Kind != editor.TokenComment {
				t.Errorf("middle line = %v state %d, want one comment token and state %d", toks, state2, state)
			}
			toks, state3 := tokenize(t, l, closing, state2)
			if state3 != 0 {
				t.Errorf("state after closing line = %d, want 0", state3)
			}
			wantEnd := strings.Index(closing, tc.close) + len(tc.close)
			if toks[0].Kind != editor.TokenComment || toks[0].End != wantEnd {
				t.Errorf("closing line tokens = %v, want comment ending at %d", toks, wantEnd)
			}
		})
	}
}

func TestNestedBlockComment(t *testing.T) {
	l := ForName("a.hs")
	_, state := tokenize(t, l, "{- a {- b -} still", 0)
	if state == 0 {
		t.Fatal("nested comment closed early")
	}
	_, state = tokenize(t, l, "end -} done", state)
	if state != 0 {
		t.Fatalf("state = %d after the outer close, want 0", state)
	}
}

func TestMultilineString(t *testing.T) {
	tests := []struct{ file, open, mid, close string }{
		{"a.ex", `x = """`, "def end", `""" end`},
		{"a.clj", `(str "abc`, "def", `ghi" x`},
		{"a.groovy", "x = '''", "def", "''' y"},
		{"a.pl", `print "abc`, "my", `x";`},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			l := ForName(tc.file)
			_, state := tokenize(t, l, tc.open, 0)
			if state == 0 {
				t.Fatalf("string did not continue past %q", tc.open)
			}
			toks, state := tokenize(t, l, tc.mid, state)
			if state == 0 || len(toks) != 1 || toks[0].Kind != editor.TokenString {
				t.Errorf("middle = %v state %d", toks, state)
			}
			toks, state = tokenize(t, l, tc.close, state)
			if state != 0 || toks[0].Kind != editor.TokenString {
				t.Errorf("closing = %v state %d", toks, state)
			}
		})
	}
}

func TestSingleLineStringEndsAtLine(t *testing.T) {
	_, state := tokenize(t, ForName("a.gleam"), `let x = "unterminated`, 0)
	if state != 0 {
		t.Fatalf("state = %d, want 0", state)
	}
}

func TestCharLiteralVersusTypeVariable(t *testing.T) {
	l := ForName("a.ml")
	line := "let f (x : 'a) = 'c'"
	toks, _ := tokenize(t, l, line, 0)
	if kind, _ := kindOf(toks, line, "'c'"); kind != editor.TokenString {
		t.Errorf("'c' kind = %d, want string", kind)
	}
	if kind, _ := kindOf(toks, line, "'a"); kind == editor.TokenString {
		t.Errorf("'a is a string")
	}
}

func TestLineBasedLanguages(t *testing.T) {
	tests := []struct {
		file, line string
		state      uint64
		sub        string
		kind       editor.TokenKind
	}{
		{".env", "# note", 0, "# note", editor.TokenComment},
		{".env", `KEY="hi" # note`, 0, "KEY", editor.TokenProperty},
		{".env", `KEY="hi" # note`, 0, `"hi"`, editor.TokenString},
		{".env", `KEY="hi" # note`, 0, "# note", editor.TokenComment},
		{".env", "export KEY=value # note", 0, "export", editor.TokenKeyword},
		{".env", "KEY=value # note", 0, "# note", editor.TokenComment},
		{".gitignore", "# note", 0, "# note", editor.TokenComment},
		{".gitignore", "!keep", 0, "!", editor.TokenOperator},
		{".gitignore", "dist/", 0, "dist", editor.TokenText},
		{"COMMIT_EDITMSG", "fix: subject", 0, "subject", editor.TokenHeading},
		{"COMMIT_EDITMSG", "# Please enter", 0, "Please", editor.TokenComment},
		{"COMMIT_EDITMSG", "body text", 1, "body", editor.TokenText},
		{"COMMIT_EDITMSG", "Signed-off-by: A <a@b>", 1, "Signed-off-by", editor.TokenProperty},
		{"git-rebase-todo", "pick abc1234 subject", 0, "pick", editor.TokenKeyword},
		{"git-rebase-todo", "pick abc1234 subject", 0, "abc1234", editor.TokenConstant},
		{"git-rebase-todo", "pick abc1234 subject", 0, "subject", editor.TokenText},
		{"git-rebase-todo", "# Rebase", 0, "Rebase", editor.TokenComment},
		{"a.log", "2024-01-02T10:11:12Z ERROR failed 42", 0, "2024", editor.TokenConstant},
		{"a.log", "2024-01-02T10:11:12Z ERROR failed 42", 0, "ERROR", editor.TokenInvalid},
		{"a.log", "2024-01-02T10:11:12Z WARN x", 0, "WARN", editor.TokenMeta},
		{"a.log", "INFO started", 0, "INFO", editor.TokenFunction},
		{"a.log", "DEBUG started", 0, "DEBUG", editor.TokenVariable},
		{"a.log", "failed 42", 0, "42", editor.TokenNumber},
		{"a.log", "myERRORx", 0, "ERROR", editor.TokenText},
		{"a.rst", ".. note:: hi", 0, "note::", editor.TokenMeta},
		{"a.rst", ".. just a comment", 0, "comment", editor.TokenComment},
		{"a.rst", "=======", 0, "====", editor.TokenHeading},
		{"a.rst", "use ``code`` here", 0, "code", editor.TokenCode},
		{"a.csv", "a,b,c", 0, "b", editor.TokenString},
		{"a.csv", "a,b,c", 0, "a", editor.TokenText},
		{"a.tsv", "a\tb", 0, "b", editor.TokenString},
	}
	for _, tc := range tests {
		l := ForName(tc.file)
		toks, _ := tokenize(t, l, tc.line, tc.state)
		if got, ok := kindOf(toks, tc.line, tc.sub); !ok || got != tc.kind {
			t.Errorf("%s: %q in %q has kind %d, want %d (%v)", tc.file, tc.sub, tc.line, got, tc.kind, toks)
		}
	}
}

func TestCommitHeadingOnlyOnFirstLine(t *testing.T) {
	l := ForName("COMMIT_EDITMSG")
	_, state := tokenize(t, l, "subject", 0)
	if state == 0 {
		t.Fatal("state after subject is 0")
	}
	toks, _ := tokenize(t, l, "second", state)
	if toks[0].Kind != editor.TokenText {
		t.Errorf("second line kind = %d", toks[0].Kind)
	}
	_, state = tokenize(t, l, "# comment first", 0)
	if state != 0 {
		t.Errorf("comment line moved the state to %d", state)
	}
}

func TestCSVQuotedNewline(t *testing.T) {
	l := ForName("a.csv")
	_, state := tokenize(t, l, `a,"multi`, 0)
	if state&csvQuoteBit == 0 {
		t.Fatalf("state = %d, want quoted", state)
	}
	_, state = tokenize(t, l, `line",b`, state)
	if state&csvQuoteBit != 0 {
		t.Fatalf("state = %d, want unquoted", state)
	}
}

func TestTokenizersNeverPanic(t *testing.T) {
	rng := rand.New(rand.NewSource(fuzzSeed))
	for _, e := range entries {
		l := e.lang
		t.Run(l.Name, func(t *testing.T) {
			var state uint64
			for i := 0; i < fuzzLines; i++ {
				line := randomLine(rng)
				toks, next := l.Tokenizer.Tokenize(line, state, nil)
				checkTokens(t, l.Name, line, toks)
				if i%fuzzRandomStateEvery == 0 {
					state = rng.Uint64()
				} else {
					state = next
				}
			}
		})
	}
}

func randomLine(rng *rand.Rand) string {
	n := rng.Intn(fuzzMaxLen)
	b := make([]byte, 0, n)
	if rng.Intn(2) == 0 {
		for len(b) < n {
			b = append(b, byte(rng.Intn(256)))
		}
	} else {
		for len(b) < n {
			b = append(b, fuzzAlphabet[rng.Intn(len(fuzzAlphabet))])
		}
	}
	return strings.ReplaceAll(string(b), "\n", " ")
}
