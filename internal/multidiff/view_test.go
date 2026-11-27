package multidiff

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testWidth      = 1100
	testHeight     = 700
	hunkLines      = 12
	testSet        = "set"
	treeScrollDown = 2000
	testFileName   = "file.txt"
)

func patchFor(t testing.TB, path string, changed int) *editor.FilePatch {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n", path, path, path, path, changed, changed)
	for i := range changed {
		fmt.Fprintf(&b, "-old %d\n", i)
	}
	for i := range changed {
		fmt.Fprintf(&b, "+new %d\n", i)
	}
	fps, err := editor.ParsePatch(b.String())
	if err != nil || len(fps) != 1 {
		t.Fatalf("patch: %v %d", err, len(fps))
	}
	return &fps[0]
}

func testEntries(t testing.TB, n int) []Entry {
	entries := make([]Entry, n)
	for i := range entries {
		p := fmt.Sprintf("dir%02d/%s%03d.txt", i/10, "f", i)
		entries[i] = Entry{Key: p, Path: p, Status: token.GitModified, Adds: hunkLines, Dels: hunkLines, Fingerprint: "v1", Patch: patchFor(t, p, hunkLines)}
	}
	return entries
}

type rig struct {
	v       *View
	tt      *ui.Tester
	viewed  *Memory
	onFrame func()
	mu      sync.Mutex
	posted  []func()
}

func (r *rig) post(f func()) {
	r.mu.Lock()
	r.posted = append(r.posted, f)
	r.mu.Unlock()
}

func (r *rig) runPosted() {
	r.mu.Lock()
	posted := r.posted
	r.posted = nil
	r.mu.Unlock()
	for _, f := range posted {
		f()
	}
}

func newRig(t testing.TB, entries []Entry) *rig {
	t.Helper()
	r := &rig{v: New(), viewed: NewMemory()}
	r.v.SetEntries(entries)
	o := Options{Label: "Changes", Set: testSet, Ctx: context.Background(), Post: r.post, Viewed: r.viewed}
	r.tt = ui.NewTester(func(c *ui.Context) {
		r.runPosted()
		r.v.Render(c, o)
		if r.onFrame != nil {
			r.onFrame()
		}
	}, testWidth, testHeight)
	r.tt.Frame()
	r.tt.Frame()
	return r
}

func (r *rig) settle() {
	for range 4 {
		r.tt.Frame()
	}
}

func TestRevealScrollsToTheFileHeader(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 30)
	r := newRig(t, entries)
	target := entries[20]
	r.v.Reveal(target.Key)
	r.settle()
	if got := r.v.Top(); got != target.Key {
		t.Fatalf("top file = %q, want %q", got, target.Key)
	}
	header, ok := r.tt.AccessByRole(ui.RoleGroup, target.Path)
	if !ok {
		t.Fatalf("no header for %s: %v", target.Path, r.tt.Texts())
	}
	list, ok := r.tt.AccessByRole(ui.RoleList, "Diffs of Changes")
	if !ok {
		t.Fatal("no list")
	}
	if header.Bounds.Y < list.Bounds.Y || header.Bounds.Y > list.Bounds.Y+token.HeaderHeight.Px() {
		t.Errorf("header at y %.1f, list top %.1f", header.Bounds.Y, list.Bounds.Y)
	}
}

func TestHeaderSticksWhileTheDiffScrolls(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 4)
	for i := range entries {
		entries[i].Patch = patchFor(t, entries[i].Path, 80)
		entries[i].Adds, entries[i].Dels = 80, 80
	}
	r := newRig(t, entries)
	list, _ := r.tt.AccessByRole(ui.RoleList, "Diffs of Changes")
	r.tt.Scroll(list.Bounds.X+list.Bounds.W/2, list.Bounds.Y+list.Bounds.H/2, 0, -300)
	r.settle()
	first := r.v.Top()
	header, ok := r.tt.AccessByRole(ui.RoleGroup, first)
	if !ok {
		t.Fatalf("header of %s scrolled away: %v", first, r.tt.Texts())
	}
	if header.Bounds.Y > list.Bounds.Y+token.HeaderHeight.Px() {
		t.Errorf("header at y %.1f, list top %.1f", header.Bounds.Y, list.Bounds.Y)
	}
	if first != entries[0].Key {
		t.Errorf("scrolled 300 DIPs and the top file is %s", first)
	}
}

