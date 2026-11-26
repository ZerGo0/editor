package gitui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

func TestMergeToolApply(t *testing.T) {
	r := &fakeRepo{
		status:   &git.Status{Branch: "main", OID: "a", Files: []git.FileStatus{{Path: "m.txt", Index: 'U', WorkTree: 'U', Conflict: true}}},
		versions: [3]string{"a\nb\nc\n", "a\nours\nc\n", "a\ntheirs\nc\nd\n"},
	}
	g, h := newFakeGit(t, r)
	g.openMergeTool("m.txt")
	tool := g.mergeTools["git-merge:m.txt"]
	h.drain(t, func() bool { return tool.model != nil })
	tt := newTester(g, tool.view)
	if !tt.HasText("1 conflicts left") {
		t.Fatalf("texts = %v", tt.Texts())
	}
	if err := tt.Click("Accept Left"); err != nil {
		t.Fatal(err)
	}
	if got := tool.result.Buffer.String(); got != "a\nours\nc\n" {
		t.Fatalf("result = %q", got)
	}
	tt.Frame()
	if err := tt.Click("Apply"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { _, open := g.mergeTools["git-merge:m.txt"]; return !open })
	if !r.called("resolved m.txt") {
		t.Fatal("file not marked as resolved")
	}
	data, err := os.ReadFile(filepath.Join(r.dir, "m.txt"))
	if err != nil || string(data) != "a\nours\nc\n" {
		t.Fatalf("file = %q, %v", data, err)
	}
}

func TestAnnotateToggle(t *testing.T) {
	r := &fakeRepo{
		status: &git.Status{Branch: "main", OID: "a"},
		blame:  []git.BlameLine{{Line: 1, OID: "0123456789", Author: "Ada", Time: time.Now(), Summary: "Init"}},
	}
	g, h := newFakeGit(t, r)
	abs := g.paths.abs("a.go")
	st := &editor.State{Buffer: editor.NewBuffer("package a\n")}
	g.Annotate(abs, st)
	if !g.Annotated(abs) || st.ContextMenu == nil || st.OnGutterClick == nil {
		t.Fatal("annotations not installed")
	}
	h.drain(t, func() bool { return len(g.blames[abs].lines) == 1 })
	st.OnGutterClick(0)
	if !g.WantsPanel() {
		t.Fatal("gutter click did not show the commit")
	}
	g.Annotate(abs, st)
	if g.Annotated(abs) || st.ContextMenu != nil || st.OnGutterClick != nil {
		t.Fatal("annotations not removed")
	}
}

func TestPushDialog(t *testing.T) {
	r := &fakeRepo{
		status:  &git.Status{Branch: "main", OID: "a", Upstream: "origin/main", Ahead: 1},
		remotes: []git.Remote{{Name: "origin"}},
		commits: []git.Commit{{OID: "c1", Subject: "Outgoing work"}},
		files:   []git.ChangedFile{{Path: "x.go", Status: 'M'}},
	}
	g, h := newFakeGit(t, r)
	g.openPush()
	tt := newTester(g, g.SideView)
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("Outgoing work") && tt.HasText("x.go") })
	if err := tt.Click("Push"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return r.called("push") })
	if r.pushed.Remote != "origin" || r.pushed.Branch != "main" || r.pushed.Force {
		t.Fatalf("push options = %+v", r.pushed)
	}
}
