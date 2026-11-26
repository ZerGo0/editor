package git

import (
	"context"
	"os"
	"strings"
	"testing"
)

func rebaseRepo(t *testing.T) (*Repo, string, []string) {
	t.Helper()
	repo := newRepo(t)
	base := commitFile(t, repo, "base.txt", "base\n", "base")
	var ids []string
	for _, name := range []string{"one", "two", "three", "four"} {
		ids = append(ids, commitFile(t, repo, name+".txt", name+"\n", "add "+name))
	}
	return repo, base, ids
}

func subjects(t *testing.T, repo *Repo, rng string) string {
	t.Helper()
	return strings.ReplaceAll(g(t, repo, "log", "--reverse", "--format=%s", rng), "\n", "|")
}

func TestRebaseTodo(t *testing.T) {
	repo, base, ids := rebaseRepo(t)
	items, err := repo.RebaseTodo(context.Background(), base)
	if err != nil || len(items) != 4 {
		t.Fatalf("todo: %+v %v", items, err)
	}
	for i, it := range items {
		if it.Action != ActionPick || it.OID != ids[i] {
			t.Fatalf("item %d: %+v", i, it)
		}
	}
	if items[0].Subject != "add one" {
		t.Fatalf("subject: %q", items[0].Subject)
	}
	if _, err := repo.RebaseTodo(context.Background(), "-x"); err == nil {
		t.Fatal("option-like ref must be rejected")
	}
}

func TestRebaseInteractiveDropRewordSquash(t *testing.T) {
	repo, base, ids := rebaseRepo(t)
	ctx := context.Background()
	items := []TodoItem{
		{Action: ActionReword, OID: ids[0], Subject: "add one"},
		{Action: ActionDrop, OID: ids[1], Subject: "add two"},
		{Action: ActionPick, OID: ids[2], Subject: "add three"},
		{Action: ActionSquash, OID: ids[3], Subject: "add four"},
	}
	messages := map[string]string{
		ids[0]: "renamed one\n\nwith a body\n",
		ids[3]: "three and four together\n",
	}
	res, err := repo.RebaseInteractive(ctx, base, items, messages)
	if err != nil || res.Stopped {
		t.Fatalf("rebase: %+v %v", res, err)
	}
	if got := subjects(t, repo, base+"..HEAD"); got != "renamed one|three and four together" {
		t.Fatalf("subjects: %q", got)
	}
	if body := g(t, repo, "log", "--format=%b", base+"..HEAD"); !strings.Contains(body, "with a body") {
		t.Fatalf("body: %q", body)
	}
	if exists(repo, "two.txt") || !exists(repo, "three.txt") || !exists(repo, "four.txt") {
		t.Fatal("wrong tree after rebase")
	}
	if st, _ := repo.State(ctx); st.Kind != StateNone {
		t.Fatalf("state: %+v", st)
	}
	if _, err := os.Stat(repo.gitPath(rebaseHelperDir)); !os.IsNotExist(err) {
		t.Fatal("helper files must be removed")
	}
}

func TestRebaseInteractiveFixupAndDefaultMessages(t *testing.T) {
	repo, base, ids := rebaseRepo(t)
	items := []TodoItem{
		{Action: ActionPick, OID: ids[0]},
		{Action: ActionFixup, OID: ids[1]},
		{Action: ActionReword, OID: ids[2]},
		{Action: ActionSquash, OID: ids[3]},
	}
	res, err := repo.RebaseInteractive(context.Background(), base, items, nil)
	if err != nil || res.Stopped {
		t.Fatalf("rebase: %+v %v", res, err)
	}
	if got := subjects(t, repo, base+"..HEAD"); got != "add one|add three" {
		t.Fatalf("subjects: %q", got)
	}
	if body := g(t, repo, "log", "-1", "--format=%B"); !strings.Contains(body, "add four") {
		t.Fatalf("squash message keeps both subjects: %q", body)
	}
}