func TestCollapseHidesTheDiff(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 3)
	r := newRig(t, entries)
	if r.v.Held() == 0 {
		t.Fatal("no diff built")
	}
	rowsBefore := len(r.v.rows)
	if err := r.tt.Click("Collapse " + entries[0].Path); err != nil {
		t.Fatal(err)
	}
	r.settle()
	if len(r.v.rows) != rowsBefore-1 {
		t.Errorf("rows %d, want %d", len(r.v.rows), rowsBefore-1)
	}
	if err := r.tt.Click("Collapse All"); err != nil {
		t.Fatal(err)
	}
	r.settle()
	if len(r.v.rows) != len(entries) {
		t.Errorf("collapse all left %d rows for %d files", len(r.v.rows), len(entries))
	}
	if err := r.tt.Click("Expand All"); err != nil {
		t.Fatal(err)
	}
	r.settle()
	if len(r.v.rows) != 2*len(entries) {
		t.Errorf("expand all left %d rows", len(r.v.rows))
	}
}

func TestViewedCollapsesAndIsRememberedPerContent(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 3)
	r := newRig(t, entries)
	box, ok := r.tt.AccessByRole(ui.RoleCheckBox, "Viewed")
	if !ok {
		t.Fatalf("no Viewed checkbox: %v", r.tt.Texts())
	}
	r.tt.ClickAt(box.Bounds.X+box.Bounds.W/2, box.Bounds.Y+box.Bounds.H/2)
	r.settle()
	if !r.viewed.IsViewed(testSet, &entries[0]) {
		t.Fatal("the file is not marked viewed")
	}
	if r.v.isOpen(&entries[0]) {
		t.Error("a viewed file stays expanded")
	}
	again := newRig(t, testEntries(t, 3))
	again.v.opts.Viewed = r.viewed
	if !again.v.viewed(&again.v.entries[0]) {
		t.Error("a new view of the same content forgot Viewed")
	}
	changed := testEntries(t, 3)
	changed[0].Fingerprint = "v2"
	if r.viewed.IsViewed(testSet, &changed[0]) {
		t.Error("changed content is still viewed")
	}
}

func TestTreeFollowsTheTopFile(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 30)
	r := newRig(t, entries)
	r.v.Reveal(entries[25].Key)
	r.settle()
	row, ok := r.v.tree.fileRow[entries[25].Key]
	if !ok || r.v.tree.sel != row {
		t.Fatalf("tree row %d, selected %d", row, r.v.tree.sel)
	}
	if err := r.tt.Click("f003.txt"); err != nil {
		t.Fatal(err)
	}
	r.settle()
	if got := r.v.Top(); got != entries[3].Key {
		t.Errorf("clicking the tree row scrolled to %q", got)
	}
}

func TestTreeCompactsSingleChildFolders(t *testing.T) {
	t.Parallel()
	const deep = "a/b/c/d/leaf.txt"
	entries := []Entry{
		{Key: deep, Path: deep, Status: token.GitModified, Adds: 1, Dels: 1, Fingerprint: "v1", Patch: patchFor(t, deep, 1)},
		{Key: "a/b/c/e/other.txt", Path: "a/b/c/e/other.txt", Status: token.GitModified, Adds: 1, Dels: 1, Fingerprint: "v1", Patch: patchFor(t, "a/b/c/e/other.txt", 1)},
		{Key: "top.txt", Path: "top.txt", Status: token.GitModified, Adds: 1, Dels: 1, Fingerprint: "v1", Patch: patchFor(t, "top.txt", 1)},
	}
	r := newRig(t, entries)
	want := []treeRow{
		{name: "a/b/c", path: "a/b/c", depth: 0, dir: true, entry: -1},
		{name: "d", path: "a/b/c/d", depth: 1, dir: true, entry: -1},
		{name: "leaf.txt", path: deep, depth: 2, entry: 0},
		{name: "e", path: "a/b/c/e", depth: 1, dir: true, entry: -1},
		{name: "other.txt", path: "a/b/c/e/other.txt", depth: 2, entry: 1},
		{name: "top.txt", path: "top.txt", depth: 0, entry: 2},
	}
	if len(r.v.tree.rows) != len(want) {
		t.Fatalf("tree rows = %+v", r.v.tree.rows)
	}
	for i, w := range want {
		if r.v.tree.rows[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, r.v.tree.rows[i], w)
		}
	}
	if _, ok := r.tt.AccessByRole(ui.RoleTreeItem, "a/b/c"); !ok {
		t.Errorf("no compacted folder row: %v", r.tt.Texts())
	}
}

