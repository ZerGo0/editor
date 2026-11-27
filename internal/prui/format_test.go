package prui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui/editor"
)

func TestRelativeTime(t *testing.T) {
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{50 * time.Hour, "2d ago"},
		{15 * 24 * time.Hour, "2w ago"},
		{400 * 24 * time.Hour, "1y ago"},
	}
	for _, tc := range cases {
		if got := relativeTime(testNow, testNow.Add(-tc.ago)); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.ago, got, tc.want)
		}
	}
	if relativeTime(testNow, time.Time{}) != "" {
		t.Error("zero time must give no text")
	}
}

func TestSummarizeChecks(t *testing.T) {
	ok := github.Check{Status: "completed", Conclusion: "success"}
	bad := github.Check{Status: "completed", Conclusion: "failure"}
	run := github.Check{Status: "in_progress"}
	skipped := github.Check{Status: "completed", Conclusion: "skipped"}
	cases := []struct {
		checks []github.Check
		want   checkState
	}{
		{nil, checkUnknown},
		{[]github.Check{ok, skipped}, checkSuccess},
		{[]github.Check{ok, run}, checkPending},
		{[]github.Check{run, bad, ok}, checkFailure},
	}
	for i, tc := range cases {
		if got := summarizeChecks(tc.checks); got != tc.want {
			t.Errorf("case %d: %v, want %v", i, got, tc.want)
		}
	}
	if countChecks([]github.Check{ok, bad, skipped}) != 2 {
		t.Error("countChecks")
	}
}

