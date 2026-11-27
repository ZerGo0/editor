package syntax

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

type want struct {
	text string
	kind editor.TokenKind
	nth  int
}

func w(text string, kind editor.TokenKind) want { return want{text: text, kind: kind} }

func wn(text string, nth int, kind editor.TokenKind) want {
	return want{text: text, kind: kind, nth: nth}
}

type highlighted struct {
	lang  *editor.Language
	kinds []editor.TokenKind
	state uint64
}

func highlight(t *testing.T, file, src string) highlighted {
	t.Helper()
	lang := For(file, src)
	if lang == nil || lang.Tokenizer == nil {
		t.Fatalf("no language for %q", file)
	}
	out := highlighted{lang: lang, kinds: make([]editor.TokenKind, len(src))}
	offset := 0
	for _, line := range strings.SplitAfter(src, "\n") {
		text := strings.TrimSuffix(line, "\n")
		toks, next := tokenize(t, lang, text, out.state)
		start := 0
		for _, tk := range toks {
			for k := start; k < tk.End; k++ {
				out.kinds[offset+k] = tk.Kind
			}
			start = tk.End
		}
		out.state = next
		offset += len(line)
	}
	return out
}

func (h highlighted) probe(t *testing.T) editor.TokenKind {
	t.Helper()
	toks, _ := tokenize(t, h.lang, "zzz", h.state)
	return toks[0].Kind
}

func checkWants(t *testing.T, file, src string, wants []want) {
	t.Helper()
	h := highlight(t, file, src)
	for _, wt := range wants {
		at, from := -1, 0
		for n := 0; n <= wt.nth; n++ {
			k := strings.Index(src[from:], wt.text)
			if k < 0 {
				t.Errorf("%s: %q occurrence %d not in the fixture", file, wt.text, wt.nth)
				at = -1
				break
			}
			at = from + k
			from = at + len(wt.text)
		}
		if at < 0 {
			continue
		}
		for k := at; k < at+len(wt.text); k++ {
			if c := src[k]; c == ' ' || c == '\t' || c == '\n' {
				continue
			}
			if h.kinds[k] != wt.kind {
				t.Errorf("%s: %q (occurrence %d) byte %q is kind %d, want %d", file, wt.text, wt.nth, src[k], h.kinds[k], wt.kind)
				break
			}
		}
	}
}

const (
	tKeyword   = editor.TokenKeyword
	tType      = editor.TokenType
	tFunction  = editor.TokenFunction
	tVariable  = editor.TokenVariable
	tProperty  = editor.TokenProperty
	tConstant  = editor.TokenConstant
	tNumber    = editor.TokenNumber
	tString    = editor.TokenString
	tEscape    = editor.TokenEscape
	tRegexp    = editor.TokenRegexp
	tComment   = editor.TokenComment
	tTag       = editor.TokenTag
	tAttribute = editor.TokenAttribute
	tMeta      = editor.TokenMeta
	tPunct     = editor.TokenPunctuation
	tOperator  = editor.TokenOperator
)