func TestFileCardsAreSeparatedByAGap(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 2)
	r := newRig(t, entries)
	second, ok := r.tt.AccessByRole(ui.RoleGroup, entries[1].Path)
	if !ok {
		t.Fatalf("no header of the second file: %v", r.tt.Texts())
	}
	diff, ok := r.tt.AccessByRole(ui.RoleGroup, diffLabelPrefix+entries[0].Path)
	if !ok {
		t.Fatalf("no diff of the first file: %v", r.tt.Texts())
	}
	if gap := second.Bounds.Y - (diff.Bounds.Y + diff.Bounds.H); gap < token.GapGroup.Px() {
		t.Errorf("gap %.1f between the diff and the next header", gap)
	}
}

func TestTreeFilterShowsTheRowsOfTheTextInTheSameFrame(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 30)
	r := newRig(t, entries)
	input, ok := r.tt.Access(filterLabel)
	if !ok {
		t.Fatal("no filter field")
	}
	r.tt.ClickAt(input.Bounds.X+input.Bounds.W/2, input.Bounds.Y+input.Bounds.H/2)
	r.settle()
	var stale, frames int
	r.onFrame = func() {
		frames++
		if r.v.tree.built != r.v.tree.filter {
			stale++
		}
	}
	for _, ch := range "f01" {
		r.tt.Type(string(ch))
		r.settle()
	}
	if frames == 0 || r.v.tree.filter != "f01" {
		t.Fatalf("filter %q after %d frames", r.v.tree.filter, frames)
	}
	if stale != 0 {
		t.Fatalf("%d of %d frames drew a tree built for an older filter", stale, frames)
	}
}

func TestTreeFilterScrollsTheTreeToTheTop(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 200)
	r := newRig(t, entries)
	tree, ok := r.tt.AccessByRole(ui.RoleList, treeLabel)
	if !ok {
		t.Fatal("no tree")
	}
	r.tt.Scroll(tree.Bounds.X+tree.Bounds.W/2, tree.Bounds.Y+tree.Bounds.H/2, 0, treeScrollDown)
	r.settle()
	if first, _ := r.v.tree.list.Visible(); first == 0 {
		t.Fatal("the tree did not scroll")
	}
	input, _ := r.tt.Access(filterLabel)
	r.tt.ClickAt(input.Bounds.X+input.Bounds.W/2, input.Bounds.Y+input.Bounds.H/2)
	r.tt.Type("f1")
	r.settle()
	if first, _ := r.v.tree.list.Visible(); first != 0 {
		t.Fatalf("the filtered tree starts at row %d", first)
	}
}

func TestOnlyFilesNearTheViewportBuildDiffs(t *testing.T) {
	t.Parallel()
	const files = 600
	entries := testEntries(t, files)
	r := newRig(t, entries)
	if built := r.v.Stats.Built; built == 0 || built > 6 {
		t.Fatalf("built %d diffs for the first screen of %d files", built, files)
	}
	list, _ := r.tt.AccessByRole(ui.RoleList, "Diffs of Changes")
	for range 60 {
		r.tt.Scroll(list.Bounds.X+list.Bounds.W/2, list.Bounds.Y+list.Bounds.H/2, 0, -2000)
		r.tt.Frame()
	}
	if held := r.v.Held(); held > MaxHeld {
		t.Errorf("held %d diffs, the bound is %d", held, MaxHeld)
	}
	if r.v.Stats.Built >= files {
		t.Errorf("built %d diffs", r.v.Stats.Built)
	}
}

func TestLargeFilesWaitForLoadDiff(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 2)
	entries[0].Adds = LargeDiffLines + 1
	r := newRig(t, entries)
	if r.v.files[entries[0].Key].state != nil {
		t.Fatal("a large diff was built")
	}
	if err := r.tt.Click(loadDiffLabel); err != nil {
		t.Fatalf("no %s: %v", loadDiffLabel, err)
	}
	r.settle()
	if r.v.files[entries[0].Key].state == nil {
		t.Error("Load Diff did not build the diff")
	}
}

func TestBinaryFilesStartCollapsed(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 2)
	entries[1].Binary, entries[1].Patch = true, nil
	r := newRig(t, entries)
	if r.v.isOpen(&entries[1]) {
		t.Error("a binary file starts expanded")
	}
}

