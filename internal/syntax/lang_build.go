package syntax

import "github.com/egoist/mygo/ui/editor"

var cmake = &editor.Language{
	Name: "CMake", Extensions: []string{".cmake"}, Filenames: []string{"CMakeLists.txt"},
	LineComment: "#", BlockComment: [2]string{"#[[", "]]"}, Brackets: brackets[:2], AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`(?i)^\s*(if|foreach|while|function|macro|else|elseif)\s*\(`),
	DecreaseIndent: re(`(?i)^\s*(endif|endforeach|endwhile|endfunction|endmacro|else|elseif)\s*\(`),
	Tokenizer: &spec{
		lineComments: hash, blocks: []blockComment{{open: "#[[", close: "]]"}},
		strings: strs(dqMulti), foldCase: true,
		words: words(
			kw("if else elseif endif foreach endforeach while endwhile function endfunction macro endmacro return break continue set option include and or not strequal matches defined exists")),
		identChars: "-.", calls: true,
	},
}

var starlark = &editor.Language{
	Name: "Starlark", Extensions: []string{".bzl", ".star", ".bazel", ".sky"},
	Filenames:   []string{"BUILD", "BUILD.bazel", "WORKSPACE", "WORKSPACE.bazel", "MODULE.bazel"},
	LineComment: "#", Brackets: brackets, AutoClose: autoClose("\"", "'"), TabSize: 4,
	IncreaseIndent: re(`:\s*$`),
	DecreaseIndent: re(`^\s*(elif|else)\b`),
	Tokenizer: &spec{
		lineComments: hash, strings: strs(dqTriple, sqTriple, dq, sq),
		words: words(
			kw("and break continue def elif else for if in lambda load not or pass return"),
			consts("True False None"),
			funcs("print len range list dict tuple str int bool type sorted reversed enumerate zip any all min max select glob native")),
		calls: true,
	},
}

var prisma = &editor.Language{
	Name: "Prisma", Extensions: []string{".prisma"},
	LineComment: "//", Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`\{\s*$`),
	Tokenizer: &spec{
		lineComments: slash, strings: strs(dq),
		words: words(
			kw("model enum type datasource generator view"),
			types("String Boolean Int BigInt Float Decimal DateTime Json Bytes"),
			consts("true false null")),
		prefixes:     map[byte]editor.TokenKind{'@': editor.TokenMeta},
		capitalTypes: true, calls: true,
	},
}
