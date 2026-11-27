package syntax

import (
	"path"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui/editor"
)

type entry struct {
	lang           *editor.Language
	replaces       bool
	filenamePrefix string
	aliases        []string
}

var entries = []entry{
	{lang: javascript, replaces: true}, {lang: typescript, replaces: true, aliases: []string{"ts"}}, {lang: typescriptReact, replaces: true, aliases: []string{"tsx"}},
	{lang: html, replaces: true}, {lang: svelte, replaces: true}, {lang: astro}, {lang: goLanguage, replaces: true, aliases: []string{"golang"}},
	{lang: sql, replaces: true, aliases: []string{"postgres", "postgresql", "psql", "pgsql", "plpgsql"}},
	{lang: markdownLang, replaces: true}, {lang: mdx, replaces: true}, {lang: json, replaces: true, aliases: []string{"jsonc", "json5"}},
	{lang: dockerfile, replaces: true, filenamePrefix: "Dockerfile.", aliases: []string{"docker"}},
	{lang: goMod, aliases: []string{"go.mod", "gomod"}}, {lang: gettext, aliases: []string{"po"}},
	{lang: elixir}, {lang: erlang}, {lang: haskell}, {lang: ocaml}, {lang: fsharp, aliases: []string{"fsharp"}},
	{lang: clojure}, {lang: lisp, aliases: []string{"scheme", "racket", "elisp", "commonlisp"}},
	{lang: julia}, {lang: perl}, {lang: powershell, aliases: []string{"pwsh"}}, {lang: groovy}, {lang: elm},
	{lang: nim}, {lang: crystal}, {lang: dlang, aliases: []string{"dlang"}}, {lang: fortran}, {lang: pascal},
	{lang: assembly, aliases: []string{"asm"}}, {lang: cmake}, {lang: starlark, aliases: []string{"bazel"}},
	{lang: gleam}, {lang: solidity}, {lang: prisma}, {lang: verilog}, {lang: vhdl},
	{lang: latex, aliases: []string{"tex"}}, {lang: restructuredText, aliases: []string{"rst"}},
	{lang: batch, aliases: []string{"bat", "cmd"}}, {lang: tcl}, {lang: awk},
	{lang: dotenv, filenamePrefix: ".env.", aliases: []string{"env"}}, {lang: ignoreFile, aliases: []string{"gitignore"}},
	{lang: gitCommit, aliases: []string{"gitcommit"}}, {lang: gitRebase, aliases: []string{"gitrebase"}},
	{lang: csv}, {lang: tsv}, {lang: logFile, aliases: []string{"logs"}},
}

type index struct {
	byFilename map[string]*editor.Language
	byExt      map[string]*editor.Language
	byInterp   map[string]*editor.Language
	byName     map[string]*editor.Language
}

var lookup = sync.OnceValue(func() *index {
	ix := &index{
		byFilename: map[string]*editor.Language{}, byExt: map[string]*editor.Language{},
		byInterp: map[string]*editor.Language{}, byName: map[string]*editor.Language{},
	}
	setFirst := func(m map[string]*editor.Language, key string, l *editor.Language) {
		if _, taken := m[key]; !taken {
			m[key] = l
		}
	}
	for _, e := range entries {
		l := e.lang
		setFirst(ix.byName, strings.ToLower(l.Name), l)
		for _, a := range e.aliases {
			setFirst(ix.byName, a, l)
		}
		for _, f := range l.Filenames {
			setFirst(ix.byFilename, f, l)
		}
		for _, x := range l.Extensions {
			setFirst(ix.byExt, x, l)
			setFirst(ix.byName, strings.TrimPrefix(x, "."), l)
		}
		for _, i := range l.Interpreters {
			setFirst(ix.byInterp, i, l)
		}
	}
	return ix
})

func extraByName(filename string) *editor.Language {
	base := path.Base(strings.ReplaceAll(filename, "\\", "/"))
	ix := lookup()
	if l, ok := ix.byFilename[base]; ok {
		return l
	}
	for _, e := range entries {
		if e.filenamePrefix != "" && strings.HasPrefix(base, e.filenamePrefix) {
			return e.lang
		}
	}
	return ix.byExt[strings.ToLower(path.Ext(base))]
}

func shebangInterpreter(text string) string {
	line, ok := strings.CutPrefix(text, "#!")
	if !ok {
		return ""
	}
	line, _, _ = strings.Cut(line, "\n")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	name := path.Base(fields[0])
	if name == "env" {
		name = ""
		for _, f := range fields[1:] {
			if !strings.HasPrefix(f, "-") && !strings.Contains(f, "=") {
				name = path.Base(f)
				break
			}
		}
	}
	return strings.TrimRight(name, "0123456789.")
}

func extraByShebang(text string) *editor.Language {
	name := shebangInterpreter(text)
	if name == "" {
		return nil
	}
	return lookup().byInterp[name]
}

func For(filename, text string) *editor.Language {
	if l := extraByName(filename); l != nil {
		return l
	}
	if editor.LanguageFor(filename) == nil {
		if l := extraByShebang(text); l != nil {
			return l
		}
	}
	return editor.LanguageForText(filename, text)
}

func ForName(filename string) *editor.Language {
	if l := extraByName(filename); l != nil {
		return l
	}
	return editor.LanguageFor(filename)
}

func Named(name string) *editor.Language {
	key := strings.ToLower(strings.TrimSpace(name))
	if l, ok := lookup().byName[key]; ok {
		return l
	}
	return editor.LanguageNamed(name)
}
