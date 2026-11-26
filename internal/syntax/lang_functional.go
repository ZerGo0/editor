package syntax

import "github.com/egoist/mygo/ui/editor"

var elixir = &editor.Language{
	Name: "Elixir", Extensions: []string{".ex", ".exs"}, Interpreters: []string{"elixir"},
	LineComment: "#", Brackets: brackets, AutoClose: autoClose("\"", "'"), TabSize: 2,
	IncreaseIndent: re(`(\bdo|->)\s*$`),
	DecreaseIndent: re(`^\s*(end|else|rescue|catch|after)\b`),
	Tokenizer: &spec{
		lineComments: hash, strings: strs(dqTriple, dq, sq),
		words: words(
			kw("def defp defmodule defmacro defmacrop defstruct defprotocol defimpl defdelegate defguard defexception do end fn if else unless cond case when with for receive try rescue catch after raise throw import require use alias quote unquote in and or not"),
			consts("true false nil")),
		prefixes:     map[byte]editor.TokenKind{'@': editor.TokenMeta, ':': editor.TokenConstant},
		identChars:   "?!",
		capitalTypes: true, calls: true,
	},
}

var erlang = &editor.Language{
	Name: "Erlang", Extensions: []string{".erl", ".hrl", ".escript"}, Interpreters: []string{"escript"},
	LineComment: "%", Brackets: brackets, AutoClose: autoClose("\""), TabSize: 4,
	IncreaseIndent: re(`(->|\bof|\bbegin)\s*$`),
	DecreaseIndent: re(`^\s*(end|after|catch)\b`),
	Tokenizer: &spec{
		lineComments: percent, strings: strs(dq, sq),
		words: words(
			kw("after and andalso band begin bnot bor bsl bsr bxor case catch cond div end fun if let not of or orelse receive rem try when xor"),
			consts("true false undefined ok error")),
		calls: true,
	},
}

var haskell = &editor.Language{
	Name: "Haskell", Extensions: []string{".hs", ".lhs", ".hsc"}, Interpreters: []string{"runhaskell", "runghc", "stack"},
	LineComment: "--", BlockComment: [2]string{"{-", "-}"}, Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`(\bwhere|\bdo|\bof|=)\s*$`),
	Tokenizer: &spec{
		lineComments: dash, blocks: []blockComment{{open: "{-", close: "-}", nested: true}},
		strings: strs(dq, charLit),
		words: words(
			kw("case class data default deriving do else family forall foreign if import in infix infixl infixr instance let module newtype of qualified hiding then type where as mdo proc rec"),
			consts("True False Nothing")),
		identChars: "'", capitalTypes: true,
	},
}

var ocaml = &editor.Language{
	Name: "OCaml", Extensions: []string{".ml", ".mli", ".mll", ".mly"}, Interpreters: []string{"ocaml", "ocamlrun"},
	BlockComment: [2]string{"(*", "*)"}, Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`(=|->|\bbegin|\bstruct|\bsig|\bthen|\belse|\bdo)\s*$`),
	DecreaseIndent: re(`^\s*(end|done|else|in)\b`),
	Tokenizer: &spec{
		blocks:  []blockComment{{open: "(*", close: "*)", nested: true}},
		strings: strs(dq, charLit),
		words: words(
			kw("and as assert begin class constraint do done downto else end exception external for fun function functor if in include inherit initializer lazy let match method module mutable new nonrec object of open or private rec sig struct then to try type val virtual when while with"),
			types("int float string bool char unit list option array ref exn bytes"),
			consts("true false")),
		identChars: "'", capitalTypes: true,
	},
}

