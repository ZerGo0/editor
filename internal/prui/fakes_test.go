package prui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testTimeout = 5 * time.Second
	testRepoURL = "https://github.com/acme/widgets.git"
)

var testNow = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

type askCall struct {
	title, action, value string
	submit               func(string) error
}

type fakeHost struct {
	root     string
	userData string
	updates  chan func()
	views    map[string]func(*ui.Context)
	titles   map[string]string
	opened   []string
	closed   []string
	notices  []string
	errs     []string
	asks     []askCall
}

func newFakeHost() *fakeHost {
	return &fakeHost{root: "/work/widgets", updates: make(chan func(), 1024), views: map[string]func(*ui.Context){}, titles: map[string]string{}}
}

func (h *fakeHost) RootDir() string                          { return h.root }
func (h *fakeHost) Window() *mygo.Window                     { return nil }
func (h *fakeHost) Update(fn func())                         { h.updates <- fn }
func (h *fakeHost) OpenAt(string, int, int)                  {}
func (h *fakeHost) CloseView(id string)                      { h.closed = append(h.closed, id) }
func (h *fakeHost) Notify(text string)                       { h.notices = append(h.notices, text) }
func (h *fakeHost) SaveAllFiles() bool                       { return true }
func (h *fakeHost) Buffer(string) *editor.Buffer             { return nil }
func (h *fakeHost) ActivePath() string                       { return "" }
func (h *fakeHost) ActiveEditor() *editor.State              { return nil }
func (h *fakeHost) UserDataDir() string                      { return h.userData }
func (h *fakeHost) Language(string, string) *editor.Language { return nil }
func (h *fakeHost) ConfigureEditor(*editor.State, string)    {}
func (h *fakeHost) ConfigureDiff(*editor.DiffState, string)  {}

func (h *fakeHost) OpenView(id, title string, view func(*ui.Context)) {
	h.opened = append(h.opened, id)
	h.views[id] = view
	h.titles[id] = title
}

func (h *fakeHost) ShowError(title string, err error) {
	h.errs = append(h.errs, title+": "+err.Error())
}

func (h *fakeHost) Ask(title, action, value string, submit func(string) error) {
	h.asks = append(h.asks, askCall{title, action, value, submit})
}

func (h *fakeHost) pump(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.After(testTimeout)
	for !done() {
		select {
		case fn := <-h.updates:
			fn()
		case <-deadline:
			t.Fatal("timed out waiting for the background work")
		}
	}
}

func (h *fakeHost) drain() {
	for {
		select {
		case fn := <-h.updates:
			fn()
		case <-time.After(20 * time.Millisecond):
			return
		}
	}
}

type fakeBackend struct {
	mu         sync.Mutex
	calls      []string
	lists      map[string][]github.PullRequest
	listPages  map[int][]github.PullRequest
	current    *github.PullRequest
	pr         github.PullRequest
	files      []github.PRFile
	checks     []github.Check
	comments   []github.Comment
	reviews    []github.Review
	threads    []github.Thread
	contents   map[string]string
	errs       map[string]error
	created    github.CreateOptions
	merged     string
	submitted  string
	body       string
	lineOpts   github.ReviewCommentOptions
	replied    string
	timeline   []TimelineEvent
	ov         Overview
	ovErr      error
	viewed     map[string]bool
	commits    []github.Commit
	compare    Comparison
	notifs     NotificationPage
	localPRs   []github.PullRequest
	logText    string
	labels     []github.Label
	milestones []github.Milestone
	users      []github.User
	viewer     string
	edit       github.IssueEdit
	arg        string
}

func (f *fakeBackend) record(name string, args ...any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
	return f.errs[name]
}

func (f *fakeBackend) called(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == name {
			n++
		}
	}
	return n
}

func listKey(o github.ListOptions) string {
	switch {
	case o.Query != "":
		return "search:" + o.Query
	case o.ReviewRequested != "":
		return "review"
	case o.Assignee != "":
		return "assigned"
	case o.Author != "":
		return "created"
	}
	return "all"
}

