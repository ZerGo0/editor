package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const zeroOID = "0000000000000000000000000000000000000000"

const sampleBlame = "1111111111111111111111111111111111111111 1 1 2\n" +
	"author Ada Lovelace\n" +
	"author-mail <ada@example.com>\n" +
	"author-time 1700000000\n" +
	"author-tz +0200\n" +
	"committer Ada Lovelace\n" +
	"summary first commit\n" +
	"filename dir/a b.txt\n" +
	"\tfirst line\n" +
	"1111111111111111111111111111111111111111 2 2\n" +
	"\t\n" +
	zeroOID + " 3 3 1\n" +
	"author Not Committed Yet\n" +
	"author-time 1700000500\n" +
	"author-tz -0130\n" +
	"summary Version of a.txt from a.txt\n" +
	"filename a.txt\n" +
	"\t\ttabbed\n"

func TestParseBlame(t *testing.T) {
	lines, err := parseBlame([]byte(sampleBlame))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines", len(lines))
	}
	if l := lines[0]; l.Line != 1 || l.OrigLine != 1 || l.Author != "Ada Lovelace" || l.Summary != "first commit" || l.Text != "first line" || l.Filename != "dir/a b.txt" || l.IsUncommitted() {
		t.Fatalf("line 1: %+v", l)
	}
	if _, off := lines[0].Time.Zone(); off != 2*3600 || lines[0].Time.Unix() != 1700000000 {
		t.Fatalf("time: %v", lines[0].Time)
	}
	if l := lines[1]; l.Line != 2 || l.Text != "" || l.Author != "Ada Lovelace" {
		t.Fatalf("line 2: %+v", l)
	}
	if l := lines[2]; !l.IsUncommitted() || l.Text != "\ttabbed" || l.Author != "Not Committed Yet" {
		t.Fatalf("line 3: %+v", l)
	}
	if _, off := lines[2].Time.Zone(); off != -(90 * 60) {
		t.Fatalf("negative zone: %v", lines[2].Time)
	}
	bad := []string{"garbage\n", "1111 1 1\n\tx\n", sampleBlame[:len(sampleBlame)-12], "1111111111111111111111111111111111111111 x 1\n"}
	for _, b := range bad {
		if _, err := parseBlame([]byte(b)); err == nil {
			t.Errorf("expected error for %q", b)
		}
	}
	for i := range sampleBlame {
		_, _ = parseBlame([]byte(sampleBlame[:i]))
	}
}

func TestBlame(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	first := commitFile(t, repo, "a.txt", "one\ntwo\nthree\n", "first")
	put(t, repo, "a.txt", "one\nTWO\nthree\n")
	second := commitAll(t, repo, "second change")
	lines, err := repo.Blame(ctx, "a.txt", "", nil)
	if err != nil || len(lines) != 3 {
		t.Fatalf("blame: %+v %v", lines, err)
	}
	if lines[0].OID != first || lines[1].OID != second || lines[2].OID != first {
		t.Fatalf("oids: %+v", lines)
	}
	if lines[1].Summary != "second change" || lines[1].Author != "Ada Lovelace" || lines[1].Text != "TWO" || lines[1].IsUncommitted() {
		t.Fatalf("line 2: %+v", lines[1])
	}
	atFirst, err := repo.Blame(ctx, "a.txt", first, nil)
	if err != nil || atFirst[1].OID != first || atFirst[1].Text != "two" {
		t.Fatalf("blame at rev: %+v %v", atFirst, err)
	}
	unsaved := []byte("one\nTWO\nunsaved edit\nthree\n")
	withContents, err := repo.Blame(ctx, "a.txt", "", unsaved)
	if err != nil || len(withContents) != 4 {
		t.Fatalf("contents: %+v %v", withContents, err)
	}
	if !withContents[2].IsUncommitted() || withContents[2].Text != "unsaved edit" || withContents[3].OID != first {
		t.Fatalf("contents lines: %+v", withContents)
	}
	if withContents[0].IsUncommitted() {
		t.Fatal("unchanged line must not be uncommitted")
	}
	put(t, repo, "a.txt", "one\nTWO\nthree\nfour\n")
	disk, err := repo.Blame(ctx, "a.txt", "", nil)
	if err != nil || !disk[3].IsUncommitted() || !strings.HasPrefix(disk[3].OID, "0000") {
		t.Fatalf("worktree blame: %+v %v", disk, err)
	}
	if _, err := repo.Blame(ctx, "missing.txt", "", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestLineLog(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	c1 := commitFile(t, repo, "a.txt", "one\ntwo\nthree\nfour\n", "create")
	c2 := commitFile(t, repo, "a.txt", "one\nTWO\nthree\nfour\n", "change two")
	commitFile(t, repo, "a.txt", "one\nTWO\nthree\nFOUR\n", "change four")
	entries, err := repo.LineLog(ctx, "a.txt", 2, 2)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries: %+v %v", entries, err)
	}
	if entries[0].Commit.OID != c2 || entries[1].Commit.OID != c1 || entries[0].Commit.Subject != "change two" {
		t.Fatalf("commits: %+v", entries)
	}
	if len(entries[0].Diffs) != 1 || entries[0].Diffs[0].Path() != "a.txt" || len(entries[0].Diffs[0].Hunks) == 0 {
		t.Fatalf("diffs: %+v", entries[0].Diffs)
	}
	var added []string
	for _, l := range entries[0].Diffs[0].Hunks[0].Lines {
		if l.Kind == '+' {
			added = append(added, l.Text)
		}
	}
	if !equalStrings(added, []string{"TWO"}) {
		t.Fatalf("added: %v", added)
	}
	if _, err := repo.LineLog(ctx, "a.txt", 0, 1); err == nil {
		t.Error("invalid range must fail")
	}
	if _, err := repo.LineLog(ctx, "a.txt", 3, 2); err == nil {
		t.Error("reversed range must fail")
	}
	if _, err := parseLineLog("garbage"); err == nil {
		t.Error("garbage must fail")
	}
}
