package issueui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestActionsAreRegistered(t *testing.T) {
	want := []string{
		"issues.refresh", "issues.create", "issues.createFromSelection", "issues.startWorking", "issues.stopWorking",
		"issues.continueWorking", "issues.copyPermalink", "issues.openPermalink",
	}
	var got []string
	for _, a := range Actions {
		got = append(got, a.ID)
		if a.Title == "" || a.Category == "" {
			t.Errorf("action %+v lacks a title or category", a)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("actions %v", got)
	}
	hs := newHarness(t)
	hs.ready(t)
	if hs.i.Run("issues.unknown") || hs.i.Run("pr.refresh") {
		t.Error("an unknown action was handled")
	}
	for _, id := range want {
		if id == ActionStartWorking || id == ActionCopyPermalink || id == ActionOpenPermalink || id == ActionCreateFromSelection {
			continue
		}
		if !hs.i.Run(id) {
			t.Errorf("action %q not handled", id)
		}
	}
}

func TestActionsNeedAConnection(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNoGH
	hs.start(t)
	for _, id := range []string{ActionCreate, ActionStartWorking, ActionContinueWorking} {
		hs.i.Run(id)
		if got := hs.h.lastNotice(); got != "GitHub CLI not found" {
			t.Errorf("%s: notice %q", id, got)
		}
	}
	if len(hs.h.asks) != 0 || len(hs.h.opened) != 0 {
		t.Errorf("asks %v opened %v", hs.h.asks, hs.h.opened)
	}
}

func TestParseIssueNumber(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"42", 42, true},
		{" #42 ", 42, true},
		{"https://github.com/acme/widgets/issues/42", 42, true},
		{"https://github.com/acme/widgets/issues/42/", 42, true},
		{"https://github.com/acme/widgets/pull/42", 0, false},
		{"", 0, false},
		{"0", 0, false},
		{"-3", 0, false},
		{"abc", 0, false},
	}
	for _, tc := range tests {
		got, err := parseIssueNumber(tc.in)
		if got != tc.want || (err == nil) != tc.ok {
			t.Errorf("parseIssueNumber(%q) = %d, %v", tc.in, got, err)
		}
	}
}

func TestPickRemote(t *testing.T) {
	remotes := []git.Remote{
		{Name: "local", FetchURL: "/srv/repo.git"},
		{Name: "fork", FetchURL: "git@github.com:me/widgets.git"},
		{Name: "upstream", FetchURL: "git@github.com:acme/widgets.git"},
	}
	repo, byName, ok := pickRemote(remotes)
	if !ok || repo.Owner != "acme" || len(byName) != 2 {
		t.Errorf("%+v %v %v", repo, byName, ok)
	}
	if got := remoteNameFor(byName, repo); got != "upstream" {
		t.Errorf("remote name %q", got)
	}
	if repo, _, ok := pickRemote(remotes[1:2]); !ok || repo.Owner != "me" {
		t.Errorf("only fork: %+v %v", repo, ok)
	}
	if _, _, ok := pickRemote(remotes[:1]); ok {
		t.Error("a local path is not a GitHub remote")
	}
}

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{3 * 24 * time.Hour, "3d ago"},
		{15 * 24 * time.Hour, "2w ago"},
		{800 * 24 * time.Hour, "2y ago"},
	}
	for _, tc := range tests {
		if got := relativeTime(testNow, testNow.Add(-tc.ago)); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.ago, got, tc.want)
		}
	}
	if relativeTime(testNow, time.Time{}) != "" {
		t.Error("a zero time must give no text")
	}
}