func TestHighlightECMA(t *testing.T) {
	tests := []struct {
		name, file, src string
		wants           []want
	}{
		{"typescript declarations", "a.ts", "import type { Foo } from \"./foo\";\n" +
			"interface Props<T extends object = {}> extends Base {\n  name: string;\n  onPick(item: T): void;\n}\n" +
			"type Mode = \"a\" | \"b\"\n  | { kind: number };\n" +
			"@Injectable()\nexport class Store<T> extends Base implements Api {\n  private count: number = 0;\n" +
			"  async load(id: string): Promise<Foo | undefined> {\n    return this.client.get<Foo>(id);\n  }\n}\n",
			[]want{w("import", tKeyword), w("Foo", tType), w("interface", tKeyword), w("Props", tType), w("extends", tKeyword), w("name", tProperty),
				w("string", tType), w("onPick", tFunction), w("item", tVariable), w("void", tType), w("\"a\"", tString), w("number", tType),
				w("@Injectable", tMeta), w("class", tKeyword), w("implements", tKeyword), w("Api", tType), w("private", tKeyword), w("count", tProperty),
				w("async", tKeyword), w("load", tFunction), w("Promise", tType), w("undefined", tConstant), w("this", tKeyword),
				w("client", tProperty), w("get", tFunction), w("kind", tProperty), wn("number", 0, tType), w("0", tNumber)}},
		{"expressions", "a.ts", "const m = new Map<string, number>();\nconst ok = /ab+c/gi.test(url) ? (x as Foo).bar! : null;\n" +
			"const f = (a: number): string => a.toString();\nconst o = { key: 1, other() {}, [dyn]: 2 };\n" +
			"switch (v) { case 1: { break; } default: break; }\n",
			[]want{w("new", tKeyword), w("Map", tType), wn("string", 0, tType), w("/ab+c/gi", tRegexp), w("test", tFunction), w("as", tKeyword),
				w("Foo", tType), w("bar", tProperty), w("null", tConstant), w("f", tFunction), wn("number", 1, tType), w("toString", tFunction),
				w("key", tProperty), w("other", tFunction), w("case", tKeyword), w("default", tKeyword)}},
		{"template literals", "a.ts", "const u = `/a/${id}/${list.map((x) => `<${x}>`).join(\",\")}\\n`;\n",
			[]want{w("/a/", tString), wn("${", 0, tEscape), w("id", tVariable), wn("}", 0, tEscape), w("map", tFunction), wn("${", 2, tEscape),
				wn("<", 0, tString), w("join", tFunction), w("\\n", tEscape)}},
		{"runes and generics in modules", "a.svelte.ts", "let count = $state<number>(0);\nconst twice = $derived(count * 2);\n",
			[]want{w("$state", tKeyword), w("number", tType), w("$derived", tKeyword), w("count", tVariable)}},
		{"jsx", "a.tsx", "const el = <div className=\"a\" onClick={() => go(1)}>text {name}<Foo.Bar id={1} /></div>;\n",
			[]want{wn("div", 0, tTag), w("className", tAttribute), w("\"a\"", tString), w("go", tFunction), w("text", editor.TokenText), w("Foo.Bar", tType),
				w("id", tAttribute), wn("div", 1, tTag)}},
		{"comments", "a.js", "// line\n/* block\n still */ const a = 1;\n",
			[]want{w("// line", tComment), w("/* block", tComment), w("still */", tComment), w("const", tKeyword), w("1", tNumber)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { checkWants(t, tc.file, tc.src, tc.wants) })
	}
}

func TestHighlightFilesEndingInsideConstructs(t *testing.T) {
	tests := []struct {
		name, file, src string
		want            editor.TokenKind
	}{
		{"ts template", "a.ts", "const a = `open\n${b}\nstill", tString},
		{"ts template interpolation", "a.ts", "const a = `open ${b + `", tString},
		{"ts block comment", "a.ts", "/* open\n", tComment},
		{"tsx template", "a.tsx", "const a = <p>{`open", tString},
		{"go raw string", "a.go", "var s = `open\n", tString},
		{"go block comment", "a.go", "/* open\n", tComment},
		{"sql block comment", "a.sql", "/* open /* nested */\n", tComment},
		{"sql string", "a.sql", "select 'open\n", tString},
		{"sql dollar string", "a.sql", "comment on table t is $q$open\n", tString},
		{"json comment", "a.jsonc", "{\n/* open\n", tComment},
		{"html comment", "a.html", "<p>\n<!-- open\n", tComment},
		{"svelte comment", "a.svelte", "<script>\nlet a = 1;\n</script>\n<!-- open", tComment},
		{"svelte script template", "a.svelte", "<script lang=\"ts\">\nconst a = `open\n", tString},
		{"svelte expression template", "a.svelte", "<p>{`open\n", tString},
		{"astro frontmatter comment", "a.astro", "---\nconst a = 1;\n/* open\n", tComment},
		{"markdown fenced template", "a.md", "```ts\nconst a = `open\n", tString},
		{"mdx esm template", "a.mdx", "export const a = `open\n", tString},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := highlight(t, tc.file, tc.src).probe(t); got != tc.want {
				t.Fatalf("a line after the end of the file is kind %d, want %d", got, tc.want)
			}
		})
	}
}