func (f *fakeBackend) List(_ context.Context, _ github.Remote, o github.ListOptions) ([]github.PullRequest, error) {
	if err := f.record("List:" + listKey(o)); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if o.Page > 1 {
		return f.listPages[o.Page], nil
	}
	return f.lists[listKey(o)], nil
}

func (f *fakeBackend) Get(context.Context, github.Remote, int) (github.PullRequest, error) {
	if err := f.record("Get"); err != nil {
		return github.PullRequest{}, err
	}
	return f.pr, nil
}

func (f *fakeBackend) CurrentBranchPR(_ context.Context, _ github.Remote, branch, owner string) (*github.PullRequest, error) {
	if err := f.record("Current:" + owner + ":" + branch); err != nil {
		return nil, err
	}
	return f.current, nil
}

func (f *fakeBackend) Files(context.Context, github.Remote, int) ([]github.PRFile, error) {
	if err := f.record("Files"); err != nil {
		return nil, err
	}
	return f.files, nil
}

func (f *fakeBackend) Checks(context.Context, github.Remote, string) ([]github.Check, error) {
	if err := f.record("Checks"); err != nil {
		return nil, err
	}
	return f.checks, nil
}

func (f *fakeBackend) Comments(context.Context, github.Remote, int) ([]github.Comment, error) {
	if err := f.record("Comments"); err != nil {
		return nil, err
	}
	return f.comments, nil
}

func (f *fakeBackend) Reviews(context.Context, github.Remote, int) ([]github.Review, error) {
	if err := f.record("Reviews"); err != nil {
		return nil, err
	}
	return f.reviews, nil
}

func (f *fakeBackend) ReviewThreads(context.Context, github.Remote, int) ([]github.Thread, error) {
	if err := f.record("Threads"); err != nil {
		return nil, err
	}
	return f.threads, nil
}

func (f *fakeBackend) AddComment(_ context.Context, _ github.Remote, _ int, body string) (github.Comment, error) {
	f.mu.Lock()
	f.body = body
	f.mu.Unlock()
	return github.Comment{ID: "c-new", DatabaseID: 99, Author: "me", Body: body, Created: testNow}, f.record("AddComment")
}

func (f *fakeBackend) Reply(_ context.Context, threadID, body string) (github.Comment, error) {
	f.mu.Lock()
	f.replied = threadID + ":" + body
	f.mu.Unlock()
	return github.Comment{ID: "c-reply", Author: "me", Body: body, Created: testNow}, f.record("Reply")
}

func (f *fakeBackend) StartReview(context.Context, github.Remote, int) (string, error) {
	return "review-1", f.record("StartReview")
}

func (f *fakeBackend) AddReviewComment(_ context.Context, reviewID string, o github.ReviewCommentOptions) (github.Thread, error) {
	f.mu.Lock()
	f.lineOpts = o
	f.mu.Unlock()
	th := github.Thread{ID: "t-new", Path: o.Path, Line: o.Line, Side: o.Side, Comments: []github.Comment{{ID: "c-t", Author: "me", Body: o.Body, Created: testNow}}}
	return th, f.record("AddReviewComment:" + reviewID)
}

func (f *fakeBackend) SubmitReview(_ context.Context, reviewID, event, body string) error {
	f.mu.Lock()
	f.submitted = reviewID + ":" + event + ":" + body
	f.mu.Unlock()
	return f.record("SubmitReview")
}

func (f *fakeBackend) ResolveThread(context.Context, string) error   { return f.record("Resolve") }
func (f *fakeBackend) UnresolveThread(context.Context, string) error { return f.record("Unresolve") }

func (f *fakeBackend) Merge(_ context.Context, _ github.Remote, n int, method, title, message string) error {
	f.mu.Lock()
	f.merged = fmt.Sprintf("%d:%s", n, method)
	f.arg = title + "|" + message
	f.mu.Unlock()
	return f.record("Merge")
}

