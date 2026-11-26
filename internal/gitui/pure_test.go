package gitui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

func TestFormatHelpers(t *testing.T) {
	if got := shortHash("0123456789abcdef"); got != "01234567" {
		t.Fatalf("shortHash = %q", got)
	}
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	cases := map[time.Time]string{
		now.Add(-time.Hour):                         "Today 11:00",
		now.Add(-20 * time.Hour):                    "Yesterday 16:00",
		time.Date(2026, 1, 2, 9, 5, 0, 0, time.UTC): "Jan 2 09:05",
		time.Date(2024, 1, 2, 9, 5, 0, 0, time.UTC): "Jan 2, 2024",
	}
	for in, want := range cases {
		if got := formatTime(in, now); got != want {
			t.Errorf("formatTime(%v) = %q, want %q", in, got, want)
		}
	}
	if got := syncLabel(2, 3); got != "↑2 ↓3" {
		t.Fatalf("syncLabel = %q", got)
	}
	if got := syncLabel(0, 0); got != "" {
		t.Fatalf("syncLabel(0,0) = %q", got)
	}
}

func TestRefBadges(t *testing.T) {
	got := refBadges([]string{"HEAD -> main", "origin/main", "tag: v1", "feature"}, map[string]bool{"origin": true})
	want := []refBadge{{"main", refHead}, {"origin/main", refRemote}, {"v1", refTag}, {"feature", refLocal}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("badge %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestAppendIgnore(t *testing.T) {
	if got := appendIgnore("", "a/b.txt"); got != "/a/b.txt\n" {
		t.Fatalf("got %q", got)
	}
	if got := appendIgnore("x", "y"); got != "x\n/y\n" {
		t.Fatalf("got %q", got)
	}
	if got := appendIgnore("/y\n", "y"); got != "/y\n" {
		t.Fatalf("duplicate added: %q", got)
	}
}

func testStatus() *git.Status {
	return &git.Status{Branch: "main", OID: "abc", Files: []git.FileStatus{
		{Path: "staged.go", Index: 'M', WorkTree: '.'},
		{Path: "both.go", Index: 'M', WorkTree: 'M'},
		{Path: "work.go", Index: '.', WorkTree: 'M'},
		{Path: "new.go", Index: '?', WorkTree: '?'},
		{Path: "conflict.go", Index: 'U', WorkTree: 'U', Conflict: true},
	}}
}

func TestBuildRows(t *testing.T) {
	st := testStatus()
	rows := buildRows(st, [groupCount]bool{})
	var headers []string
	for _, r := range rows {
		if r.header {
			headers = append(headers, groupTitles[r.group])
		}
	}
	if strings.Join(headers, ",") != "Merge Conflicts,Staged,Changes,Unversioned Files" {
		t.Fatalf("headers = %v", headers)
	}
	collapsed := buildRows(st, [groupCount]bool{groupStaged: true})
	for _, r := range collapsed {
		if !r.header && r.group == groupStaged {
			t.Fatalf("collapsed group shows %s", r.file.Path)
		}
	}
	if buildRows(nil, [groupCount]bool{}) != nil {
		t.Fatal("rows for nil status")
	}
}

func TestPlanRollback(t *testing.T) {
	rows := []changeRow{
		{group: groupUntracked, file: git.FileStatus{Path: "u"}},
		{group: groupUnstaged, file: git.FileStatus{Path: "w"}},
		{group: groupStaged, file: git.FileStatus{Path: "a", Index: 'A'}},
		{group: groupStaged, file: git.FileStatus{Path: "r", OrigPath: "o", Index: 'R'}},
		{group: groupStaged, file: git.FileStatus{Path: "m", Index: 'M'}},
		{group: groupConflicts, file: git.FileStatus{Path: "c"}},
	}
	p := planRollback(rows)
	check := func(name string, got []string, want string) {
		if strings.Join(got, ",") != want {
			t.Errorf("%s = %v, want %s", name, got, want)
		}
	}
	check("untracked", p.untracked, "u")
	check("discard", p.discard, "w,o,m")
	check("unstage", p.unstage, "a,r,o,m")
}

func TestParseNameStatus(t *testing.T) {
	files, err := parseNameStatus("M\x00a.go\x00R100\x00old.go\x00new.go\x00D\x00gone.go\x00")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || files[1].OrigPath != "old.go" || files[1].Path != "new.go" || files[2].Status != 'D' {
		t.Fatalf("files = %+v", files)
	}
	if _, err := parseNameStatus("R100\x00only\x00"); err == nil {
		t.Fatal("truncated rename accepted")
	}
	if files, err := parseNameStatus(""); err != nil || len(files) != 0 {
		t.Fatalf("empty = %v, %v", files, err)
	}
}

func TestCheckName(t *testing.T) {
	for _, bad := range []string{"", "-x", "a b", "a\nb"} {
		if checkName(bad) == nil {
			t.Errorf("checkName(%q) accepted", bad)
		}
	}
	if checkName("feature/x") != nil {
		t.Error("valid name rejected")
	}
	if checkURL("-u") == nil || checkURL("https://example.com/r.git") != nil {
		t.Error("checkURL wrong")
	}
}

func TestBranchEntries(t *testing.T) {
	now := time.Now()
	branches := []git.Branch{
		{Name: "main", Current: true, Time: now},
		{Name: "zeta", Time: now.Add(-time.Hour)},
		{Name: "alpha", Time: now.Add(-2 * time.Hour)},
		{Name: "origin/HEAD", Remote: true},
		{Name: "origin/main", Remote: true},
	}
	tags := []git.Tag{{Name: "v1"}, {Name: "v2"}}
	entries := buildBranchEntries(branches, tags, "")
	var flat []string
	for _, e := range entries {
		if e.header {
			flat = append(flat, "#"+e.title)
		} else {
			flat = append(flat, e.name)
		}
	}
	want := "#Recent,zeta,alpha,#Local,alpha,main,zeta,#Remote,origin/main,#Tags,v2,v1"
	if strings.Join(flat, ",") != want {
		t.Fatalf("entries = %s", strings.Join(flat, ","))
	}
	keys := map[string]bool{}
	for _, e := range entries {
		if keys[e.key()] {
			t.Fatalf("duplicate key %s", e.key())
		}
		keys[e.key()] = true
	}
	filtered := buildBranchEntries(branches, tags, "alp")
	if len(filtered) != 2 || filtered[1].name != "alpha" {
		t.Fatalf("filtered = %+v", filtered)
	}
	if i := nextEntry(entries, -1, 1); i != 1 {
		t.Fatalf("first entry = %d", i)
	}
	if i := nextEntry(entries, 2, 1); i != 4 {
		t.Fatalf("next entry skips header: %d", i)
	}
	if i := nextEntry(entries, 1, -1); i != 1 {
		t.Fatalf("previous at top = %d", i)
	}
}

func TestMergeModel(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nB1\nc\n"
	theirs := "a\nB2\nc\nd\n"
	m := newMergeModel(base, ours, theirs)
	if m.unresolved() != 1 {
		t.Fatalf("unresolved = %d", m.unresolved())
	}
	if m.clean() {
		t.Fatal("conflict reported clean")
	}
	if got := m.result(); got != "a\nb\nc\nd\n" {
		t.Fatalf("auto result = %q", got)
	}
	conflict := -1
	for i, ch := range m.chunks {
		if ch.Kind == git.ChunkConflict {
			conflict = i
		}
	}
	m.choose(conflict, choiceOurs)
	if got := m.result(); got != "a\nB1\nc\nd\n" {
		t.Fatalf("ours result = %q", got)
	}
	m.choose(conflict, choiceTheirs)
	if got := m.result(); got != "a\nB1\nB2\nc\nd\n" {
		t.Fatalf("both result = %q", got)
	}
	m.acceptAll(false)
	if got := m.result(); got != theirs {
		t.Fatalf("accept right = %q", got)
	}
	r := m.ranges(sideTheirs)
	if r[len(r)-1].end != 4 {
		t.Fatalf("ranges = %v", r)
	}
	if !newMergeModel(base, "a\nb\nc\nx\n", "z\nb\nc\n").clean() {
		t.Fatal("clean merge reported conflicts")
	}
}

func TestTodo(t *testing.T) {
	items := []git.TodoItem{{Action: git.ActionPick, OID: "a1"}, {Action: git.ActionPick, OID: "b2"}, {Action: git.ActionPick, OID: "c3"}}
	actions := []string{"pick", "fixup", "drop"}
	if _, err := buildTodo(items, actions); err != nil {
		t.Fatal(err)
	}
	if _, err := buildTodo(items, []string{"drop", "squash", "pick"}); !errors.Is(err, errSquashFirst) {
		t.Fatalf("err = %v", err)
	}
	if j := moveTodo(items, actions, 0, 1); j != 1 || items[1].OID != "a1" || actions[1] != "pick" {
		t.Fatalf("move = %d %v %v", j, items, actions)
	}
	if j := moveTodo(items, actions, 0, -1); j != 0 {
		t.Fatalf("move above top = %d", j)
	}
	key, ok := setTodoAction(items, "c3ffff", git.ActionReword)
	if !ok || key != "c3" || items[2].Action != git.ActionReword {
		t.Fatalf("setTodoAction = %q %v %v", key, ok, items)
	}
}

func TestShelfRoom(t *testing.T) {
	if err := checkShelfRoom(shelfCapBytes-10, 5); err != nil {
		t.Fatal(err)
	}
	err := checkShelfRoom(shelfCapBytes-10, 20)
	var full errShelfFull
	if !errors.As(err, &full) || !strings.Contains(err.Error(), "200 MB") {
		t.Fatalf("err = %v", err)
	}
}

func TestLogHelpers(t *testing.T) {
	if !isHashLike("abc123") || isHashLike("abc") || isHashLike("xyz123") {
		t.Fatal("isHashLike wrong")
	}
	page := []git.Commit{{OID: "a"}, {OID: "b"}}
	if got := prependMissing([]git.Commit{{OID: "b"}}, page); len(got) != 2 {
		t.Fatalf("duplicate prepended: %v", got)
	}
	if got := prependMissing([]git.Commit{{OID: "c"}}, page); got[0].OID != "c" {
		t.Fatalf("hit not first: %v", got)
	}
	s := newLogState(nil, "")
	if o := s.options(10); !o.All || o.Skip != 10 || o.Max != defaultLogPageSize {
		t.Fatalf("options = %+v", o)
	}
	s.applied = logFilter{branch: "dev", author: "ada", path: "x"}
	if o := s.options(0); o.All || o.Revs[0] != "dev" || o.Author != "ada" || o.Paths[0] != "x" {
		t.Fatalf("filtered options = %+v", o)
	}
	if s.showsGraph() {
		t.Fatal("graph shown with a path filter")
	}
	h := newLogState(nil, "f.go")
	if o := h.options(0); o.All || !o.Follow || o.Paths[0] != "f.go" {
		t.Fatalf("history options = %+v", o)
	}
	s.commits = []git.Commit{{OID: "c", Parents: []string{"b"}}, {OID: "b", Parents: []string{"a"}}, {OID: "a"}}
	if s.childOf("a") != 1 || s.childOf("c") != -1 {
		t.Fatal("childOf wrong")
	}
}

func TestGraphWidth(t *testing.T) {
	if graphWidth(nil) != laneWidth+graphPadding {
		t.Fatal("empty graph width")
	}
	if graphWidth([]git.GraphRow{{Width: maxLanes * 2}}) != maxLanes*laneWidth+graphPadding {
		t.Fatal("graph width not capped")
	}
}

func TestBlameLabel(t *testing.T) {
	now := time.Now()
	bl := git.BlameLine{OID: "0123456789", Author: "Ada", Time: now, Summary: "Fix"}
	if got := blameLabel(bl, blameOptions{}, now); !strings.HasPrefix(got, "Ada, Today") || !strings.HasSuffix(got, " · Fix") {
		t.Fatalf("label = %q", got)
	}
	if got := blameLabel(bl, blameOptions{hideAuthor: true, hideDate: true, showHash: true, hideSummary: true}, now); got != "01234567" {
		t.Fatalf("label = %q", got)
	}
	if got := blameLabel(git.BlameLine{OID: strings.Repeat("0", 40)}, blameOptions{}, now); got != notCommittedText {
		t.Fatalf("uncommitted label = %q", got)
	}
}

func TestLineChange(t *testing.T) {
	base := editor.NewBuffer("a\nb\nc\nd\n")
	buf := editor.NewBuffer("a\nB\nc\nd\ne\n")
	d := &editor.DiffState{Old: base, New: buf}
	hunks := d.Hunks()
	if len(hunks) != 2 {
		t.Fatalf("hunks = %+v", hunks)
	}
	if hunkAt(hunks, 1) != 0 || hunkAt(hunks, 4) != 1 || hunkAt(hunks, 2) != -1 {
		t.Fatalf("hunkAt wrong for %+v", hunks)
	}
	rollbackHunk(buf, base, hunks[0])
	if buf.String() != "a\nb\nc\nd\ne\n" {
		t.Fatalf("after rollback = %q", buf.String())
	}
}

func TestFileDiffItems(t *testing.T) {
	fd := git.FileDiff{OldPath: "a.txt", NewPath: "a.txt", Hunks: []git.Hunk{{OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1,
		Lines: []git.Line{{Kind: '-', Text: "x"}, {Kind: '+', Text: "y"}}}}}
	items := fileDiffItems([]git.FileDiff{fd, {OldPath: "b.bin", NewPath: "b.bin", Binary: true}})
	if len(items) != 2 || items[0].patch == nil || items[0].code != 'M' || items[1].note == "" {
		t.Fatalf("items = %+v", items)
	}
}