func TestHighlightSvelte(t *testing.T) {
	src := "<script lang=\"ts\">\n  let { a, b = 1 }: { a: string; b?: number } = $props();\n  let n = $state(0);\n</script>\n\n" +
		"<!-- note -->\n<Foo {a} on:click={() => n++} bind:value class:on={n > 1} use:tip />\n" +
		"{#if n > 1 && a}\n  <p class=\"x {a} y\">{n} {@html a}</p>\n{:else}\n  text\n{/if}\n" +
		"{#each items as item, i}\n  <li onclick={() => {\n    n = i;\n  }}>{item.name}</li>\n{/each}\n" +
		"{@render children?.()}\n<style lang=\"scss\">\n  .a { color: red; margin: 0 4px; }\n</style>\n"
	checkWants(t, "a.svelte", src, []want{w("script", tTag), w("lang", tAttribute), w("\"ts\"", tString), w("$props", tKeyword), w("$state", tKeyword),
		wn("string", 0, tType), w("number", tType), w("let", tKeyword), w("<!-- note -->", tComment), w("Foo", tType), w("on", tKeyword), w("click", tAttribute),
		w("bind", tKeyword), w("class", tKeyword), w("use", tKeyword), w("tip", tAttribute), w("#if", tKeyword), w(":else", tKeyword), w("/if", tKeyword), w("#each", tKeyword),
		wn("as", 0, tKeyword), w("item", tVariable), w("@html", tKeyword), w("@render", tKeyword), w("children", tVariable), w("onclick", tAttribute), w("name", tProperty), w("style", tTag), w("color", tProperty), w("red", tConstant), w("4px", tNumber)})
}

func TestHighlightAstro(t *testing.T) {
	src := "---\nimport Layout from \"../Layout.astro\";\ninterface Props { title: string }\nconst { title } = Astro.props as Props;\n---\n" +
		"<Layout title={title} class:list={[\"a\"]}>\n  <h1 class=\"t\">{title}</h1>\n  {posts.map((p) => <li>{p.title}</li>)}\n</Layout>\n" +
		"<script>\n  const el = document.querySelector<HTMLElement>(\"#x\");\n</script>\n<style>\n  h1 { color: red; }\n</style>\n"
	checkWants(t, "a.astro", src, []want{wn("---", 0, tMeta), wn("---", 1, tMeta), w("import", tKeyword), w("interface", tKeyword), wn("string", 0, tType),
		w("Props", tType), w("Layout", tType), w("class", tKeyword), w("list", tAttribute), wn("h1", 0, tTag), w("map", tFunction), wn("li", 1, tTag), w("querySelector", tFunction),
		w("HTMLElement", tType), w("color", tProperty)})
}

