package syntax

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

const benchLines = 5000

type benchFixture struct {
	name, file, builtin, src string
}

var benchFixtures = []benchFixture{
	{"typescript", "a.ts", "TypeScript", "import type { Foo } from \"./foo\";\nexport interface Props<T extends object = {}> extends Base {\n  name: string;\n  items?: Array<Map<string, number>>;\n}\n" +
		"export class Store<T> extends Base implements Api {\n  private count: number = 0;\n  async load(id: string): Promise<Foo | undefined> {\n" +
		"    const url = `/api/${id}/${list.map((x) => x.id).join(\",\")}`;\n    const ok = /ab+c/gi.test(url) ? new Map<string, number>() : null;\n    return this.client.get<Foo>(url);\n  }\n}\n"},
	{"svelte", "a.svelte", "Svelte", "<script lang=\"ts\">\n  let { a, b = 1 }: { a: string; b?: number } = $props();\n  let n = $state(0);\n</script>\n" +
		"<Foo {a} on:click={() => n++} bind:value class:on={n > 1} />\n{#if n > 1 && a}\n  <p class=\"x {a} y\">{n} {@html a}</p>\n{:else}\n  text\n{/if}\n<style>\n  .a { color: red; margin: 0 4px; }\n</style>\n"},
	{"go", "a.go", "Go", "package main\n\ntype Server struct {\n\tName string `json:\"name\"`\n\titems map[string][]Entry\n}\n\n" +
		"func (s *Server) Handle(w http.ResponseWriter, r *http.Request) (int, error) {\n\titems := []Entry{{Name: \"a\"}}\n\tfmt.Printf(\"%s: %d\\n\", len(items), 1)\n\tif v, ok := x.(*Claims); ok && v != nil {\n\t\treturn 0, nil\n\t}\n\treturn 1, nil\n}\n"},
	{"sql", "a.sql", "SQL", "-- users\nCREATE TABLE IF NOT EXISTS public.users (\n  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),\n  data jsonb DEFAULT '{}'::jsonb,\n  at timestamptz NOT NULL DEFAULT now()\n);\n" +
		"SELECT u.id, count(*) FROM users u WHERE u.id = $1 AND k = 'a' LIMIT 10;\n"},
	{"markdown", "a.md", "Markdown", "# Title\n\nSome *text* and `code`.\n\n```ts\nconst x: number = 1; // c\n```\n\n- item\n"},
	{"json", "a.json", "JSON", "{\n  \"name\": \"x\",\n  \"list\": [1, 2.5, true, null],\n  \"nested\": { \"k\": \"v\" }\n}\n"},
	{"astro", "a.astro", "", "<Layout title={title} class:list={[\"a\"]}>\n  <h1 class=\"t\">{title}</h1>\n  {posts.map((p) => <li>{p.title}</li>)}\n</Layout>\n"},
}

func benchSource(unit string) []string {
	lines := strings.Split(strings.TrimSuffix(unit, "\n"), "\n")
	out := make([]string, 0, benchLines)
	for len(out) < benchLines {
		out = append(out, lines...)
	}
	return out[:benchLines]
}

func BenchmarkTokenizeLine(b *testing.B) {
	for _, fx := range benchFixtures {
		impls := []struct {
			name string
			tk   editor.Tokenizer
		}{{"syntax", For(fx.file, "").Tokenizer}}
		if fx.builtin != "" {
			impls = append([]struct {
				name string
				tk   editor.Tokenizer
			}{{"mygo", editor.LanguageNamed(fx.builtin).Tokenizer}}, impls...)
		}
		lines := benchSource(fx.src)
		for _, impl := range impls {
			b.Run(fx.name+"/"+impl.name, func(b *testing.B) {
				var toks []editor.Token
				var state uint64
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					state = 0
					for _, line := range lines {
						toks, state = impl.tk.Tokenize(line, state, toks[:0])
					}
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(lines)), "ns/line")
			})
		}
	}
}