var fsharp = &editor.Language{
	Name: "F#", Extensions: []string{".fs", ".fsi", ".fsx", ".fsscript"},
	LineComment: "//", BlockComment: [2]string{"(*", "*)"}, Brackets: brackets, AutoClose: autoClose("\""), TabSize: 4,
	IncreaseIndent: re(`(=|->|\bthen|\belse|\bdo|\bwith|\bbegin|\bstruct)\s*$`),
	Tokenizer: &spec{
		lineComments: slash, blocks: []blockComment{{open: "(*", close: "*)", nested: true}},
		strings: strs(dqTriple, dq, charLit),
		words: words(
			kw("abstract and as assert base begin class default delegate do done downcast downto elif else end exception extern finally fixed for fun function global if in inherit inline interface internal lazy let match member module mutable namespace new not of open or override private public rec return sig static struct then to try type upcast use val void when while with yield"),
			types("int float string bool char unit list option array byte int64 decimal obj seq"),
			consts("true false null")),
		identChars: "'", capitalTypes: true,
	},
}

var clojure = &editor.Language{
	Name: "Clojure", Extensions: []string{".clj", ".cljs", ".cljc", ".edn", ".bb"}, Interpreters: []string{"clojure", "bb"},
	LineComment: ";", Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	Tokenizer: &spec{
		lineComments: semicolon, strings: strs(dqMulti),
		words: words(
			kw("def defn defn- defmacro defmulti defmethod defprotocol defrecord deftype defonce ns let letfn fn if if-let if-not when when-let when-not cond case do loop recur try catch finally throw quote require use import for doseq dotimes and or not"),
			consts("true false nil")),
		prefixes:   map[byte]editor.TokenKind{':': editor.TokenConstant},
		identStart: "-+*/<>=!?%&.@^~$", identChars: "-+*/<>=!?%&.@^~$:#'",
		parenCalls: true,
	},
}

var lisp = &editor.Language{
	Name: "Lisp", Extensions: []string{".lisp", ".lsp", ".cl", ".el", ".scm", ".ss", ".sld", ".rkt"},
	Interpreters: []string{"guile", "racket", "sbcl", "clisp", "scheme", "csi", "emacs"},
	LineComment:  ";", BlockComment: [2]string{"#|", "|#"}, Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	Tokenizer: &spec{
		lineComments: semicolon, blocks: []blockComment{{open: "#|", close: "|#", nested: true}},
		strings: strs(dqMulti),
		words: words(
			kw("define defun defvar defparameter defmacro defstruct defclass defmethod defgeneric defconstant define-syntax define-record-type lambda let let* letrec letrec* labels flet if cond case when unless do dolist dotimes loop progn begin set! setq setf quote quasiquote unquote and or not require provide module import export library in-package use-package declare"),
			consts("t nil #t #f true false")),
		prefixes:   map[byte]editor.TokenKind{':': editor.TokenConstant},
		identStart: "-+*/<>=!?%&^~$@#.", identChars: "-+*/<>=!?%&^~$@:.#",
		parenCalls: true,
	},
}

var elm = &editor.Language{
	Name: "Elm", Extensions: []string{".elm"},
	LineComment: "--", BlockComment: [2]string{"{-", "-}"}, Brackets: brackets, AutoClose: autoClose("\""), TabSize: 4,
	IncreaseIndent: re(`(=|->|\bof|\bthen|\belse|\blet)\s*$`),
	Tokenizer: &spec{
		lineComments: dash, blocks: []blockComment{{open: "{-", close: "-}", nested: true}},
		strings: strs(dqTriple, dq, charLit),
		words: words(
			kw("alias as case else exposing if import in let module of port then type where"),
			consts("True False")),
		identChars: "'", capitalTypes: true,
	},
}

var gleam = &editor.Language{
	Name: "Gleam", Extensions: []string{".gleam"},
	LineComment: "//", Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`(\{|->)\s*$`),
	Tokenizer: &spec{
		lineComments: slash, strings: strs(dq),
		words: words(
			kw("as assert auto case const delegate derive echo else fn if implement import let macro opaque panic pub test todo type use"),
			consts("True False Nil")),
		capitalTypes: true, calls: true,
	},
}
