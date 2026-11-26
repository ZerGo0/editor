package everywhere

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/files"
	"github.com/ZerGo0/editor/internal/search/symbols"
)

func newEnv(t *testing.T, names ...string) *Env {
	t.Helper()
	root := t.TempDir()
	for _, n := range names {
		p := filepath.Join(root, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx := &files.Index{Root: root}
	if err := idx.Refresh(context.Background(), func(string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	return &Env{Root: root, Files: idx}
}

func TestFilesRankOpenAndRecentFirst(t *testing.T) {
	env := newEnv(t, "src/main.go", "src/main_test.go", "docs/main.md")
	env.Recent = func() []string { return []string{filepath.Join(env.Root, "docs", "main.md")} }
	s := New(env).Session()
	s.MarkOpen([]string{filepath.Join(env.Root, "src", "main_test.go")})
	rs, total := s.Files("main", 10)
	if total != 3 || len(rs) != 3 {
		t.Fatalf("results %+v total %d", rs, total)
	}
	if rs[0].Label != "main_test.go" || !rs[0].Open {
		t.Fatalf("open file not first: %+v", rs[0])
	}
	if rs[1].Label != "main.md" || !rs[1].Recent {
		t.Fatalf("recent file not second: %+v", rs[1])
	}
	if rs[2].Recent || rs[2].Open {
		t.Fatalf("plain file flagged: %+v", rs[2])
	}
}

func TestFilesPositionsSplitNameAndFolder(t *testing.T) {
	env := newEnv(t, "internal/deep/hello.go")
	rs, _ := New(env).Session().Files("deephello", 5)
	if len(rs) != 1 {
		t.Fatalf("results %+v", rs)
	}
	if rs[0].Label != "hello.go" || rs[0].Detail != "internal/deep" || len(rs[0].Positions) == 0 || len(rs[0].DetailPositions) == 0 {
		t.Fatalf("result %+v", rs[0])
	}
}

func TestFilesEmptyQueryListsRecent(t *testing.T) {
	env := newEnv(t, "a.go", "b.go")
	env.Recent = func() []string { return []string{filepath.Join(env.Root, "b.go"), filepath.Join(env.Root, "a.go")} }
	rs, total := New(env).Session().Files("", 1)
	if len(rs) != 1 || rs[0].Label != "b.go" || total != 1 {
		t.Fatalf("results %+v total %d", rs, total)
	}
}

func TestActionsFilterAndKeys(t *testing.T) {
	env := &Env{Actions: func() []palette.Item {
		return []palette.Item{{ID: "save", Label: "Save File", Keys: "⌘S"}, {ID: "close", Label: "Close Tab"}}
	}}
	s := New(env).Session()
	rs, total := s.Actions("sav", 10)
	if total != 1 || rs[0].ActionID != "save" || rs[0].Keys != "⌘S" || rs[0].Kind != KindAction {
		t.Fatalf("results %+v", rs)
	}
	if all, _ := s.Actions("", 10); len(all) != 2 || all[0].ActionID != "save" {
		t.Fatalf("empty query should keep the order: %+v", all)
	}
	if rs, _ := New(&Env{}).Session().Actions("x", 5); rs != nil {
		t.Fatalf("no actions source: %+v", rs)
	}
}

func TestSymbolsRankedAndTyped(t *testing.T) {
	idx := symbols.NewIndex()
	idx.Update("/r/a.go", "Go", "package a\n\nfunc Hello() {}\n\ntype HelloWorld struct{}\n")
	s := New(&Env{Root: "/r", Symbols: idx}).Session()
	rs, total := s.Symbols(context.Background(), "hello", 10)
	if total != 2 || len(rs) != 2 {
		t.Fatalf("results %+v", rs)
	}
	if rs[0].Label != "Hello" || rs[0].Symbol != symbols.Function || rs[0].Detail != "a.go" || !rs[0].HasPos || rs[0].Line != 2 {
		t.Fatalf("first %+v", rs[0])
	}
	if rs[1].Symbol != symbols.Struct {
		t.Fatalf("second %+v", rs[1])
	}
	if got, _ := s.Symbols(context.Background(), "", 10); got != nil {
		t.Fatalf("empty query: %+v", got)
	}
}

func TestFilterRefs(t *testing.T) {
	refs := []Result{{Kind: KindGitRef, Label: "main"}, {Kind: KindGitRef, Label: "feature/login"}, {Kind: KindGitRef, Label: "v1.0", Ref: GitRef{Name: "v1.0", Tag: true}}}
	got := filterRefs("log", refs)
	if len(got) != 1 || got[0].Label != "feature/login" || len(got[0].Positions) == 0 {
		t.Fatalf("got %+v", got)
	}
	if all := filterRefs("", refs); len(all) != 3 {
		t.Fatalf("empty query: %+v", all)
	}
}