func TestParsePRNumber(t *testing.T) {
	good := map[string]int{"12": 12, " #7 ": 7, "https://github.com/a/b/pull/99": 99, "https://github.com/a/b/pull/5/files": 5}
	for in, want := range good {
		if got, err := parsePRNumber(in); err != nil || got != want {
			t.Errorf("%q: %d %v", in, got, err)
		}
	}
	for _, in := range []string{"", "abc", "0", "-3", "https://github.com/a/b/issues"} {
		if _, err := parsePRNumber(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestPendingAndLatestReviews(t *testing.T) {
	reviews := []github.Review{
		{ID: "1", Author: "a", State: reviewChangesRequested, Submitted: testNow.Add(-time.Hour)},
		{ID: "2", Author: "a", State: reviewApproved, Submitted: testNow},
		{ID: "3", Author: "b", State: reviewPending},
	}
	if pendingReviewID(reviews) != "3" {
		t.Error("pending review")
	}
	if got := latestReviewStates(reviews); !reflect.DeepEqual(got, map[string]string{"a": reviewApproved}) {
		t.Errorf("%v", got)
	}
}

func TestPickRemoteAndBranch(t *testing.T) {
	remotes := []git.Remote{
		{Name: "zed", FetchURL: "git@github.com:zed/widgets.git"},
		{Name: "origin", FetchURL: "https://github.com/acme/widgets.git"},
		{Name: "gitlab", FetchURL: "git@gitlab.com:acme/widgets.git"},
	}
	repo, byName, ok := pickRemote(remotes)
	if !ok || repo.Owner != "acme" || repo.Host != "github.com" || len(byName) != 3 {
		t.Fatalf("%+v %v %v", repo, byName, ok)
	}
	if _, _, ok := pickRemote([]git.Remote{{Name: "x", FetchURL: "not a url"}}); ok {
		t.Error("an invalid remote was accepted")
	}
	only := []git.Remote{{Name: "zed", FetchURL: "git@github.com:zed/widgets.git"}}
	if repo, _, ok := pickRemote(only); !ok || repo.Owner != "zed" {
		t.Errorf("fallback remote: %+v %v", repo, ok)
	}
	branches := []git.Branch{{Name: "topic", Current: true, Upstream: "zed/topic-x", Subject: "S", OID: "abc"}, {Name: "origin/main", Remote: true}}
	info := currentBranch(branches, repo, byName)
	if info.HeadRef != "topic-x" || info.HeadOwner != "zed" || info.Remote != "zed" || info.OID != "abc" {
		t.Errorf("%+v", info)
	}
	if headSpec(repo, info) != "zed:topic-x" {
		t.Errorf("head spec %q", headSpec(repo, info))
	}
	if got := currentBranch(nil, repo, byName); got.Name != "" {
		t.Errorf("detached: %+v", got)
	}
}

func TestRemoteBranchNames(t *testing.T) {
	branches := []git.Branch{
		{Name: "origin/zeta", Remote: true}, {Name: "origin/main", Remote: true}, {Name: "origin/feature", Remote: true},
		{Name: "origin/develop", Remote: true}, {Name: "fork/main", Remote: true}, {Name: "local", Current: true},
	}
	got := remoteBranchNames(branches, "origin", "feature")
	if !reflect.DeepEqual(got, []string{"main", "develop", "zeta"}) {
		t.Errorf("%v", got)
	}
	if remoteNameFor(map[string]github.Remote{"a": {Owner: "x"}, "b": {Owner: "x"}}, github.Remote{Owner: "x"}) != "a" {
		t.Error("remoteNameFor must be deterministic")
	}
}

func TestPRStateLabelsAndIDs(t *testing.T) {
	cases := map[string]github.PullRequest{
		prStateOpen:   {State: github.StateOpen},
		prStateDraft:  {State: github.StateOpen, Draft: true},
		prStateMerged: {State: github.StateClosed, Merged: true},
		prStateClosed: {State: github.StateClosed},
	}
	for want, pr := range cases {
		if got := prStateLabel(pr); got != want {
			t.Errorf("%q, want %q", got, want)
		}
	}
	repo := github.Remote{Owner: "o", Name: "r"}
	if detailID(repo, 3) != "pr/o/r/3" || reviewID(repo, 3, diffSpec{mode: modePR}, "") != "pr/o/r/3/diff" || detailTitle(3, "") != "#3" || shortSHA("0123456789") != "0123456" {
		t.Error("ids")
	}
	if reviewID(repo, 3, diffSpec{mode: modeSince}, "x") != "pr/o/r/3/since" {
		t.Error("diff tab id")
	}
	for in, want := range map[string]string{statusAdded: "A", statusRemoved: "D", statusRenamed: "R", statusModified: "M", "weird": "?"} {
		if fileStatusLetter(in) != want {
			t.Errorf("letter of %s", in)
		}
	}
}

func TestEventSentenceCoversTimelineKinds(t *testing.T) {
	cases := map[string]string{
		"LabeledEvent":            "added the x label",
		"AssignedEvent":           "assigned x",
		"ReviewRequestedEvent":    "requested a review from x",
		"HeadRefForcePushedEvent": "force-pushed the branch (x)",
		"MergedEvent":             "merged commit x",
		"ClosedEvent":             "closed this pull request",
		"AutoMergeEnabledEvent":   "enabled auto-merge",
		"RenamedTitleEvent":       "changed the title from x",
		"HeadRefDeletedEvent":     "deleted the branch x",
	}
	for kind, want := range cases {
		if got := eventSentence(TimelineEvent{Kind: kind, Detail: "x"}); got != want {
			t.Errorf("%s: %q, want %q", kind, got, want)
		}
	}
}

func TestMergeSummary(t *testing.T) {
	base := func(mut func(d *prData)) *prData {
		d := newPRData(1)
		d.pr = github.PullRequest{State: github.StateOpen}
		d.hasOv = true
		mut(d)
		return d
	}
	cases := []struct {
		name string
		d    *prData
		want string
	}{
		{"merged", base(func(d *prData) { d.pr.Merged = true }), "Merged"},
		{"clean", base(func(d *prData) { d.ov.MergeState = mergeStateClean }), "Ready to merge"},
		{"behind", base(func(d *prData) { d.ov.MergeState = mergeStateBehind }), "This branch is out of date with the base branch"},
		{"conflict", base(func(d *prData) { d.ov.MergeState = mergeStateDirty }), "This branch has conflicts with the base branch"},
		{"review", base(func(d *prData) { d.ov.MergeState, d.ov.ReviewDecision = mergeStateBlocked, decisionRequired }), "Review required before merging"},
		{"changes", base(func(d *prData) { d.ov.MergeState, d.ov.ReviewDecision = mergeStateBlocked, decisionChanges }), "Changes were requested"},
		{"auto", base(func(d *prData) { d.ov.AutoMerge = &AutoMerge{Method: "squash", By: "bob"} }), "Auto-merge enabled (squash) by bob"},
		{"queue", base(func(d *prData) { d.ov.Queue = MergeQueue{InQueue: true, Position: 2, State: "QUEUED"} }), "In the merge queue (position 2, queued)"},
		{"draft", base(func(d *prData) { d.pr.Draft = true }), "Draft: mark it ready for review to merge"},
	}
	for _, tc := range cases {
		if got := mergeSummary(tc.d); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestLastReviewedSHA(t *testing.T) {
	reviews := []github.Review{
		{Author: "me", State: reviewApproved, CommitID: "old", Submitted: testNow.Add(-time.Hour)},
		{Author: "me", State: reviewCommented, CommitID: "newer", Submitted: testNow},
		{Author: "me", State: reviewPending, CommitID: "pending", Submitted: testNow.Add(time.Hour)},
		{Author: "other", State: reviewApproved, CommitID: "theirs", Submitted: testNow.Add(2 * time.Hour)},
	}
	if got := lastReviewedSHA(reviews, "me", "head"); got != "newer" {
		t.Errorf("%q", got)
	}
	if lastReviewedSHA(reviews, "me", "newer") != "" || lastReviewedSHA(reviews, "", "head") != "" || lastReviewedSHA(nil, "me", "head") != "" {
		t.Error("nothing to compare must give no sha")
	}
}

func TestEnrichChecks(t *testing.T) {
	rows := enrichChecks([]github.Check{{Name: "build"}, {Name: "lint"}}, []CheckRun{{Name: "build", Required: true, JobID: 5, RunID: 6}})
	if !rows[0].Required || rows[0].JobID != 5 || rows[0].RunID != 6 || rows[1].Required {
		t.Errorf("%+v", rows)
	}
}

func TestMergeChoices(t *testing.T) {
	d := newPRData(1)
	if len(d.allowedMerges()) != 3 {
		t.Error("without an overview every method is offered")
	}
	d.hasOv, d.ov = true, Overview{AllowSquash: true}
	if got := d.allowedMerges(); len(got) != 1 || got[0].method != github.MergeMethodSquash {
		t.Errorf("%+v", got)
	}
	if defaultMergeIndex(mergeChoices, github.MergeMethodRebase) != 2 || defaultMergeIndex(mergeChoices, "x") != 0 {
		t.Error("default merge index")
	}
}

func TestVisibleFilesAndViewed(t *testing.T) {
	d := newPRData(1)
	d.files = []github.PRFile{{Path: "a"}, {Path: "b"}}
	d.viewed = map[string]bool{"a": true}
	if d.viewedCount() != 1 || len(d.visibleFiles()) != 2 {
		t.Error("viewed count")
	}
	d.hideViewed = true
	if got := d.visibleFiles(); len(got) != 1 || got[0].Path != "b" {
		t.Errorf("%+v", got)
	}
}

func TestLinks(t *testing.T) {
	repo := github.Remote{Host: "github.com", Owner: "acme", Name: "widgets"}
	if got := blobURL(repo, "abc", "src/a b.go", 3, 5); got != "https://github.com/acme/widgets/blob/abc/src/a%20b.go#L3-L5" {
		t.Errorf("%q", got)
	}
	if got := blobURL(github.Remote{Owner: "o", Name: "r"}, "feature/x", "f.go", 7, 7); got != "https://github.com/o/r/blob/feature/x/f.go#L7" {
		t.Errorf("%q", got)
	}
	if got := blobURL(repo, "abc", "f.go", 0, 0); strings.Contains(got, "#") {
		t.Errorf("%q", got)
	}
	if u, ok := devURL(repo, "abc", "f.go", 2, 3); !ok || u != "https://github.dev/acme/widgets/blob/abc/f.go#L2-L3" {
		t.Errorf("%q %v", u, ok)
	}
	if _, ok := devURL(github.Remote{Host: "ghe.example.com"}, "a", "b", 0, 0); ok {
		t.Error("github.dev link for an enterprise host")
	}
	if got := markdownPermalink("f.go", 2, 3, "https://x"); got != "[f.go#L2-L3](https://x)" {
		t.Errorf("%q", got)
	}
	if pullURL(repo, 4) != "https://github.com/acme/widgets/pull/4" {
		t.Error("pull url")
	}
	if markdownLink(github.PullRequest{Number: 1, Title: "T", URL: "u"}) != "[#1 T](u)" {
		t.Error("markdown link")
	}
}

func TestSuggestions(t *testing.T) {
	code, ok := suggestionOf("Try this:\n```suggestion\nfoo()\nbar()\n```\nthanks")
	if !ok || code != "foo()\nbar()" {
		t.Errorf("%q %v", code, ok)
	}
	for _, body := range []string{"no block", "```suggestion", "```suggestion\nunclosed"} {
		if _, ok := suggestionOf(body); ok {
			t.Errorf("%q parsed", body)
		}
	}
	if got := appendSuggestion("hi\n", "x"); got != "hi\n\n```suggestion\nx\n```" {
		t.Errorf("%q", got)
	}
	if got := appendSuggestion("  ", "x"); got != "```suggestion\nx\n```" {
		t.Errorf("%q", got)
	}
	out, err := applySuggestionText("a\nb\nc\n", 2, 3, "B\nC\nD")
	if err != nil || out != "a\nB\nC\nD\n" {
		t.Errorf("%q %v", out, err)
	}
	out, err = applySuggestionText("a\r\nb\r\n", 1, 1, "A")
	if err != nil || out != "A\r\nb\r\n" {
		t.Errorf("%q %v", out, err)
	}
	for _, r := range [][2]int{{0, 1}, {2, 1}, {1, 9}} {
		if _, err := applySuggestionText("a\nb\n", r[0], r[1], "x"); err == nil {
			t.Errorf("range %v accepted", r)
		}
	}
}

func TestExpandQuery(t *testing.T) {
	repo := github.Remote{Owner: "acme", Name: "widgets"}
	got := expandQuery("is:open repo:${owner}/${repository} author:${user} label:bug", repo)
	if got != "is:open author:@me label:bug" {
		t.Errorf("%q", got)
	}
}

func TestSavedQueriesRoundTrip(t *testing.T) {
	path := queriesPath(t.TempDir())
	if qs, err := loadQueries(path); err != nil || qs != nil {
		t.Fatalf("missing file: %v %v", qs, err)
	}
	want := []savedQuery{{Label: "Mine", Query: "author:@me"}, {Label: "Bugs", Query: "label:bug"}}
	if err := saveQueries(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadQueries(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%v %v", got, err)
	}
	if err := os.WriteFile(path, []byte("[{\"label\":\"\",\"query\":\"x\"},{\"label\":\"ok\",\"query\":\"y\"}]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := loadQueries(path); len(got) != 1 || got[0].Label != "ok" {
		t.Errorf("invalid entries kept: %v", got)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadQueries(path); err == nil {
		t.Error("bad json accepted")
	}
	if queriesPath("") != "" || saveQueries("", want) != nil {
		t.Error("an empty data dir must disable persistence")
	}
}

func TestReadTemplate(t *testing.T) {
	root := t.TempDir()
	if readTemplate(root) != "" {
		t.Error("template from nothing")
	}
	dir := filepath.Join(root, ".github")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pull_request_template.md"), []byte("## Summary\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if readTemplate(root) != "## Summary\n" {
		t.Error("template not read")
	}
}

func TestAnnotationsAndHeights(t *testing.T) {
	long := strings.Repeat("x", annotationCharsPerLine*2+1)
	th := github.Thread{ID: "a", Path: "f", Line: 99, Side: github.SideLeft, Comments: []github.Comment{{Body: "one"}, {Body: long + "\nsecond"}}}
	resolved := github.Thread{ID: "b", Path: "f", Line: 1, Resolved: true, Comments: []github.Comment{{Body: "x"}}}
	if wrappedLines("a\nb") != 2 || wrappedLines(long) != 3 {
		t.Error("wrapped lines")
	}
	if threadHeight(resolved, false) != annotationCollapsed || threadHeight(resolved, true) <= annotationCollapsed {
		t.Error("collapsed height")
	}
	if threadHeight(th, false) <= threadHeight(github.Thread{Comments: []github.Comment{{Body: "one"}}}, false) {
		t.Error("height must grow with the text")
	}
	got := buildAnnotations([]github.Thread{th, resolved, {ID: "c", Path: "other"}}, "f", map[string]bool{}, 5, 7)
	if len(got) != 2 || got[0].Side != editor.DiffOld || got[0].Line != 4 || got[1].Side != editor.DiffNew || got[1].Line != 0 {
		t.Errorf("%+v", got)
	}
}

func TestLineComment(t *testing.T) {
	got := lineComment(editor.DiffLineRange{Side: editor.DiffOld, Start: 1, End: 3}, "f", "b")
	want := github.ReviewCommentOptions{Body: "b", Path: "f", Line: 4, StartLine: 2, Side: github.SideLeft}
	if got != want {
		t.Errorf("%+v", got)
	}
	if got := lineComment(editor.DiffLineRange{Side: editor.DiffNew, Start: 2, End: 2}, "f", ""); got.StartLine != 0 || got.Side != github.SideRight {
		t.Errorf("%+v", got)
	}
}

func TestLoadVersionsModes(t *testing.T) {
	hs := newHarness(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "w.go"), []byte("working\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hs.be.contents["headsha:w.go"] = "head\n"
	hs.be.contents["basesha:w.go"] = "base\n"
	hs.be.contents["since:w.go"] = "since\n"
	src := fileSource{be: hs.be, repo: github.Remote{Owner: "a", Name: "b"}, root: root}
	pr := github.PullRequest{HeadSHA: "headsha", BaseRef: "main"}
	f := github.PRFile{Path: "w.go", Status: statusModified}
	ctx := t.Context()
	v, err := loadVersions(ctx, src, pr, f, diffSpec{mode: modePR}, "")
	if err != nil || v.old != "base\n" || v.new != "head\n" || v.baseSHA != "basesha" {
		t.Errorf("pr: %+v %v", v, err)
	}
	v, err = loadVersions(ctx, src, pr, f, diffSpec{mode: modeSince, oldRef: "since"}, "")
	if err != nil || v.old != "since\n" || v.new != "head\n" {
		t.Errorf("since: %+v %v", v, err)
	}
	v, err = loadVersions(ctx, src, pr, f, diffSpec{mode: modeLocal}, "")
	if err != nil || v.old != "head\n" || v.new != "working\n" {
		t.Errorf("local: %+v %v", v, err)
	}
	v, err = loadVersions(ctx, src, pr, github.PRFile{Path: "gone.go", Status: statusAdded}, diffSpec{mode: modeLocal}, "")
	if err != nil || v.new != "" || v.old != "" {
		t.Errorf("missing local file: %+v %v", v, err)
	}
	if _, err := (fileSource{root: root}).working("../escape"); err == nil {
		t.Error("path outside the project accepted")
	}
	hs.be.contents["headsha:bin"] = "a\x00"
	if _, err := loadVersions(ctx, src, pr, github.PRFile{Path: "bin", Status: statusAdded}, diffSpec{mode: modePR}, ""); !errors.Is(err, ErrBinaryFile) {
		t.Errorf("binary: %v", err)
	}
}

func TestStripANSI(t *testing.T) {
	if got := stripANSI("\x1b[31mred\x1b[0m plain \x1b[1;32mgreen\x1b[0m"); got != "red plain green" {
		t.Errorf("%q", got)
	}
}

func TestReasonLabel(t *testing.T) {
	if reasonLabel("review_requested") != "review requested" || reasonLabel("unknown_reason") != "unknown_reason" {
		t.Error("reason label")
	}
}

func TestDecodeNotifications(t *testing.T) {
	data := []byte(`[
	 {"id":"1","unread":true,"reason":"mention","updated_at":"2026-11-26T10:00:00Z","subject":{"title":"A","url":"https://api.github.com/repos/a/b/pulls/7","type":"PullRequest"}},
	 {"id":"2","unread":false,"subject":{"title":"Issue","url":"https://api.github.com/repos/a/b/issues/8","type":"Issue"}},
	 {"id":"3","subject":{"title":"Bad","url":"https://api.github.com/repos/a/b/pulls/x","type":"PullRequest"}}
	]`)
	items, err := decodeNotifications(data)
	if err != nil || len(items) != 1 || items[0].Number != 7 || !items[0].Unread || items[0].Reason != "mention" {
		t.Errorf("%+v %v", items, err)
	}
	if _, err := decodeNotifications([]byte("{")); err == nil {
		t.Error("bad json accepted")
	}
}

func TestErrorText(t *testing.T) {
	reset := time.Date(2026, 1, 1, 15, 4, 0, 0, time.Local)
	cases := map[string]error{
		"GitHub rate limit reached. Try again after 15:04.": &github.RateLimitError{Reset: reset},
		"GitHub rate limit reached. Try again later.":       &github.RateLimitError{},
		"GitHub answered 404: Not Found":                    &github.APIError{Status: 404, Message: "Not Found"},
		"plain":                                             errors.New("github: plain"),
	}
	for want, err := range cases {
		if got := errorText(err); got != want {
			t.Errorf("%q, want %q", got, want)
		}
	}
}