func TestHighlightGo(t *testing.T) {
	src := "package main\n\n// Server serves.\ntype Server struct {\n\tName string `json:\"name,omitempty\"`\n\tport, id int\n\t*Base\n\tEntries map[string][]Entry\n}\n\n" +
		"type Handler interface {\n\tRun(ctx context.Context) error\n}\n\n" +
		"func (s *Server) Handle(w http.ResponseWriter, r *http.Request) (int, error) {\n" +
		"\titems := []Entry{{Name: \"a\"}}\n\tfmt.Printf(\"%s: %d%%\\n\", len(items), 1)\n\tvar u User\n" +
		"\tif v, ok := x.(*Claims); ok && v != nil {\n\t\treturn 0, nil\n\t}\n\treturn 1, errors.New(`raw\nstring`)\n}\n"
	checkWants(t, "a.go", src, []want{w("package", tKeyword), wn("main", 0, tType), w("// Server serves.", tComment), wn("Server", 1, tType), w("struct", tKeyword),
		wn("Name", 0, tProperty), wn("string", 0, tType), w("json", tAttribute), w("\"name,omitempty\"", tString), w("port", tProperty), w("id", tProperty), w("Base", tType),
		w("Entries", tProperty), w("Entry", tType), w("Run", tFunction), w("ctx", tVariable), w("Context", tType), w("error", tType), wn("Handle", 1, tFunction),
		w("ResponseWriter", tType), w("Request", tType), w("int", tType), wn("Name", 1, tProperty), w("Printf", tFunction),
		w("%s", tEscape), w("%d", tEscape), w("%%", tEscape), w("\\n", tEscape), w("len", tFunction), wn("User", 0, tType), w("Claims", tType), w("nil", tConstant),
		w("return", tKeyword), w("New", tFunction), w("raw", tString), w("string`", tString)})
}

func TestHighlightSQL(t *testing.T) {
	src := "-- users\nCREATE TABLE IF NOT EXISTS public.users (\n  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),\n  data jsonb DEFAULT '{}'::jsonb, /* a /* b */ c */\n  at timestamptz NOT NULL DEFAULT now()\n);\n" +
		"CREATE FUNCTION f(a int) RETURNS int AS $$\nBEGIN\n  RETURN a + 1;\nEND;\n$$ LANGUAGE plpgsql;\n" +
		"COMMENT ON TABLE users IS $q$it's $$ fine$q$;\nSELECT u.id, count(*) FROM users u WHERE u.id = $1 AND k = E'a\\nb' LIMIT 10;\n"
	checkWants(t, "a.sql", src, []want{w("-- users", tComment), w("CREATE", tKeyword), w("public", tVariable), wn("users", 1, tProperty), w("uuid", tType), w("PRIMARY", tKeyword),
		w("gen_random_uuid", tFunction), wn("jsonb", 1, tType), w("/* a /* b */ c */", tComment), w("timestamptz", tType), w("NULL", tConstant), w("now", tFunction),
		wn("$$", 0, tMeta), wn("BEGIN", 0, tKeyword), w("RETURN", tKeyword), w("LANGUAGE", tKeyword), w("it's $$ fine", tString), w("count", tFunction), w("$1", tVariable),
		w("\\n", tEscape), w("10", tNumber)})
}

func TestHighlightMarkdown(t *testing.T) {
	src := "---\ntitle: \"Hi\"\ntags: [a]\n---\n# Title\n\n```ts\nconst x: number = 1; // c\n```\n\n```go\nfunc main() { fmt.Println(\"x\") }\n```\n\n~~~sql\nselect now();\n~~~\n"
	checkWants(t, "a.md", src, []want{wn("---", 0, tMeta), w("title", tProperty), w("\"Hi\"", tString), w("# Title", editor.TokenHeading), w("const", tKeyword),
		w("number", tType), w("// c", tComment), w("func", tKeyword), w("main", tFunction), w("Println", tFunction), w("select", tKeyword), w("now", tFunction)})
	mdx := "import { Callout } from \"./c\"\nexport const meta = {\n  answer: 1,\n}\n\n# Hi\n\n<Callout type=\"info\">\n  Hello {meta.a}\n</Callout>\n"
	checkWants(t, "a.mdx", mdx, []want{w("import", tKeyword), w("Callout", tType), w("export", tKeyword), w("answer", tProperty), wn("Callout", 2, tType), w("type", tAttribute), w("\"info\"", tString)})
}

func TestHighlightMarkdownUnclosedFence(t *testing.T) {
	h := highlight(t, "a.md", "text\n```go\nfunc main() {}\n")
	if h.state&mdFenceBit == 0 {
		t.Fatalf("state %x does not keep the fence open", h.state)
	}
	if got := h.probe(t); got != tVariable {
		t.Fatalf("a line inside the open fence is kind %d, want %d", got, tVariable)
	}
}