func (f *fakeBackend) Create(_ context.Context, _ github.Remote, o github.CreateOptions) (github.PullRequest, error) {
	f.mu.Lock()
	f.created = o
	f.mu.Unlock()
	pr := testPR(77, o.Title)
	pr.HeadRef, pr.BaseRef = o.Head, o.Base
	return pr, f.record("Create")
}

func (f *fakeBackend) MarkReady(context.Context, github.Remote, int) error {
	return f.record("MarkReady")
}

func (f *fakeBackend) MergeBase(context.Context, github.Remote, string, string) (string, error) {
	return "basesha", f.record("MergeBase")
}

func (f *fakeBackend) FileAt(_ context.Context, _ github.Remote, ref, path string) ([]byte, error) {
	if err := f.record("FileAt"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	text, ok := f.contents[ref+":"+path]
	if !ok {
		return nil, errors.New("missing " + ref + ":" + path)
	}
	return []byte(text), nil
}

type fakeLocal struct {
	mu        sync.Mutex
	remotes   []git.Remote
	branches  []git.Branch
	files     map[string]string
	pushes    []git.PushOptions
	checkouts []string
	ahead     []git.Commit
	err       error
}

func (l *fakeLocal) Remotes(context.Context) ([]git.Remote, error) { return l.remotes, l.err }

func (l *fakeLocal) Branches(context.Context) ([]git.Branch, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.branches), l.err
}

func (l *fakeLocal) Show(_ context.Context, rev, path string) ([]byte, error) {
	text, ok := l.files[rev+":"+path]
	if !ok {
		return nil, git.ErrNotFound
	}
	return []byte(text), nil
}

func (l *fakeLocal) Push(_ context.Context, o git.PushOptions) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pushes = append(l.pushes, o)
	return nil
}

func (l *fakeLocal) setCurrent(name, upstream string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.branches = []git.Branch{
		{Name: name, Current: true, Upstream: upstream, Subject: "Subject of " + name},
		{Name: "origin/main", Remote: true},
		{Name: "origin/develop", Remote: true},
		{Name: "origin/" + name, Remote: true},
	}
}

func testPR(n int, title string) github.PullRequest {
	return github.PullRequest{
		NodeID: fmt.Sprintf("PR_%d", n), Number: n, Title: title, State: github.StateOpen, URL: fmt.Sprintf("https://github.com/acme/widgets/pull/%d", n),
		Author: "alice", HeadRef: "feature", HeadSHA: "headsha", BaseRef: "main", HeadOwner: "acme", HeadRepo: "widgets",
		Updated: testNow.Add(-3 * time.Hour), Created: testNow.Add(-48 * time.Hour), Body: "Fixes the **widget**.",
		Labels: []string{"bug"}, Reviewers: []string{"bob"}, Additions: 5, Deletions: 2, ChangedFiles: 1,
	}
}

type harness struct {
	p      *PRs
	h      *fakeHost
	be     *fakeBackend
	local  *fakeLocal
	env    Env
	mu     sync.Mutex
	co     []int
	urls   []string
	copied []string
	ok     bool
	token  error
	open   error
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	hs := &harness{h: newFakeHost(), ok: true}
	hs.local = &fakeLocal{remotes: []git.Remote{{Name: "origin", FetchURL: testRepoURL, PushURL: testRepoURL}}, files: map[string]string{}}
	hs.local.setCurrent("feature", "origin/feature")
	cur := testPR(12, "Fix the widget")
	hs.be = &fakeBackend{
		lists: map[string][]github.PullRequest{
			"review": {testPR(31, "Review me please")},
			"all":    {testPR(40, "Everything open")},
		},
		current:  &cur,
		pr:       cur,
		files:    []github.PRFile{{Path: "src/widget.go", Status: statusModified, Additions: 5, Deletions: 2}},
		checks:   []github.Check{{Name: "build", Status: statusCompleted, Conclusion: "success", URL: "https://ci.example/build"}},
		comments: []github.Comment{{ID: "c1", DatabaseID: 1, Author: "carol", Body: "Looks good to me", Created: testNow.Add(-time.Hour)}},
		contents: map[string]string{},
		errs:     map[string]error{},
		viewer:   "me",
		ov:       Overview{ID: "PR_12", Viewer: "me", AllowMerge: true, AllowSquash: true, AllowRebase: true, CanUpdate: true, MergeState: mergeStateClean, DefaultMethod: github.MergeMethodMerge},
	}
	hs.env = Env{
		Token:      func(context.Context, string) (string, error) { return "tok", hs.token },
		NewBackend: func(string, string) Backend { return hs.be },
		OpenLocal: func(context.Context, string) (Local, error) {
			if hs.open != nil {
				return nil, hs.open
			}
			return hs.local, nil
		},
		Checkout: func(_ context.Context, _ string, n int) error {
			hs.mu.Lock()
			defer hs.mu.Unlock()
			hs.co = append(hs.co, n)
			return nil
		},
		Confirm:  func(*mygo.Window, ConfirmOptions) bool { hs.mu.Lock(); defer hs.mu.Unlock(); return hs.ok },
		OpenURL:  func(u string) error { hs.mu.Lock(); defer hs.mu.Unlock(); hs.urls = append(hs.urls, u); return nil },
		CopyText: func(s string) { hs.mu.Lock(); defer hs.mu.Unlock(); hs.copied = append(hs.copied, s) },
		Now:      func() time.Time { return testNow },
	}
	return hs
}

