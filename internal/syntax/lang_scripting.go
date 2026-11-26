package syntax

import "github.com/egoist/mygo/ui/editor"

var julia = &editor.Language{
	Name: "Julia", Extensions: []string{".jl"}, Interpreters: []string{"julia"},
	LineComment: "#", BlockComment: [2]string{"#=", "=#"}, Brackets: brackets, AutoClose: autoClose("\""), TabSize: 4,
	IncreaseIndent: re(`^\s*(function|if|elseif|else|for|while|begin|struct|mutable struct|module|let|try|catch|finally|macro|quote)\b`),
	DecreaseIndent: re(`^\s*(end|else|elseif|catch|finally)\b`),
	Tokenizer: &spec{
		lineComments: hash, blocks: []blockComment{{open: "#=", close: "=#", nested: true}},
		strings: strs(dqTriple, dq, charLit),
		words: words(
			kw("abstract baremodule begin break catch const continue do else elseif end export finally for function global if import in isa let local macro module mutable primitive quote return struct try using where while"),
			consts("true false nothing missing Inf NaN pi")),
		prefixes:   map[byte]editor.TokenKind{'@': editor.TokenMeta},
		identChars: "!", capitalTypes: true, calls: true,
	},
}

var perl = &editor.Language{
	Name: "Perl", Extensions: []string{".pl", ".pm", ".pod", ".psgi"}, Interpreters: []string{"perl"},
	LineComment: "#", Brackets: brackets, AutoClose: autoClose("\"", "'"), TabSize: 4,
	Tokenizer: &spec{
		lineComments: hash, strings: strs(dqMulti, stringSpec{open: "'", close: "'", escape: '\\', multiline: true}),
		words: words(
			kw("my our local sub if elsif else unless while until for foreach do return last next redo package use no require BEGIN END qw qq q tr y s m and or not xor eq ne lt gt le ge cmp"),
			funcs("print say die warn defined undef ref bless wantarray push pop shift unshift split join map grep sort keys values open close")),
		prefixes: map[byte]editor.TokenKind{'$': editor.TokenVariable, '@': editor.TokenVariable, '%': editor.TokenVariable},
		calls:    true,
	},
}

var powershell = &editor.Language{
	Name: "PowerShell", Extensions: []string{".ps1", ".psm1", ".psd1"}, Interpreters: []string{"pwsh", "powershell"},
	LineComment: "#", BlockComment: [2]string{"<#", "#>"}, Brackets: brackets, AutoClose: autoClose("\"", "'"), TabSize: 4,
	IncreaseIndent: re(`\{\s*$`),
	Tokenizer: &spec{
		lineComments: hash, blocks: []blockComment{{open: "<#", close: "#>"}},
		strings: strs(stringSpec{open: "\"", close: "\"", escape: '`', multiline: true}, stringSpec{open: "'", close: "'", multiline: true}),
		words: words(
			kw("begin break catch class continue data do dynamicparam else elseif end enum exit filter finally for foreach from function if in param process return switch throw trap try until using var while workflow")),
		prefixes:   map[byte]editor.TokenKind{'$': editor.TokenVariable, '-': editor.TokenKeyword},
		identChars: "-", foldCase: true,
	},
}

var groovy = &editor.Language{
	Name: "Groovy", Extensions: []string{".groovy", ".gradle", ".gvy"}, Filenames: []string{"Jenkinsfile"}, Interpreters: []string{"groovy"},
	LineComment: "//", BlockComment: [2]string{"/*", "*/"}, Brackets: brackets, AutoClose: autoClose("\"", "'"), TabSize: 4,
	Tokenizer: &spec{
		lineComments: slash, blocks: cBlock,
		strings: strs(dqTriple, sqTriple, dq, sq),
		words: words(
			kw("abstract as assert boolean break byte case catch char class const continue def default do double else enum extends final finally float for goto if implements import in instanceof int interface long native new package private protected public return short static strictfp super switch synchronized this threadsafe throw throws trait transient try var void volatile while"),
			consts("true false null")),
		prefixes:     map[byte]editor.TokenKind{'@': editor.TokenMeta},
		capitalTypes: true, calls: true,
	},
}

