package references

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const (
	goDecl = "package a\n\n// Foo is documented here.\nfunc Foo() {}\n\nfunc use() {\n\tFoo()\n\tprintln(\"Foo\")\n\tFooBar()\n\txFoo()\n}\n"
	goUse  = "package b\n\nfunc other() { Foo() }\n"
	tsUse  = "export const x = Foo();\n"
	tsOnly = "const onlyHere = 1;\n"
)

func fixture(t *testing.T, files map[string]string) (string, []string) {
	t.Helper()
	root := t.TempDir()
	var paths []string
	for name, data := range files {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return root, paths
}

func lineCols(res Result, path string) [][2]int {
	var out [][2]int
	for _, f := range res.Files {
		if f.Path != path {
			continue
		}
		for _, m := range f.Matches {
			out = append(out, [2]int{m.Start.Line, m.Start.Col})
		}
	}
	return out
}

func TestFindSkipsCommentsStringsAndPartialWords(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": goDecl, "b.go": goUse, "c.ts": tsUse})
	a, b := filepath.Join(root, "a.go"), filepath.Join(root, "b.go")
	q := Query{Word: "Foo", Root: root, Current: a, Files: paths, Language: "Go", Sites: []Site{{Path: a, Line: 3, Col: 5}}}
	res, err := Find(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	got := lineCols(res, a)
	want := [][2]int{{3, 5}, {6, 1}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("a.go matches %v, want %v", got, want)
	}
	if len(lineCols(res, filepath.Join(root, "c.ts"))) != 0 {
		t.Fatal("a TypeScript file was searched for a Go word")
	}
	if len(lineCols(res, b)) != 1 || res.Total != 3 {
		t.Fatalf("total %d, b.go %v", res.Total, lineCols(res, b))
	}
}

func TestFindPutsTheCurrentFileAndTheDefinitionFirst(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": goDecl, "b.go": goUse})
	a, b := filepath.Join(root, "a.go"), filepath.Join(root, "b.go")
	q := Query{Word: "Foo", Root: root, Current: b, Files: paths, Language: "Go", Sites: []Site{{Path: a, Line: 3, Col: 5}}}
	res, err := Find(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if res.Files[0].Path != b || res.Files[1].Path != a {
		t.Fatalf("order %s, %s", res.Files[0].Path, res.Files[1].Path)
	}
	first := res.Files[1].Matches[0]
	if !first.Definition || first.Start.Line != 3 {
		t.Fatalf("the first match of a.go is %+v, want the definition", first)
	}
	if res.Files[1].Matches[1].Definition {
		t.Fatal("a use is marked as a definition")
	}
}

func TestFindFallsBackToOtherLanguagesWhenTheFamilyHasNoMatch(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": "package a\n", "c.ts": tsUse, "d.ts": tsOnly})
	q := Query{Word: "Foo", Root: root, Current: filepath.Join(root, "a.go"), Files: paths, Language: "Go"}
	res, err := Find(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Files[0].Path != filepath.Join(root, "c.ts") {
		t.Fatalf("fallback result %+v", res)
	}
}

func TestFindSearchesTheScriptFamilyTogether(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.ts": "export function Foo() {}\n", "b.tsx": "const a = <Foo />;\n", "c.js": "Foo();\n", "d.go": goDecl})
	a := filepath.Join(root, "a.ts")
	q := Query{Word: "Foo", Root: root, Current: a, Files: paths, Language: "TypeScript", Sites: []Site{{Path: a, Line: 0, Col: 16}}}
	res, err := Find(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 3 {
		t.Fatalf("files %d, want the ts, tsx and js files", len(res.Files))
	}
}

func TestFindReadsOpenBuffersBeforeTheDisk(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": "package a\n"})
	a := paths[0]
	q := Query{Word: "Foo", Root: root, Current: a, Files: paths, Language: "Go", Overlay: func(p string) (string, bool) { return "package a\n\nvar _ = Foo\n", p == a }}
	res, err := Find(context.Background(), q)
	if err != nil || res.Total != 1 {
		t.Fatalf("result %+v, error %v", res, err)
	}
}

func TestFindStopsWhenCancelled(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": goDecl})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Find(ctx, Query{Word: "Foo", Root: root, Files: paths, Language: "Go"}); err == nil {
		t.Fatal("a cancelled search returned no error")
	}
}

func TestLocateKeepsOnlyTheSites(t *testing.T) {
	root, _ := fixture(t, map[string]string{"a.go": goDecl})
	a := filepath.Join(root, "a.go")
	res, err := Locate(context.Background(), Query{Word: "Foo", Root: root, Current: a, Sites: []Site{{Path: a, Line: 3, Col: 5}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || !res.Files[0].Matches[0].Definition {
		t.Fatalf("result %+v", res)
	}
}

func TestFindCapsTheMatches(t *testing.T) {
	var body string
	for range MaxMatches + 10 {
		body += "Foo()\n"
	}
	root, paths := fixture(t, map[string]string{"a.go": "package a\n\n" + body})
	res, err := Find(context.Background(), Query{Word: "Foo", Root: root, Files: paths, Language: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != MaxMatches || !res.Truncated {
		t.Fatalf("total %d, truncated %v", res.Total, res.Truncated)
	}
}

func TestWholeWordTreatsUnicodeLettersAsWordCharacters(t *testing.T) {
	line := "café x é"
	if wholeWord(line, len("caf"), len("café")) {
		t.Fatal("é inside café is a whole word")
	}
	if !wholeWord(line, len("café x "), len(line)) {
		t.Fatal("a lone é is not a whole word")
	}
}

const (
	memberSource = "package a\n\nfunc size() int { return 1 }\n\nfunc use(x T) {\n\t_ = x.size\n\t_ = size()\n\t_ = x.size()\n\t_ = p->size\n}\n"
	freeOnly     = "package a\n\nfunc use() { size() }\n"
	memberOnly   = "package a\n\nfunc use(x T) { x.size() }\n"
)

func TestFindPrefersMembersWhenTheNameFollowsAMemberOperator(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": memberSource})
	a := filepath.Join(root, "a.go")
	q := Query{Word: "size", Root: root, Current: a, Files: paths, Language: "Go", Member: true, Sites: []Site{{Path: a, Line: 2, Col: 5}}}
	res, err := Find(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	got := lineCols(res, a)
	want := [][2]int{{2, 5}, {5, 7}, {7, 7}, {8, 8}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("member matches %v, want the definition and %v", got, want)
	}
}

func TestFindPrefersFreeNamesWhenTheNameFollowsNoMemberOperator(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": memberSource})
	a := filepath.Join(root, "a.go")
	q := Query{Word: "size", Root: root, Current: a, Files: paths, Language: "Go", Sites: []Site{{Path: a, Line: 2, Col: 5}}}
	res, err := Find(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	got := lineCols(res, a)
	want := [][2]int{{2, 5}, {6, 5}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("free matches %v, want %v", got, want)
	}
}

func TestFindKeepsTheOtherGroupWhenThePreferredGroupIsEmpty(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": freeOnly, "b.go": memberOnly})
	a := filepath.Join(root, "a.go")
	member, err := Find(context.Background(), Query{Word: "size", Root: root, Current: a, Files: paths, Language: "Go", Member: true, Sites: []Site{{Path: a, Line: 0, Col: 0}}})
	if err != nil || member.Total != 1 || member.Files[0].Path != filepath.Join(root, "b.go") {
		t.Fatalf("member result %+v, error %v", member, err)
	}
	free, err := Find(context.Background(), Query{Word: "size", Root: root, Current: filepath.Join(root, "b.go"), Files: paths[:1], Language: "Go", Sites: []Site{{Path: a, Line: 0, Col: 0}}})
	if err != nil || free.Total != 1 {
		t.Fatalf("free result %+v, error %v", free, err)
	}
	only, err := Find(context.Background(), Query{Word: "size", Root: root, Current: filepath.Join(root, "b.go"), Files: paths[1:], Language: "Go", Member: false, Sites: []Site{{Path: a, Line: 0, Col: 0}}})
	if err != nil || only.Total != 1 {
		t.Fatalf("fallback result %+v, error %v", only, err)
	}
}

func TestMemberAccess(t *testing.T) {
	for _, c := range []struct {
		line   string
		start  int
		member bool
	}{
		{"a.size", 2, true}, {"a . size", 4, true}, {"a?.size", 3, true}, {"p->size", 3, true}, {"T::size", 3, true},
		{"size", 0, false}, {"x := size", 5, false}, {"f(...size)", 6, false}, {"::size", 2, false}, {"a..size", 3, false},
	} {
		if _, got := memberAccess(c.line, c.start); got != c.member {
			t.Errorf("memberAccess(%q, %d) = %v, want %v", c.line, c.start, got, c.member)
		}
	}
}

func TestPreferMemberSites(t *testing.T) {
	sites := []Site{{Path: "a", Member: true}, {Path: "b"}, {Path: "c", Member: true}}
	if got := PreferMember(sites, true); len(got) != 2 || got[0].Path != "a" || got[1].Path != "c" {
		t.Fatalf("member sites %v", got)
	}
	if got := PreferMember(sites, false); len(got) != 1 || got[0].Path != "b" {
		t.Fatalf("free sites %v", got)
	}
	if got := PreferMember(sites[:1], false); len(got) != 1 {
		t.Fatalf("fallback sites %v", got)
	}
}

func TestFindLimitsANameWithoutADefinitionToTheCurrentFile(t *testing.T) {
	root, paths := fixture(t, map[string]string{"a.go": "package a\n\nfunc f() {\n\tcount := 1\n\t_ = count\n}\n", "b.go": "package b\n\nvar _ = count\n"})
	a, b := filepath.Join(root, "a.go"), filepath.Join(root, "b.go")
	res, err := Find(context.Background(), Query{Word: "count", Root: root, Current: a, Files: paths, Language: "Go"})
	if err != nil || len(res.Files) != 1 || res.Files[0].Path != a || res.Total != 2 {
		t.Fatalf("local result %+v, error %v", res, err)
	}
	withDefinition, err := Find(context.Background(), Query{Word: "count", Root: root, Current: a, Files: paths, Language: "Go", Sites: []Site{{Path: b, Line: 2, Col: 4}}})
	if err != nil || len(withDefinition.Files) != 2 {
		t.Fatalf("project result %+v, error %v", withDefinition, err)
	}
	elsewhere, err := Find(context.Background(), Query{Word: "count", Root: root, Current: filepath.Join(root, "c.go"), Files: paths, Language: "Go"})
	if err != nil || len(elsewhere.Files) != 2 {
		t.Fatalf("a name that is not in the current file searched %+v, error %v", elsewhere, err)
	}
}