func (hs *harness) start(t *testing.T) {
	t.Helper()
	hs.p = NewWith(hs.h, hs.env)
	t.Cleanup(hs.p.Close)
	hs.h.pump(t, func() bool { return hs.p.phase != phaseLoading })
}

func (hs *harness) ready(t *testing.T) {
	t.Helper()
	hs.start(t)
	if hs.p.phase != phaseReady {
		t.Fatalf("phase %d, want ready", hs.p.phase)
	}
	hs.h.pump(t, func() bool { return hs.p.current.loaded })
}

func (f *fakeBackend) note(name, arg string) error {
	f.mu.Lock()
	f.arg = arg
	f.mu.Unlock()
	return f.record(name)
}

func (f *fakeBackend) Commits(context.Context, github.Remote, int) ([]github.Commit, error) {
	if err := f.record("Commits"); err != nil {
		return nil, err
	}
	return f.commits, nil
}

func (f *fakeBackend) RequestReviewers(_ context.Context, _ github.Remote, _ int, users, _ []string) error {
	return f.note("RequestReviewers", fmt.Sprint(users))
}

func (f *fakeBackend) EditIssue(_ context.Context, _ github.Remote, _ int, e github.IssueEdit) (github.Issue, error) {
	f.mu.Lock()
	f.edit = e
	f.mu.Unlock()
	return github.Issue{}, f.record("EditIssue")
}

func (f *fakeBackend) Viewer(context.Context) (github.User, error) {
	if err := f.record("Viewer"); err != nil {
		return github.User{}, err
	}
	return github.User{Login: f.viewer}, nil
}

func (f *fakeBackend) Labels(context.Context, github.Remote) ([]github.Label, error) {
	return f.labels, f.record("Labels")
}

func (f *fakeBackend) Milestones(context.Context, github.Remote) ([]github.Milestone, error) {
	return f.milestones, f.record("Milestones")
}

func (f *fakeBackend) AssignableUsers(context.Context, github.Remote, string) (github.UserPage, error) {
	return github.UserPage{Users: f.users}, f.record("AssignableUsers")
}

func (f *fakeBackend) Timeline(context.Context, github.Remote, int) ([]TimelineEvent, error) {
	if err := f.record("Timeline"); err != nil {
		return nil, err
	}
	if f.timeline != nil {
		return f.timeline, nil
	}
	var events []TimelineEvent
	for _, c := range f.comments {
		events = append(events, TimelineEvent{Kind: "IssueComment", ID: c.ID, DatabaseID: c.DatabaseID, Actor: c.Author, Time: c.Created, Body: c.Body, URL: c.URL})
	}
	return events, nil
}

func (f *fakeBackend) Overview(context.Context, github.Remote, int, string) (Overview, error) {
	if err := f.record("Overview"); err != nil {
		return Overview{}, err
	}
	return f.ov, f.ovErr
}

