package git

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseCommitFields(t *testing.T) {
	t.Parallel()
	fields := []string{
		"aaaa", "bbbb cccc", "Ada", "ada@example.com", "1700000000", "Bob", "bob@example.com", "1700000100",
		"HEAD -> main, tag: v1, origin/main", "Subject line", "Body line 1\nBody line 2\n",
	}
	c, err := parseCommitFields(fields)
	if err != nil {
		t.Fatal(err)
	}
	want := Commit{
		OID: "aaaa", Parents: []string{"bbbb", "cccc"}, Author: "Ada", AuthorEmail: "ada@example.com",
		AuthorTime: time.Unix(1700000000, 0), Committer: "Bob", CommitTime: time.Unix(1700000100, 0),
		Subject: "Subject line", Body: "Body line 1\nBody line 2", Refs: []string{"HEAD -> main", "tag: v1", "origin/main"},
	}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("got %+v\nwant %+v", c, want)
	}
	bad := [][]string{fields[:5], append(append([]string{}, fields[:4]...), append([]string{"not-a-number"}, fields[5:]...)...)}
	for _, b := range bad {
		if _, err := parseCommitFields(b); err == nil {
			t.Errorf("expected error for %q", b)
		}
	}
}

func TestParseNameStatus(t *testing.T) {
	t.Parallel()
	files, err := parseNameStatus([]byte("M\x00a.txt\x00R087\x00old.txt\x00new.txt\x00A\x00sp ace.txt\x00"))
	if err != nil {
		t.Fatal(err)
	}
	want := []ChangedFile{{Path: "a.txt", Status: 'M'}, {Path: "new.txt", OrigPath: "old.txt", Status: 'R'}, {Path: "sp ace.txt", Status: 'A'}}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("got %+v", files)
	}
	for _, bad := range []string{"R100\x00only\x00", "M\x00", "\x00x\x00"} {
		if _, err := parseNameStatus([]byte(bad)); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func historyRepo(t *testing.T) (*Repo, map[string]string) {
	t.Helper()
	repo := newRepo(t)
	ids := map[string]string{}
	ids["root"] = commitFile(t, repo, "a.txt", numbered(10), "root commit")
	ids["second"] = commitFile(t, repo, "b.txt", "b\n", "add b\n\nmulti line body")
	g(t, repo, "switch", "--quiet", "--create", "feature")
	ids["feature"] = commitFile(t, repo, "f.txt", "f\n", "feature work")
	g(t, repo, "switch", "--quiet", "main")
	ids["main2"] = commitFile(t, repo, "m.txt", "m\n", "main work")
	g(t, repo, "merge", "--quiet", "--no-ff", "-m", "merge feature", "feature")
	ids["merge"] = g(t, repo, "rev-parse", "HEAD")
	g(t, repo, "tag", "v1")
	return repo, ids
}

func TestLog(t *testing.T) {
	t.Parallel()
	repo, ids := historyRepo(t)
	ctx := context.Background()
	all, err := repo.Log(ctx, LogOptions{})
	if err != nil || len(all) != 5 {
		t.Fatalf("log: %d commits, %v", len(all), err)
	}
	head := all[0]
	if head.OID != ids["merge"] || head.Subject != "merge feature" || len(head.Parents) != 2 || head.Author != "Ada Lovelace" || head.AuthorEmail != "ada@example.com" {
		t.Fatalf("head: %+v", head)
	}
	joined := strings.Join(head.Refs, "|")
	if !strings.Contains(joined, "HEAD -> main") || !strings.Contains(joined, "tag: v1") {
		t.Fatalf("refs: %v", head.Refs)
	}
	var second Commit
	for _, c := range all {
		if c.OID == ids["second"] {
			second = c
		}
	}
	if second.Subject != "add b" || second.Body != "multi line body" || len(second.Parents) != 1 {
		t.Fatalf("second: %+v", second)
	}
	if time.Since(second.AuthorTime) > time.Hour {
		t.Fatalf("author time: %v", second.AuthorTime)
	}
	limited, err := repo.Log(ctx, LogOptions{Max: 2, Skip: 1})
	if err != nil || len(limited) != 2 || limited[0].OID != all[1].OID {
		t.Fatalf("max/skip: %+v %v", limited, err)
	}
	byGrep, err := repo.Log(ctx, LogOptions{Grep: "feature"})
	if err != nil || len(byGrep) != 2 {
		t.Fatalf("grep: %+v %v", byGrep, err)
	}
	byAuthor, err := repo.Log(ctx, LogOptions{Author: "nobody"})
	if err != nil || len(byAuthor) != 0 {
		t.Fatalf("author: %+v %v", byAuthor, err)
	}
	byPath, err := repo.Log(ctx, LogOptions{Paths: []string{"f.txt"}})
	if err != nil || len(byPath) != 1 || byPath[0].OID != ids["feature"] {
		t.Fatalf("path: %+v %v", byPath, err)
	}
	g(t, repo, "branch", "side", ids["root"])
	rangeLog, err := repo.Log(ctx, LogOptions{Revs: []string{"side..main"}})
	if err != nil || len(rangeLog) != 4 {
		t.Fatalf("range: %d %v", len(rangeLog), err)
	}
	g(t, repo, "switch", "--quiet", "side")
	g(t, repo, "branch", "-D", "feature")
	allRefs, err := repo.Log(ctx, LogOptions{All: true})
	if err != nil || len(allRefs) != 5 {
		t.Fatalf("all: %d %v", len(allRefs), err)
	}
	if _, err := repo.Log(ctx, LogOptions{Revs: []string{"--all"}}); err == nil {
		t.Error("option-like rev must be rejected")
	}
	if _, err := repo.Log(ctx, LogOptions{Follow: true}); err == nil {
		t.Error("follow without a path must fail")
	}
}

func TestLogUnbornAndFollow(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ctx := context.Background()
	commits, err := repo.Log(ctx, LogOptions{})
	if err != nil || len(commits) != 0 {
		t.Fatalf("unborn: %+v %v", commits, err)
	}
	commitFile(t, repo, "old.txt", numbered(30), "create")
	commitFile(t, repo, "other.txt", "x\n", "other")
	g(t, repo, "mv", "old.txt", "new.txt")
	renamed := commitAll(t, repo, "rename")
	commitFile(t, repo, "new.txt", numbered(30)+"extra\n", "edit")
	plain, err := repo.Log(ctx, LogOptions{Paths: []string{"new.txt"}})
	if err != nil || len(plain) != 2 {
		t.Fatalf("without follow: %d %v", len(plain), err)
	}
	followed, err := repo.Log(ctx, LogOptions{Paths: []string{"new.txt"}, Follow: true})
	if err != nil || len(followed) != 3 || followed[1].OID != renamed {
		t.Fatalf("follow: %+v %v", followed, err)
	}
	files, err := repo.CommitFiles(ctx, renamed)
	if err != nil || len(files) != 1 || files[0].Status != 'R' || files[0].OrigPath != "old.txt" || files[0].Path != "new.txt" {
		t.Fatalf("rename files: %+v %v", files, err)
	}
	rootFiles, err := repo.CommitFiles(ctx, followed[2].OID)
	if err != nil || len(rootFiles) != 1 || rootFiles[0].Status != 'A' {
		t.Fatalf("root files: %+v %v", rootFiles, err)
	}
}

func TestCommitFilesMerge(t *testing.T) {
	t.Parallel()
	repo, ids := historyRepo(t)
	files, err := repo.CommitFiles(context.Background(), ids["merge"])
	if err != nil || len(files) != 1 || files[0].Path != "f.txt" || files[0].Status != 'A' {
		t.Fatalf("merge files against first parent: %+v %v", files, err)
	}
}

func commit(oid string, parents ...string) Commit { return Commit{OID: oid, Parents: parents} }

func TestLanes(t *testing.T) {
	t.Parallel()
	t.Run("linear", func(t *testing.T) {
		rows := Lanes([]Commit{commit("c", "b"), commit("b", "a"), commit("a")})
		for i, row := range rows {
			if row.Column != 0 || row.Width != 1 {
				t.Errorf("row %d: %+v", i, row)
			}
		}
		if !reflect.DeepEqual(rows[0].Edges, []GraphEdge{{Parent: "b", Column: 0}}) || len(rows[2].Edges) != 0 {
			t.Errorf("edges: %+v", rows)
		}
		if !reflect.DeepEqual(rows[1].Incoming, []int{0}) || len(rows[0].Incoming) != 0 {
			t.Errorf("incoming: %+v", rows)
		}
	})
	t.Run("merge and fork", func(t *testing.T) {
		rows := Lanes([]Commit{
			commit("m", "a", "b"),
			commit("a", "r"),
			commit("b", "r"),
			commit("r"),
		})
		if got := rows[0].Edges; !reflect.DeepEqual(got, []GraphEdge{{"a", 0}, {"b", 1}}) {
			t.Fatalf("merge edges: %+v", got)
		}
		if rows[1].Column != 0 || !reflect.DeepEqual(rows[1].Through, []int{1}) {
			t.Fatalf("row a: %+v", rows[1])
		}
		if rows[2].Column != 1 || rows[2].Edges[0].Column != 0 {
			t.Fatalf("row b: %+v", rows[2])
		}
		if rows[3].Column != 0 || rows[3].Width != 1 {
			t.Fatalf("row r: %+v", rows[3])
		}
		if !reflect.DeepEqual(rows[3].Incoming, []int{0}) {
			t.Fatalf("row r incoming: %+v", rows[3])
		}
	})
	t.Run("two heads converge", func(t *testing.T) {
		rows := Lanes([]Commit{commit("x", "p"), commit("y", "p"), commit("p")})
		if rows[1].Column != 1 || rows[1].Edges[0].Column != 0 {
			t.Fatalf("y: %+v", rows[1])
		}
		if !reflect.DeepEqual(rows[2].Incoming, []int{0}) {
			t.Fatalf("p incoming: %+v", rows[2])
		}
	})
	t.Run("octopus and roots", func(t *testing.T) {
		rows := Lanes([]Commit{commit("o", "a", "b", "c"), commit("a"), commit("b"), commit("c")})
		if len(rows[0].Edges) != 3 || rows[0].Edges[2].Column != 2 || rows[0].Width != 3 {
			t.Fatalf("octopus: %+v", rows[0])
		}
		if rows[2].Column != 1 || rows[3].Column != 2 {
			t.Fatalf("columns: %+v", rows)
		}
	})
	t.Run("empty and truncated", func(t *testing.T) {
		if len(Lanes(nil)) != 0 {
			t.Fatal("expected no rows")
		}
		rows := Lanes([]Commit{commit("a", "missing")})
		if rows[0].Edges[0].Column != 0 {
			t.Fatalf("%+v", rows)
		}
	})
	t.Run("real history", func(t *testing.T) {
		repo, _ := historyRepo(t)
		commits, err := repo.Log(context.Background(), LogOptions{})
		if err != nil {
			t.Fatal(err)
		}
		rows := Lanes(commits)
		if len(rows) != len(commits) || rows[0].Column != 0 || len(rows[0].Edges) != 2 {
			t.Fatalf("rows: %+v", rows)
		}
		last := rows[len(rows)-1]
		if len(last.Edges) != 0 {
			t.Fatalf("root commit must have no edges: %+v", last)
		}
	})
}

func TestLanesNeverReusesLiveColumn(t *testing.T) {
	t.Parallel()
	commits := []Commit{
		commit("h1", "p1"), commit("h2", "p2"), commit("h3", "p3"),
		commit("p1"), commit("p2", "p3"), commit("p3"),
	}
	rows := Lanes(commits)
	live := map[string]int{}
	for i, c := range commits {
		row := rows[i]
		for _, e := range row.Edges {
			if col, ok := live[e.Parent]; ok && col != e.Column {
				t.Errorf("row %d: parent %s moved from lane %d to %d", i, e.Parent, col, e.Column)
			}
			live[e.Parent] = e.Column
		}
		if col, ok := live[c.OID]; ok && col != row.Column && !contains(row.Incoming, col) {
			t.Errorf("row %d: %s expected in lane %d got %d", i, c.OID, col, row.Column)
		}
	}
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
