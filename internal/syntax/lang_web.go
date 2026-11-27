package syntax

import "github.com/egoist/mygo/ui/editor"

func derive(name string, tokenizer editor.Tokenizer, mutate func(*editor.Language)) *editor.Language {
	lang := *editor.LanguageNamed(name)
	lang.Tokenizer = tokenizer
	if mutate != nil {
		mutate(&lang)
	}
	return &lang
}

var cssTokenizer = editor.LanguageNamed("CSS").Tokenizer

var htmlMarkup = &markup{flavor: flavorHTML, script: &ecma{}, style: cssTokenizer}

var html = derive("HTML", htmlMarkup, nil)

var svelte = derive("Svelte", &markup{flavor: flavorSvelte, script: &ecma{}, expr: &ecma{}, style: cssTokenizer}, nil)

var astro = derive("HTML", &markup{flavor: flavorAstro, script: &ecma{}, expr: &ecma{jsx: true}, style: cssTokenizer}, func(l *editor.Language) {
	l.Name, l.Extensions = "Astro", []string{".astro"}
})

var goLanguage = derive("Go", golang{}, nil)

var sql = derive("SQL", sqlLexer{}, nil)

var markdownBlock = editor.LanguageNamed("Markdown").Tokenizer

func newMarkdown(mdx bool) *markdown {
	return &markdown{mdx: mdx, block: markdownBlock, esm: &ecma{jsx: true}, jsx: &markup{flavor: flavorMDX, script: &ecma{}, expr: &ecma{jsx: true}, style: cssTokenizer}}
}

var markdownLang = derive("Markdown", newMarkdown(false), func(l *editor.Language) {
	l.Extensions = []string{".md", ".markdown", ".mkd"}
})

var mdx = derive("Markdown", newMarkdown(true), func(l *editor.Language) {
	l.Name, l.Extensions, l.Filenames = "MDX", []string{".mdx"}, nil
})

var goMod = &editor.Language{
	Name: "Go Module", Filenames: []string{"go.mod", "go.sum", "go.work", "go.work.sum"}, LineComment: "//",
	Brackets: brackets[:1], AutoClose: autoClose("\""), TabSize: 4, HardTabs: true, Tokenizer: goModLexer{},
}

var gettext = &editor.Language{
	Name: "Gettext", Extensions: []string{".po", ".pot"}, LineComment: "#",
	AutoClose: autoClose("\""), TabSize: 2, Tokenizer: gettextLexer{},
}

var json = derive("JSON", jsonLexer{}, func(l *editor.Language) {
	l.Extensions = append(append([]string{}, l.Extensions...), ".code-workspace", ".jsonl", ".har")
	l.Filenames = append(append([]string{}, l.Filenames...), ".prettierrc", ".swcrc", ".stylelintrc", ".markdownlintrc", ".lintstagedrc", ".babelrc.json", "jsconfig.json", "deno.lock", ".watchmanconfig", ".releaserc")
})

var dockerfile = derive("Dockerfile", dockerfileLexer{}, func(l *editor.Language) {
	l.Extensions = []string{".dockerfile"}
	l.Filenames = []string{"Dockerfile", "Containerfile"}
})

var javascript = derive("JavaScript", &ecma{jsx: true}, nil)

var typescript = derive("TypeScript", &ecma{}, func(l *editor.Language) {
	l.Extensions = []string{".ts", ".mts", ".cts"}
})

var typescriptReact = derive("TypeScript", &ecma{jsx: true}, func(l *editor.Language) {
	l.Name, l.Extensions, l.Interpreters = "TypeScript React", []string{".tsx"}, nil
})