func TestRebaseInteractiveEditStop(t *testing.T) {
	repo, base, ids := rebaseRepo(t)
	ctx := context.Background()
	items := []TodoItem{
		{Action: ActionPick, OID: ids[0]},
		{Action: ActionEdit, OID: ids[1]},
		{Action: ActionReword, OID: ids[2]},
		{Action: ActionPick, OID: ids[3]},
	}
	res, err := repo.RebaseInteractive(ctx, base, items, map[string]string{ids[2]: "third renamed"})
	if err != nil || !res.Stopped {
		t.Fatalf("expected stop at edit: %+v %v", res, err)
	}
	st, _ := repo.State(ctx)
	if st.Kind != StateRebaseInteractive || st.Step != 2 || st.Total != 5 {
		t.Fatalf("state: %+v", st)
	}
	if _, err := os.Stat(repo.gitPath(rebaseHelperDir)); err != nil {
		t.Fatal("helper files must stay while the rebase is open")
	}
	put(t, repo, "edited.txt", "e\n")
	g(t, repo, "add", "edited.txt")
	g(t, repo, "commit", "--quiet", "--amend", "--no-edit")
	res, err = repo.Continue(ctx)
	if err != nil || res.Stopped {
		t.Fatalf("continue: %+v %v", res, err)
	}
	if got := subjects(t, repo, base+"..HEAD"); got != "add one|add two|third renamed|add four" {
		t.Fatalf("subjects: %q", got)
	}
	if !exists(repo, "edited.txt") {
		t.Fatal("amended file missing")
	}
	if _, err := os.Stat(repo.gitPath(rebaseHelperDir)); !os.IsNotExist(err) {
		t.Fatal("helper files must be removed after continue")
	}
}

func TestRebaseInteractiveConflictAbort(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	base := commitFile(t, repo, "f.txt", "1\n", "base")
	a := commitFile(t, repo, "f.txt", "1\n2\n", "add 2")
	b := commitFile(t, repo, "f.txt", "1\n2\n3\n", "add 3")
	original := g(t, repo, "rev-parse", "HEAD")
	items := []TodoItem{{Action: ActionPick, OID: b}, {Action: ActionPick, OID: a}}
	res, err := repo.RebaseInteractive(ctx, base, items, nil)
	if err != nil || !res.Stopped {
		t.Fatalf("expected conflict stop: %+v %v", res, err)
	}
	conflicts, _ := repo.Conflicts(ctx)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts: %+v", conflicts)
	}
	if err := repo.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	if g(t, repo, "rev-parse", "HEAD") != original {
		t.Fatal("abort must restore the original head")
	}
	if _, err := os.Stat(repo.gitPath(rebaseHelperDir)); !os.IsNotExist(err) {
		t.Fatal("helper files must be removed after abort")
	}
}

func TestRebaseInteractiveValidation(t *testing.T) {
	repo, base, ids := rebaseRepo(t)
	ctx := context.Background()
	bad := [][]TodoItem{
		nil,
		{{Action: "bogus", OID: ids[0]}},
		{{Action: ActionSquash, OID: ids[0]}},
		{{Action: ActionDrop, OID: ids[0]}, {Action: ActionFixup, OID: ids[1]}},
		{{Action: ActionPick, OID: "--exec=x"}},
	}
	for i, items := range bad {
		if _, err := repo.RebaseInteractive(ctx, base, items, nil); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
	if st, _ := repo.State(ctx); st.Kind != StateNone {
		t.Fatalf("state: %+v", st)
	}
}

func TestShellQuoteAndTodoText(t *testing.T) {
	if got := shellQuote("it's here"); got != "'it'\\''s here'" {
		t.Fatalf("got %s", got)
	}
	dir := t.TempDir()
	todo, err := buildTodo(dir, []TodoItem{
		{Action: ActionReword, OID: "a1", Subject: "multi\nline"},
		{Action: ActionDrop, OID: "b2", Subject: "x"},
		{Action: ActionFixup, OID: "c3", Subject: "y"},
	}, map[string]string{"c3": "final"})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(todo), "\n")
	if len(lines) != 4 || lines[0] != "reword a1 multi line" || lines[1] != "drop b2 x" || lines[2] != "fixup c3 y" || !strings.HasPrefix(lines[3], "exec git commit --amend") {
		t.Fatalf("todo:\n%s", todo)
	}
}
