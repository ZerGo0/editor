package issueui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

const (
	testRepoURL = "https://github.com/acme/widgets.git"
	testUser    = "me"
	testRoot    = "/work/widgets"
)

var testNow = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

type fakeBackend struct {
	ghtest.Calls
	mu         sync.Mutex
	searches   []string
	viewer     github.User
	pages      map[string][]github.IssuePage
	issues     map[int]github.Issue
	comments   map[int][]github.Comment
	users      []github.User
	labels     []github.Label
	milestones []github.Milestone
	errs       map[string]error
	created    github.IssueInput
	edits      []github.IssueEdit
	commented  string
	linked     []string
	gate       map[string]chan struct{}
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		viewer:   github.User{Login: testUser},
		pages:    map[string][]github.IssuePage{},
		issues:   map[int]github.Issue{},
		comments: map[int][]github.Comment{},
		errs:     map[string]error{},
		gate:     map[string]chan struct{}{},
	}
}

func (f *fakeBackend) record(name string) error {
	f.Add(name)
	f.mu.Lock()
	gate := f.gate[name]
	err := f.errs[name]
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	return err
}

func (f *fakeBackend) with(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func (f *fakeBackend) Viewer(context.Context) (github.User, error) {
	return f.viewer, f.record("Viewer")
}

func (f *fakeBackend) SearchIssues(_ context.Context, query string, page int) (github.IssuePage, error) {
	f.mu.Lock()
	f.searches = append(f.searches, query)
	pages := f.pages[query]
	f.mu.Unlock()
	if err := f.record("Search"); err != nil {
		return github.IssuePage{}, err
	}
	if page < 1 || page > len(pages) {
		return github.IssuePage{}, nil
	}
	return pages[page-1], nil
}

func (f *fakeBackend) GetIssue(_ context.Context, _ github.Remote, number int) (github.Issue, error) {
	if err := f.record("GetIssue"); err != nil {
		return github.Issue{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	issue, ok := f.issues[number]
	if !ok {
		return github.Issue{}, fmt.Errorf("issue %d not found", number)
	}
	return issue, nil
}

func (f *fakeBackend) IssueComments(_ context.Context, _ github.Remote, number int) ([]github.Comment, error) {
	if err := f.record("IssueComments"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.comments[number], nil
}

func (f *fakeBackend) AddIssueComment(_ context.Context, _ github.Remote, _ int, body string) (github.Comment, error) {
	f.mu.Lock()
	f.commented = body
	f.mu.Unlock()
	return github.Comment{ID: "c-new", DatabaseID: 99, Author: testUser, Body: body, Created: testNow}, f.record("AddIssueComment")
}

func (f *fakeBackend) CreateIssue(_ context.Context, _ github.Remote, in github.IssueInput) (github.Issue, error) {
	f.mu.Lock()
	f.created = in
	f.mu.Unlock()
	issue := testIssue(77, in.Title)
	issue.Body = in.Body
	return issue, f.record("CreateIssue")
}

func (f *fakeBackend) EditIssue(_ context.Context, _ github.Remote, number int, edit github.IssueEdit) (github.Issue, error) {
	if err := f.record("EditIssue"); err != nil {
		return github.Issue{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.edits = append(f.edits, edit)
	issue := f.issues[number]
	if edit.Title != nil {
		issue.Title = *edit.Title
	}
	if edit.Body != nil {
		issue.Body = *edit.Body
	}
	if edit.State != nil {
		issue.State = *edit.State
	}
	if edit.Assignees != nil {
		issue.Assignees = *edit.Assignees
	}
	f.issues[number] = issue
	return issue, nil
}

func (f *fakeBackend) AssignableUsers(_ context.Context, _ github.Remote, after string) (github.UserPage, error) {
	if err := f.record("AssignableUsers"); err != nil {
		return github.UserPage{}, err
	}
	return github.UserPage{Users: f.users}, nil
}

func (f *fakeBackend) MentionableUsers(_ context.Context, _ github.Remote, after string) (github.UserPage, error) {
	if err := f.record("MentionableUsers"); err != nil {
		return github.UserPage{}, err
	}
	return github.UserPage{Users: f.users}, nil
}

func (f *fakeBackend) Milestones(context.Context, github.Remote) ([]github.Milestone, error) {
	return f.milestones, f.record("Milestones")
}

func (f *fakeBackend) Labels(context.Context, github.Remote) ([]github.Label, error) {
	return f.labels, f.record("Labels")
}

func (f *fakeBackend) CreateLinkedBranch(_ context.Context, _ github.Remote, number int, name, base string) (github.LinkedBranch, error) {
	f.mu.Lock()
	f.linked = append(f.linked, fmt.Sprintf("%d:%s:%s", number, name, base))
	f.mu.Unlock()
	return github.LinkedBranch{ID: "lb", Name: name}, f.record("CreateLinkedBranch")
}

func testIssue(number int, title string) github.Issue {
	return github.Issue{
		Number:  number,
		Title:   title,
		Body:    "Body of " + title,
		State:   github.StateOpen,
		Author:  "alice",
		URL:     fmt.Sprintf("https://github.com/acme/widgets/issues/%d", number),
		Created: testNow.Add(-48 * time.Hour),
		Updated: testNow.Add(-3 * time.Hour),
		Repo:    github.Remote{Host: "github.com", Owner: "acme", Name: "widgets"},
	}
}

type fakeLocal struct {
	mu       sync.Mutex
	root     string
	remotes  []git.Remote
	status   git.Status
	branches []git.Branch
	outgoing []git.Commit
	calls    []string
	errs     map[string]error
}

func newFakeLocal() *fakeLocal {
	return &fakeLocal{
		root:    testRoot,
		remotes: []git.Remote{{Name: "origin", FetchURL: testRepoURL}},
		status:  git.Status{Branch: "main", Upstream: "origin/main", OID: "abc123"},
		branches: []git.Branch{
			{Name: "main", Current: true},
			{Name: "origin/main", Remote: true},
		},
		errs: map[string]error{},
	}
}

func (l *fakeLocal) record(call string) error {
	l.calls = append(l.calls, call)
	return l.errs[strings.SplitN(call, ":", 2)[0]]
}

func (l *fakeLocal) called() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.calls...)
}

func (l *fakeLocal) setBranch(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status.Branch = name
}

func (l *fakeLocal) WorkDir() string { return l.root }

func (l *fakeLocal) Remotes(context.Context) ([]git.Remote, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.remotes, l.record("Remotes")
}

func (l *fakeLocal) Status(context.Context) (*git.Status, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st := l.status
	return &st, l.record("Status")
}

func (l *fakeLocal) Branches(context.Context) ([]git.Branch, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.branches, l.record("Branches")
}

func (l *fakeLocal) OutgoingCommits(context.Context) ([]git.Commit, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.outgoing, l.record("OutgoingCommits")
}

func (l *fakeLocal) CreateBranch(_ context.Context, name, start string, checkout bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.record(fmt.Sprintf("CreateBranch:%s:%s:%v", name, start, checkout)); err != nil {
		return err
	}
	l.branches = append(l.branches, git.Branch{Name: name})
	if checkout {
		l.status.Branch = name
	}
	return nil
}

func (l *fakeLocal) Checkout(_ context.Context, ref string, _ bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.record("Checkout:" + ref); err != nil {
		return err
	}
	l.status.Branch = ref
	return nil
}

func (l *fakeLocal) CheckoutNewTracking(_ context.Context, remoteBranch string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.record("CheckoutNewTracking:" + remoteBranch); err != nil {
		return err
	}
	_, name, _ := strings.Cut(remoteBranch, "/")
	l.status.Branch = name
	return nil
}

func (l *fakeLocal) Fetch(_ context.Context, remote string, _ bool, _ func(string)) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.record("Fetch:" + remote)
}

type harness struct {
	t      *testing.T
	h      *ghtest.FakeHost
	be     *fakeBackend
	local  *fakeLocal
	env    Env
	token  error
	open   error
	copied []string
	urls   []string
	i      *Issues
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	hs := &harness{t: t, h: ghtest.NewFakeHost(testRoot, t.TempDir()), be: newFakeBackend(), local: newFakeLocal()}
	hs.env = Env{
		Token:      func(context.Context, string) (string, error) { return "tok", hs.token },
		NewBackend: func(string, string) Backend { return hs.be },
		OpenLocal:  ghtest.OpenLocal(&hs.open, func() Local { return hs.local }),
		OpenURL:    func(u string) error { hs.urls = append(hs.urls, u); return nil },
		Copy:       func(text string) { hs.copied = append(hs.copied, text) },
		Now:        func() time.Time { return testNow },
	}
	return hs
}

func (hs *harness) start(t *testing.T) {
	t.Helper()
	hs.i = NewWith(hs.h, hs.env)
	ghtest.Start(t, hs.h, &hs.i.Session)
}

func (hs *harness) ready(t *testing.T) {
	t.Helper()
	hs.start(t)
	if hs.i.Phase != ghshared.PhaseReady {
		t.Fatalf("phase %d, error %v", hs.i.Phase, hs.i.PhaseErr)
	}
}

var errBoom = errors.New("boom")