var nim = &editor.Language{
	Name: "Nim", Extensions: []string{".nim", ".nims", ".nimble"}, Interpreters: []string{"nim"},
	LineComment: "#", BlockComment: [2]string{"#[", "]#"}, Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`(:|=)\s*$`),
	Tokenizer: &spec{
		lineComments: hash, blocks: []blockComment{{open: "#[", close: "]#", nested: true}},
		strings: strs(dqTriple, dq, charLit),
		words: words(
			kw("addr and as asm bind block break case cast concept const continue converter defer discard distinct div do elif else end enum except export finally for from func if import in include interface is isnot iterator let macro method mixin mod nil not notin object of or out proc ptr raise ref return shl shr static template try tuple type using var when while xor yield"),
			types("int int8 int16 int32 int64 uint uint8 uint16 uint32 uint64 float float32 float64 bool char string seq array set openArray cstring"),
			consts("true false nil")),
		capitalTypes: true, calls: true,
	},
}

var crystal = &editor.Language{
	Name: "Crystal", Extensions: []string{".cr"}, Interpreters: []string{"crystal"},
	LineComment: "#", Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`^\s*(def|class|module|struct|enum|if|unless|while|until|case|begin|macro|lib|annotation)\b|\bdo(\s*\|.*\|)?\s*$`),
	DecreaseIndent: re(`^\s*(end|else|elsif|when|rescue|ensure)\b`),
	Tokenizer: &spec{
		lineComments: hash, strings: strs(dqMulti, charLit, stringSpec{open: "`", close: "`", escape: '\\', multiline: true}),
		words: words(
			kw("abstract alias annotation as begin break case class def do else elsif end ensure enum extend for fun if in include lib macro module next of out private protected require rescue return select self sizeof struct super then type typeof union uninitialized unless until verbatim when while with yield"),
			consts("true false nil")),
		prefixes:     map[byte]editor.TokenKind{'@': editor.TokenVariable, ':': editor.TokenConstant},
		identChars:   "?!",
		capitalTypes: true, calls: true,
	},
}

var tcl = &editor.Language{
	Name: "Tcl", Extensions: []string{".tcl", ".tk"}, Interpreters: []string{"tclsh", "wish"},
	LineComment: "#", Brackets: brackets, AutoClose: autoClose("\""), TabSize: 4,
	Tokenizer: &spec{
		lineComments: hash, strings: strs(dq),
		words: words(
			kw("after append array break catch cd close continue dict else elseif error eval exec exit expr file for foreach format global if incr info join lappend lindex list llength lrange lreplace lsearch lset lsort namespace package proc puts read regexp regsub rename return scan set split string subst switch then trace unset uplevel upvar variable vwait while")),
		prefixes: map[byte]editor.TokenKind{'$': editor.TokenVariable},
	},
}

var awk = &editor.Language{
	Name: "Awk", Extensions: []string{".awk", ".gawk"}, Interpreters: []string{"awk", "gawk", "mawk", "nawk"},
	LineComment: "#", Brackets: brackets, AutoClose: autoClose("\""), TabSize: 4,
	Tokenizer: &spec{
		lineComments: hash, strings: strs(dq),
		words: words(
			kw("BEGIN END BEGINFILE ENDFILE break continue delete do else exit for function func getline if in next nextfile print printf return while"),
			funcs("length substr index split sub gsub match sprintf sin cos atan2 exp log sqrt int rand srand tolower toupper system close fflush"),
			consts("NR NF FS OFS RS ORS FILENAME FNR SUBSEP RSTART RLENGTH ENVIRON ARGC ARGV CONVFMT OFMT")),
		prefixes: map[byte]editor.TokenKind{'$': editor.TokenVariable},
	},
}

var batch = &editor.Language{
	Name: "Batch", Extensions: []string{".bat", ".cmd"},
	LineComment: "REM ", Brackets: brackets, AutoClose: autoClose("\""), TabSize: 4,
	IncreaseIndent: re(`\(\s*$`),
	DecreaseIndent: re(`^\s*\)`),
	Tokenizer: &spec{
		lineComments: []string{"::", "REM ", "rem ", "Rem "}, strings: strs(rawDQ),
		words: words(
			kw("if else not exist defined goto call set setlocal endlocal echo exit for in do pause rem shift start cd md rd del copy move type cls errorlevel equ neq lss leq gtr geq")),
		prefixes: map[byte]editor.TokenKind{'@': editor.TokenMeta, '%': editor.TokenVariable, ':': editor.TokenMeta},
		foldCase: true,
	},
}