func TestHighlightJSON(t *testing.T) {
	src := "{\n  // c\n  \"name\": \"x\\n\",\n  \"n\": [1, -2.5e3, true, null],\n  /* b */ \"o\": {}\n}\n"
	checkWants(t, "tsconfig.json", src, []want{w("// c", tComment), w("\"name\"", tProperty), wn("\"x", 0, tString), w("\\n", tEscape), w("1", tNumber), w("-2.5e3", tNumber),
		w("true", tConstant), w("null", tConstant), w("/* b */", tComment), w("\"o\"", tProperty)})
	for _, file := range []string{".prettierrc", ".vscode/settings.json", "x.code-workspace", "c.jsonc", "m.webmanifest"} {
		if got := nameOf(For(file, "")); got != "JSON" {
			t.Errorf("For(%q) = %q, want JSON", file, got)
		}
	}
}

func TestHighlightSmallFormats(t *testing.T) {
	checkWants(t, "go.mod", "module example.com/m\n\ngo 1.22\n\nrequire (\n\tgithub.com/a/b v1.2.3 // indirect\n)\nreplace x => ../x\n",
		[]want{w("module", tKeyword), w("example.com/m", tVariable), w("go", tKeyword), w("1.22", tNumber), w("require", tKeyword), w("v1.2.3", tNumber), w("// indirect", tComment), w("=>", tOperator)})
	checkWants(t, "go.sum", "github.com/a/b v1.2.3 h1:abc=\ngithub.com/a/b v1.2.3/go.mod h1:def=\n", []want{w("v1.2.3", tNumber), w("h1:abc=", tString)})
	checkWants(t, "de.po", "# translator\n#: src/a.ts:3\n#, fuzzy\nmsgid \"Hello %s\"\nmsgstr \"\"\n\"Hallo %s\\n\"\n",
		[]want{w("# translator", tComment), w("#: src/a.ts:3", tMeta), w("#, fuzzy", tKeyword), w("msgid", tKeyword), wn("%s", 0, tEscape), w("msgstr", tKeyword), w("\\n", tEscape)})
	checkWants(t, "Dockerfile.dev", "FROM golang:1.22 AS build\nENV KEY=1\nRUN apt-get update && \\\n    make \"$A\"\n# c\nEXPOSE 80\n",
		[]want{w("FROM", tKeyword), w("golang:1.22", tType), wn("AS", 0, tKeyword), w("build", tType), w("ENV", tKeyword), w("KEY", tProperty), w("RUN", tKeyword), w("apt-get", tFunction),
			w("&&", tOperator), w("make", tFunction), w("$A", tConstant), w("# c", tComment), w("80", tNumber)})
}

func TestStateConvergesAfterEdit(t *testing.T) {
	lines := []string{"const a = 1;", "const b = `x", "y${z}", "w`;", "function f() {", "  return 1;", "}", "const c = 2;"}
	states := func(src []string) []uint64 {
		out := make([]uint64, len(src)+1)
		for i, line := range src {
			_, out[i+1] = tokenize(t, typescript, line, out[i])
		}
		return out
	}
	before := states(lines)
	inside := append([]string{}, lines...)
	inside[2] = "other ${text}"
	for i, s := range states(inside) {
		if s != before[i] {
			t.Fatalf("an edit inside the template moved the state after line %d: %x vs %x", i, s, before[i])
		}
	}
	unquoted := append([]string{}, lines...)
	unquoted[1], unquoted[3] = "const b = 'x';", "w;"
	after := states(unquoted)
	if after[2] == before[2] {
		t.Fatalf("closing the template early left the state inside it")
	}
	if after[len(lines)] != before[len(lines)] {
		t.Fatalf("final states differ: %x vs %x", after[len(lines)], before[len(lines)])
	}
}