func TestChangeKeysStepThroughHunks(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 5)
	r := newRig(t, entries)
	r.tt.Key(0, ui.KeyF7)
	r.settle()
	if r.v.nav.key != entries[0].Key {
		t.Fatalf("first F7 aimed at %q", r.v.nav.key)
	}
	r.tt.Key(0, ui.KeyF7)
	r.settle()
	if r.v.nav.key != entries[1].Key {
		t.Fatalf("second F7 aimed at %q", r.v.nav.key)
	}
	r.tt.Key(ui.Shift, ui.KeyF7)
	r.settle()
	if r.v.nav.key != entries[0].Key {
		t.Errorf("Shift+F7 aimed at %q", r.v.nav.key)
	}
}

func TestFileKeysStepThroughFiles(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 5)
	r := newRig(t, entries)
	r.tt.Key(ui.Alt, ui.KeyF5)
	r.settle()
	if got := r.v.Top(); got != entries[1].Key {
		t.Errorf("Alt+F5 moved to %q", got)
	}
}

func BenchmarkScrollFrame(b *testing.B) {
	const (
		files     = 600
		wheelStep = 160
		turnAfter = 150
	)
	r := newRig(b, testEntries(b, files))
	list, _ := r.tt.AccessByRole(ui.RoleList, "Diffs of Changes")
	x, y := list.Bounds.X+list.Bounds.W/2, list.Bounds.Y+list.Bounds.H/2
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		step := -wheelStep
		if i/turnAfter%2 == 1 {
			step = wheelStep
		}
		r.tt.Scroll(x, y, 0, float32(step))
		r.tt.Frame()
	}
	b.ReportMetric(float64(r.v.Held()), "held")
	b.ReportMetric(float64(r.v.Stats.Built), "built")
}

func TestCurrentPathIsTheTopFile(t *testing.T) {
	t.Parallel()
	entries := testEntries(t, 30)
	r := newRig(t, entries)
	if _, ok := New().CurrentPath(); ok {
		t.Error("an empty view has a current file")
	}
	r.v.Reveal(entries[20].Key)
	r.settle()
	if got, ok := r.v.CurrentPath(); !ok || got != entries[20].Path {
		t.Errorf("CurrentPath = %q, %v, want %q", got, ok, entries[20].Path)
	}
}

func TestFilesFollowTheTreeOrderWithinEachSection(t *testing.T) {
	file := func(p string) Entry { return Entry{Key: p, Path: p, Fingerprint: "v1", Patch: patchFor(t, p, 1)} }
	section := Entry{Key: "section", Title: "Second", Section: func(*ui.Context) {}}
	r := newRig(t, []Entry{file("c.go"), file("a/x.go"), file("b.go"), file("a/b/y.go"), section, file("z.go"), file("d/e.go")})
	var got []string
	for _, e := range r.v.Entries() {
		got = append(got, e.Key)
	}
	want := []string{"a/b/y.go", "a/x.go", "b.go", "c.go", "section", "d/e.go", "z.go"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	r.tt.Frame()
	var rows []string
	for _, row := range r.v.tree.rows {
		rows = append(rows, row.name)
	}
	wantRows := []string{"a", "b", "y.go", "x.go", "b.go", "c.go", "Second", "d", "e.go", "z.go"}
	if strings.Join(rows, " ") != strings.Join(wantRows, " ") {
		t.Fatalf("tree rows = %v, want %v", rows, wantRows)
	}
}

func TestChangedFileKeepsItsDiffWhileItReloads(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	load := func(newText string, wait chan struct{}) func(context.Context, bool) (string, string, error) {
		return func(context.Context, bool) (string, string, error) {
			if wait != nil {
				<-wait
			}
			return "old\n", newText, nil
		}
	}
	entry := Entry{Key: testFileName, Path: testFileName, Status: token.GitModified, Adds: 1, Dels: 1, Fingerprint: "v1", Load: load("first\n", nil)}
	r := newRig(t, []Entry{entry})
	settled := func() {
		for range 20 {
			r.tt.Frame()
		}
	}
	shows := func(text string) bool {
		d := r.v.File(testFileName).State()
		return d != nil && d.New != nil && d.New.String() == text
	}
	settled()
	if !shows("first\n") {
		t.Fatal("the first diff did not show")
	}
	entry.Fingerprint, entry.Load = "v2", load("second\n", gate)
	r.v.SetEntries([]Entry{entry})
	settled()
	if _, loading := r.tt.AccessByRole(ui.RoleProgress, loadingLabel); loading {
		t.Error("the reload replaced the diff with a loading state")
	}
	if !shows("first\n") {
		t.Error("the old diff is gone while the new one loads")
	}
	close(gate)
	for range 200 {
		r.tt.Frame()
		if shows("second\n") {
			return
		}
	}
	t.Error("the new diff did not show")
}