func (f *fakeBackend) ViewedFiles(context.Context, github.Remote, int) (map[string]bool, error) {
	if err := f.record("ViewedFiles"); err != nil {
		return nil, err
	}
	return f.viewed, nil
}

func (f *fakeBackend) SetViewed(_ context.Context, _, path string, viewed bool) error {
	return f.note("SetViewed", fmt.Sprintf("%s:%v", path, viewed))
}

func (f *fakeBackend) Compare(_ context.Context, _ github.Remote, base, head string) (Comparison, error) {
	if err := f.note("Compare", base+"..."+head); err != nil {
		return Comparison{}, err
	}
	return f.compare, nil
}

func (f *fakeBackend) UpdateBranch(context.Context, github.Remote, int, string) error {
	return f.record("UpdateBranch")
}
func (f *fakeBackend) ConvertToDraft(context.Context, string) error {
	return f.record("ConvertToDraft")
}
func (f *fakeBackend) SetAutoMerge(_ context.Context, _, method string, enable bool) error {
	return f.note("SetAutoMerge", fmt.Sprintf("%s:%v", method, enable))
}
func (f *fakeBackend) Enqueue(context.Context, string) error { return f.record("Enqueue") }
func (f *fakeBackend) Dequeue(context.Context, string) error { return f.record("Dequeue") }
func (f *fakeBackend) DeleteBranch(_ context.Context, r github.Remote, ref string) error {
	return f.note("DeleteBranch", r.Owner+"/"+r.Name+":"+ref)
}
func (f *fakeBackend) Revert(context.Context, string) (int, error) { return 90, f.record("Revert") }
func (f *fakeBackend) React(_ context.Context, id, content string, add bool) error {
	return f.note("React", fmt.Sprintf("%s:%s:%v", id, content, add))
}
func (f *fakeBackend) EditComment(_ context.Context, kind, id, body string) error {
	return f.note("EditComment", kind+":"+id+":"+body)
}
func (f *fakeBackend) DeleteComment(_ context.Context, kind, id string) error {
	return f.note("DeleteComment", kind+":"+id)
}
func (f *fakeBackend) DeleteReview(context.Context, string) error { return f.record("DeleteReview") }
func (f *fakeBackend) AddFileThread(_ context.Context, review, path, body string) error {
	return f.note("AddFileThread", review+":"+path+":"+body)
}
func (f *fakeBackend) JobLogs(context.Context, github.Remote, int64) (string, error) {
	return f.logText, f.record("JobLogs")
}
func (f *fakeBackend) RunAction(_ context.Context, _ github.Remote, run int64, action string) error {
	return f.note("RunAction", fmt.Sprintf("%d:%s", run, action))
}
func (f *fakeBackend) Notifications(_ context.Context, _ github.Remote, since string) (NotificationPage, error) {
	if err := f.note("Notifications", since); err != nil {
		return NotificationPage{}, err
	}
	return f.notifs, nil
}
func (f *fakeBackend) MarkNotification(_ context.Context, id string, done bool) error {
	return f.note("MarkNotification", fmt.Sprintf("%s:%v", id, done))
}
func (f *fakeBackend) LocalBranchPRs(context.Context, github.Remote, []string) ([]github.PullRequest, error) {
	if err := f.record("LocalBranchPRs"); err != nil {
		return nil, err
	}
	return f.localPRs, nil
}

func (l *fakeLocal) Checkout(_ context.Context, ref string, _ bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checkouts = append(l.checkouts, ref)
	return nil
}

func (l *fakeLocal) CompareBranches(context.Context, string, string) (git.BranchComparison, error) {
	return git.BranchComparison{AOnly: l.ahead}, nil
}

func prPtr(pr github.PullRequest) *github.PullRequest { return &pr }

func (h *fakeHost) waitFor(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for a condition")
		}
		select {
		case fn := <-h.updates:
			fn()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (hs *harness) setOK(v bool) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.ok = v
}

func (f *fakeBackend) clearErr(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.errs, name)
}