func TestLabelColors(t *testing.T) {
	fallbackBG, fallbackFG := ui.RGBA(1, 2, 3, 255), ui.RGBA(4, 5, 6, 255)
	bg, fg := labelColors("zzzzzz", fallbackBG, fallbackFG)
	if bg != fallbackBG || fg != fallbackFG {
		t.Error("invalid hex must use the fallback")
	}
	if bg, fg := labelColors("", fallbackBG, fallbackFG); bg != fallbackBG || fg != fallbackFG {
		t.Error("empty hex must use the fallback")
	}
	if _, fg := labelColors("ffffff", fallbackBG, fallbackFG); fg != ui.RGBA(0, 0, 0, 255) {
		t.Errorf("light label text %v", fg)
	}
	if bg, fg := labelColors("000000", fallbackBG, fallbackFG); fg != ui.RGBA(255, 255, 255, 255) || bg != ui.RGBA(0, 0, 0, 255) {
		t.Errorf("dark label %v %v", bg, fg)
	}
}

func TestStoreRoundTripAndValidation(t *testing.T) {
	dir := t.TempDir()
	repo := github.Remote{Host: "github.com", Owner: "acme", Name: "widgets"}
	other := github.Remote{Host: "github.com", Owner: "acme", Name: "other"}
	if state, err := loadWork(dir, repo); err != nil || state.Active != nil {
		t.Fatalf("missing file: %+v %v", state, err)
	}
	var state workState
	for n := 1; n <= maxRecentWork+3; n++ {
		state.activate(workItem{Number: n, Title: "t", Branch: "b"})
	}
	state.activate(workItem{Number: 5, Title: "again", Branch: "b5"})
	if len(state.Recent) != maxRecentWork || state.Recent[0].Number != 5 || state.Recent[0].Title != "again" {
		t.Fatalf("recent %+v", state.Recent)
	}
	if err := saveWork(dir, repo, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadWork(dir, repo)
	if err != nil || loaded.Active == nil || loaded.Active.Number != 5 || len(loaded.Recent) != maxRecentWork {
		t.Fatalf("loaded %+v %v", loaded, err)
	}
	if again, _ := loadWork(dir, other); again.Active != nil {
		t.Error("state leaked to another repository")
	}
	if err := saveWork("", repo, state); err != nil {
		t.Errorf("saving without a directory must be a no-op: %v", err)
	}
	if err := os.WriteFile(storePath(dir, repo), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadWork(dir, repo); err == nil {
		t.Error("broken state must be an error")
	}
	if err := os.WriteFile(storePath(dir, repo), []byte(`{"active":{"number":0,"branch":"x"},"recent":[{"number":3,"branch":""},{"number":4,"branch":"ok"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadWork(dir, repo)
	if err != nil || loaded.Active != nil || len(loaded.Recent) != 1 || loaded.Recent[0].Number != 4 {
		t.Errorf("invalid entries must be dropped: %+v %v", loaded, err)
	}
	if filepath.Dir(storePath(dir, repo)) != filepath.Join(dir, "issues") {
		t.Errorf("store path %q", storePath(dir, repo))
	}
}

func TestDeactivateKeepsHistory(t *testing.T) {
	var s workState
	s.activate(workItem{Number: 1, Branch: "a"})
	s.activate(workItem{Number: 2, Branch: "b"})
	s.deactivate()
	if s.Active != nil || len(s.Recent) != 2 || s.Recent[0].Number != 2 {
		t.Errorf("state %+v", s)
	}
	if _, ok := s.previous(1); !ok {
		t.Error("issue 1 must be in the history")
	}
	if _, ok := s.previous(3); ok {
		t.Error("issue 3 was never worked on")
	}
	s.deactivate()
}

func TestIssueCompletions(t *testing.T) {
	issues := []github.Issue{
		{Number: 12, Title: "Fix the crash", State: github.StateOpen, Body: "long body", Updated: testNow.Add(-time.Hour)},
		{Number: 120, Title: "Other", State: github.StateClosed, Updated: testNow},
		{Number: 7, Title: "Crash at 12 o'clock", State: github.StateOpen, Updated: testNow},
		{Number: 12, Title: "Duplicate of twelve", State: github.StateOpen},
	}
	labels := func(items []editor.Completion) []string {
		var out []string
		for _, c := range items {
			out = append(out, c.Label)
		}
		return out
	}
	got := issueCompletions(issues, "12")
	want := []string{"#120 Other", "#12 Fix the crash", "#7 Crash at 12 o'clock"}
	if !slices.Equal(labels(got), want) {
		t.Errorf("by number: %v", labels(got))
	}
	if got[1].Insert != "#12" || got[1].Detail != "Open" || got[1].Documentation != "long body" {
		t.Errorf("completion %+v", got[1])
	}
	if got := issueCompletions(issues, "crash"); !slices.Equal(labels(got), []string{"#7 Crash at 12 o'clock", "#12 Fix the crash"}) {
		t.Errorf("by title: %v", labels(got))
	}
	if got := issueCompletions(issues, ""); len(got) != 3 {
		t.Errorf("empty needle: %v", labels(got))
	}
	if got := issueCompletions(issues, "zzz"); len(got) != 0 {
		t.Errorf("no match: %v", labels(got))
	}
	many := make([]github.Issue, maxCompletions+10)
	for n := range many {
		many[n] = github.Issue{Number: n + 1, Title: "t"}
	}
	if got := issueCompletions(many, ""); len(got) != maxCompletions {
		t.Errorf("cap: %d", len(got))
	}
}

func TestUserCompletions(t *testing.T) {
	users := []github.User{{Login: "alice", Name: "Alice Smith"}, {Login: "bob", Name: "Bob Alison"}, {Login: "carol"}}
	got := userCompletions(users, "ali")
	if len(got) != 2 || got[0].Insert != "@alice" || got[1].Insert != "@bob" || got[0].Detail != "Alice Smith" {
		t.Errorf("%+v", got)
	}
	if got := userCompletions(users, ""); len(got) != 3 {
		t.Errorf("empty needle: %d", len(got))
	}
	if got := userCompletions(users, "zz"); len(got) != 0 {
		t.Errorf("no match: %d", len(got))
	}
}

func TestCompletionsLoadInBackground(t *testing.T) {
	hs := newHarness(t)
	hs.be.users = []github.User{{Login: "alice", Name: "Alice"}}
	hs.be.pages["repo:acme/widgets sort:updated-desc"] = []github.IssuePage{{Issues: []github.Issue{testIssue(12, "Fix the crash")}}}
	hs.ready(t)
	if got := hs.i.Completions("#1"); len(got) != 0 {
		t.Errorf("completions before the data arrived: %v", got)
	}
	hs.h.pump(t, func() bool { return hs.i.mention.usersLoaded && hs.i.mention.issuesLoaded })
	if got := hs.i.Completions("#1"); len(got) != 1 || got[0].Insert != "#12" {
		t.Errorf("issues %+v", got)
	}
	if got := hs.i.Completions("@al"); len(got) != 1 || got[0].Insert != "@alice" {
		t.Errorf("users %+v", got)
	}
	if hs.i.Completions("") != nil || hs.i.Completions("x") != nil {
		t.Error("only # and @ trigger completions")
	}
	if hs.be.called("MentionableUsers") != 1 {
		t.Errorf("mentionable users loaded %d times", hs.be.called("MentionableUsers"))
	}
	hs.i.Close()
	if hs.i.Completions("#1") != nil {
		t.Error("completions after close")
	}
}

func TestCompletionsIncludeLoadedIssues(t *testing.T) {
	hs := newHarness(t)
	hs.seedSide()
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	got := issueCompletions(hs.i.allKnownIssues(), "")
	if len(got) < 3 {
		t.Errorf("known issues %d", len(got))
	}
}

func TestSettingsJSONNames(t *testing.T) {
	var q []Query
	if err := json.Unmarshal([]byte(`[{"label":"A","query":"is:open","groupBy":["milestone"]}]`), &q); err != nil || len(q) != 1 || q[0].Label != "A" || !q[0].groupsByMilestone() {
		t.Errorf("%+v %v", q, err)
	}
}
