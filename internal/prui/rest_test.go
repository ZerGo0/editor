package prui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/github"
)

type recordedRequest struct {
	method string
	path   string
	query  string
	body   string
	header http.Header
}

type apiServer struct {
	mu       sync.Mutex
	requests []recordedRequest
	handler  func(w http.ResponseWriter, r *http.Request, body string)
}

func newAPIServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body string)) (*restClient, *apiServer) {
	t.Helper()
	api := &apiServer{handler: handler}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		api.mu.Lock()
		api.requests = append(api.requests, recordedRequest{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, string(data), r.Header.Clone()})
		api.mu.Unlock()
		api.handler(w, r, string(data))
	}))
	t.Cleanup(srv.Close)
	client := newRESTClient("github.com", "tok")
	client.base, client.graphqlURL, client.http = srv.URL, srv.URL+"/graphql", srv.Client()
	return client, api
}

func (a *apiServer) last() recordedRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.requests[len(a.requests)-1]
}

func (a *apiServer) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.requests)
}

func gqlReply(w http.ResponseWriter, data string) {
	_, _ = io.WriteString(w, `{"data":`+data+`}`)
}

var apiRepo = github.Remote{Host: "github.com", Owner: "acme", Name: "widgets"}

func TestRESTClientHostMapping(t *testing.T) {
	if c := newRESTClient("github.com", "t"); c.base != "https://api.github.com" || c.graphqlURL != "https://api.github.com/graphql" {
		t.Errorf("%s %s", c.base, c.graphqlURL)
	}
	if c := newRESTClient("ghe.example.com", "t"); c.base != "https://ghe.example.com/api/v3" || c.graphqlURL != "https://ghe.example.com/api/graphql" {
		t.Errorf("%s %s", c.base, c.graphqlURL)
	}
}

func TestGraphQLErrorHandling(t *testing.T) {
	var reply string
	client, _ := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) { _, _ = io.WriteString(w, reply) })
	ctx := t.Context()
	var out map[string]any
	reply = `{"data":{"a":1}}`
	if err := client.gql(ctx, "q", nil, &out); err != nil || out["a"] != float64(1) {
		t.Fatalf("%v %v", out, err)
	}
	reply = `{"errors":[{"message":"bad field","type":"INVALID"}]}`
	var gqlErr *github.GraphQLError
	if err := client.gql(ctx, "q", nil, &out); !errors.As(err, &gqlErr) {
		t.Errorf("%v", err)
	}
	reply = `{"errors":[{"message":"slow down","type":"RATE_LIMITED"}]}`
	if err := client.gql(ctx, "q", nil, &out); !errors.Is(err, github.ErrRateLimited) {
		t.Errorf("%v", err)
	}
	reply = `{"data":null}`
	if err := client.gql(ctx, "q", nil, &out); err == nil {
		t.Error("null data accepted")
	}
	reply = "<html>"
	if err := client.gql(ctx, "q", nil, &out); err == nil {
		t.Error("html accepted")
	}
}

func TestHTTPStatusErrors(t *testing.T) {
	client, _ := newAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		switch r.URL.Path {
		case "/limited":
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1800000000")
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"message":"API rate limit exceeded"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"Not Found"}`)
		}
	})
	var apiErr *github.APIError
	err := client.json(t.Context(), http.MethodGet, "/missing", nil, nil)
	if !errors.As(err, &apiErr) || apiErr.Status != 404 || apiErr.Message != "Not Found" {
		t.Errorf("%v", err)
	}
	var limit *github.RateLimitError
	err = client.json(t.Context(), http.MethodGet, "/limited", nil, nil)
	if !errors.As(err, &limit) || limit.Reset.Unix() != 1800000000 {
		t.Errorf("%v", err)
	}
}

func TestMergeBaseAndFileAt(t *testing.T) {
	client, api := newAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		switch {
		case strings.Contains(r.URL.Path, "/compare/"):
			_, _ = io.WriteString(w, `{"merge_base_commit":{"sha":"mb123"}}`)
		case strings.Contains(r.URL.Path, "/contents/"):
			_, _ = io.WriteString(w, "file body")
		}
	})
	ctx := t.Context()
	sha, err := client.MergeBase(ctx, apiRepo, "feature/x", "head")
	if err != nil || sha != "mb123" {
		t.Fatalf("%q %v", sha, err)
	}
	req := api.last()
	if req.path != "/repos/acme/widgets/compare/feature%2Fx...head" || req.header.Get("Authorization") != "Bearer tok" || req.header.Get("X-GitHub-Api-Version") == "" {
		t.Errorf("%+v", req)
	}
	for range 2 {
		data, err := client.FileAt(ctx, apiRepo, "abc", "dir/a b.go")
		if err != nil || string(data) != "file body" {
			t.Fatalf("%q %v", data, err)
		}
	}
	req = api.last()
	if req.path != "/repos/acme/widgets/contents/dir/a%20b.go" || req.query != "ref=abc" || req.header.Get("Accept") != acceptRaw || api.count() != 2 {
		t.Errorf("%+v requests %d", req, api.count())
	}
	for _, bad := range []github.Remote{{}, {Owner: "a"}} {
		if _, err := client.MergeBase(ctx, bad, "a", "b"); err == nil {
			t.Error("invalid repo accepted")
		}
	}
	if _, err := client.MergeBase(ctx, apiRepo, "", "b"); err == nil {
		t.Error("empty base accepted")
	}
	if _, err := client.FileAt(ctx, apiRepo, "", "p"); err == nil {
		t.Error("empty ref accepted")
	}
}

func TestMergeBaseRejectsIncompleteResponse(t *testing.T) {
	client, _ := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) { _, _ = io.WriteString(w, `{}`) })
	if _, err := client.MergeBase(t.Context(), apiRepo, "a", "b"); err == nil {
		t.Error("response without merge base accepted")
	}
}

func TestSendLimits(t *testing.T) {
	client, _ := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) { _, _ = io.WriteString(w, "0123456789abcdef") })
	ctx := t.Context()
	if _, err := client.send(ctx, request{method: http.MethodGet, target: client.base + "/x", limit: 8}); !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("%v", err)
	}
	res, err := client.send(ctx, request{method: http.MethodGet, target: client.base + "/x", limit: 8, tail: true})
	if err != nil || string(res.data) != "89abcdef" {
		t.Errorf("%q %v", res.data, err)
	}
}

const overviewReply = `{"viewer":{"login":"me"},"repository":{"mergeCommitAllowed":true,"squashMergeAllowed":true,"rebaseMergeAllowed":false,"deleteBranchOnMerge":true,"viewerDefaultMergeMethod":"SQUASH",
"pullRequest":{"id":"PR1","mergeable":"MERGEABLE","mergeStateStatus":"BEHIND","reviewDecision":"APPROVED","baseRefOid":"b","headRefOid":"h","maintainerCanModify":true,"viewerCanUpdate":true,
"assignees":{"nodes":[{"login":"a1"}]},"milestone":{"number":3,"title":"v1"},
"closingIssuesReferences":{"nodes":[{"number":8,"title":"Bug","url":"https://github.com/acme/widgets/issues/8"}]},
"commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"nodes":[
 {"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"SUCCESS","detailsUrl":"https://ci/build","isRequired":true,"databaseId":55,"checkSuite":{"workflowRun":{"databaseId":66}}},
 {"__typename":"StatusContext","context":"legacy","state":"PENDING","targetUrl":"https://ci/legacy","isRequired":false},
 {"__typename":"Other"}]}}}}]}}}}`

const queueReply = `{"repository":{"mergeQueue":{"id":"MQ"},"pullRequest":{"viewerCanEnableAutoMerge":true,"viewerCanDisableAutoMerge":false,"isInMergeQueue":true,"autoMergeRequest":{"mergeMethod":"SQUASH","enabledBy":{"login":"bob"}},"mergeQueueEntry":{"id":"E1","state":"QUEUED","position":2}}}}`

func TestOverviewDecoding(t *testing.T) {
	client, _ := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, body string) {
		if strings.Contains(body, "mergeQueue(branch") {
			gqlReply(w, queueReply)
			return
		}
		gqlReply(w, overviewReply)
	})
	o, err := client.Overview(t.Context(), apiRepo, 12, "main")
	if err != nil {
		t.Fatal(err)
	}
	if o.ID != "PR1" || o.Viewer != "me" || o.MergeState != "BEHIND" || !o.AllowSquash || o.AllowRebase || !o.DeleteBranchOnMerge || o.DefaultMethod != "squash" ||
		len(o.Assignees) != 1 || o.Milestone == nil || o.Milestone.Title != "v1" || len(o.Linked) != 1 || o.Linked[0].Number != 8 {
		t.Errorf("%+v", o)
	}
	if len(o.Checks) != 2 || !o.Checks[0].Required || o.Checks[0].JobID != 55 || o.Checks[0].RunID != 66 || o.Checks[0].Conclusion != "success" || o.Checks[1].Status != "in_progress" {
		t.Errorf("checks %+v", o.Checks)
	}
	if !o.Queue.Enabled || !o.Queue.InQueue || o.Queue.EntryID != "E1" || o.Queue.Position != 2 || o.AutoMerge == nil || o.AutoMerge.By != "bob" || !o.CanEnableAutoMerge || o.CanDisableAutoMerge {
		t.Errorf("queue %+v %+v", o.Queue, o.AutoMerge)
	}
}

func TestOverviewToleratesMissingQueueFields(t *testing.T) {
	client, _ := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, body string) {
		if strings.Contains(body, "mergeQueue(branch") {
			_, _ = io.WriteString(w, `{"errors":[{"message":"Field 'mergeQueue' doesn't exist"}]}`)
			return
		}
		gqlReply(w, overviewReply)
	})
	o, err := client.Overview(t.Context(), apiRepo, 12, "main")
	if err != nil || o.ID != "PR1" || o.Queue.Enabled {
		t.Errorf("%+v %v", o, err)
	}
	if _, err := client.Overview(t.Context(), github.Remote{}, 1, "main"); err == nil {
		t.Error("invalid repo accepted")
	}
}

func TestOverviewNotFound(t *testing.T) {
	client, _ := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		gqlReply(w, `{"viewer":{"login":"me"},"repository":null}`)
	})
	if _, err := client.Overview(t.Context(), apiRepo, 1, "main"); err == nil {
		t.Error("missing pull request accepted")
	}
}

func TestTimelineDecodingAndPaging(t *testing.T) {
	page := 0
	client, api := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		page++
		if page == 1 {
			gqlReply(w, `{"repository":{"pullRequest":{"timelineItems":{"pageInfo":{"hasNextPage":true,"endCursor":"C1"},"nodes":[
			 {"__typename":"IssueComment","id":"IC1","databaseId":1,"url":"u1","body":"hello","createdAt":"2026-11-26T10:00:00Z","viewerCanUpdate":true,"viewerCanDelete":false,"author":{"login":"carol"},"reactionGroups":[{"content":"THUMBS_UP","viewerHasReacted":true,"users":{"totalCount":2}}]},
			 {"__typename":"PullRequestReview","id":"R1","state":"APPROVED","body":"ok","submittedAt":"2026-11-26T11:00:00Z","author":{"login":"dave"}},
			 {"__typename":"PullRequestReview","id":"R2","state":"PENDING","author":{"login":"me"}},
			 {"__typename":"LabeledEvent","createdAt":"2026-11-26T09:00:00Z","actor":{"login":"x"},"label":{"name":"bug"}},
			 {"__typename":"HeadRefForcePushedEvent","createdAt":"2026-11-26T09:30:00Z","actor":{"login":"x"},"beforeCommit":{"oid":"1111111aaa"},"afterCommit":{"oid":"2222222bbb"}},
			 {"__typename":"ReviewRequestedEvent","createdAt":"2026-11-26T09:40:00Z","actor":{"login":"x"},"requestedReviewer":{"name":"core-team"}},
			 {"__typename":"AutoMergeEnabledEvent","createdAt":"2026-11-26T09:50:00Z","enabler":{"login":"bob"}},
			 {"__typename":"SomethingNew"}]}}}}`)
			return
		}
		gqlReply(w, `{"repository":{"pullRequest":{"timelineItems":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[
		 {"__typename":"MergedEvent","createdAt":"2026-11-26T12:00:00Z","actor":{"login":"x"},"commit":{"oid":"3333333ccc"}},
		 {"__typename":"RenamedTitleEvent","createdAt":"2026-11-26T12:10:00Z","actor":{"login":"x"},"previousTitle":"Old","currentTitle":"New"}]}}}}`)
	})
	events, err := client.Timeline(t.Context(), apiRepo, 12)
	if err != nil {
		t.Fatal(err)
	}
	if api.count() != 2 || len(events) != 8 {
		t.Fatalf("requests %d events %d: %+v", api.count(), len(events), events)
	}
	c := events[0]
	if !c.isComment() || c.Actor != "carol" || !c.CanEdit || c.CanDelete || len(c.Reactions) != 1 || c.Reactions[0].Count != 2 || !c.Reactions[0].Viewer {
		t.Errorf("%+v", c)
	}
	if !events[1].isReview() || events[1].Actor != "dave" || events[1].State != "APPROVED" || events[1].Time.IsZero() {
		t.Errorf("%+v", events[1])
	}
	if events[2].Detail != "bug" || events[3].Detail != "1111111 to 2222222" || events[4].Detail != "core-team" || events[5].Actor != "bob" || events[6].Detail != "3333333" || events[7].Detail != "Old to New" {
		t.Errorf("%+v", events[2:])
	}
}

func TestViewedFilesAndMutations(t *testing.T) {
	client, api := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, body string) {
		switch {
		case strings.Contains(body, "viewerViewedState"):
			gqlReply(w, `{"repository":{"pullRequest":{"files":{"pageInfo":{"hasNextPage":false},"nodes":[{"path":"a.go","viewerViewedState":"VIEWED"},{"path":"b.go","viewerViewedState":"UNVIEWED"}]}}}}`)
		case strings.Contains(body, "revertPullRequest"):
			gqlReply(w, `{"revertPullRequest":{"revertPullRequest":{"number":91}}}`)
		default:
			gqlReply(w, `{"x":{}}`)
		}
	})
	ctx := t.Context()
	viewed, err := client.ViewedFiles(ctx, apiRepo, 1)
	if err != nil || !viewed["a.go"] || viewed["b.go"] || len(viewed) != 2 {
		t.Errorf("%v %v", viewed, err)
	}
	type call struct {
		name string
		run  func() error
		want string
	}
	calls := []call{
		{"mark", func() error { return client.SetViewed(ctx, "PR1", "a.go", true) }, "markFileAsViewed"},
		{"unmark", func() error { return client.SetViewed(ctx, "PR1", "a.go", false) }, "unmarkFileAsViewed"},
		{"draft", func() error { return client.ConvertToDraft(ctx, "PR1") }, "convertPullRequestToDraft"},
		{"auto on", func() error { return client.SetAutoMerge(ctx, "PR1", "squash", true) }, "\"method\":\"SQUASH\""},
		{"auto off", func() error { return client.SetAutoMerge(ctx, "PR1", "", false) }, "disablePullRequestAutoMerge"},
		{"enqueue", func() error { return client.Enqueue(ctx, "PR1") }, "enqueuePullRequest"},
		{"dequeue", func() error { return client.Dequeue(ctx, "E1") }, "dequeuePullRequest"},
		{"react", func() error { return client.React(ctx, "C1", "HEART", true) }, "addReaction"},
		{"unreact", func() error { return client.React(ctx, "C1", "HEART", false) }, "removeReaction"},
		{"edit issue comment", func() error { return client.EditComment(ctx, commentIssue, "C1", "b") }, "updateIssueComment"},
		{"edit review comment", func() error { return client.EditComment(ctx, commentReview, "C1", "b") }, "updatePullRequestReviewComment"},
		{"delete issue comment", func() error { return client.DeleteComment(ctx, commentIssue, "C1") }, "deleteIssueComment"},
		{"delete review comment", func() error { return client.DeleteComment(ctx, commentReview, "C1") }, "deletePullRequestReviewComment"},
		{"delete review", func() error { return client.DeleteReview(ctx, "R1") }, "deletePullRequestReview"},
		{"file thread", func() error { return client.AddFileThread(ctx, "R1", "a.go", "b") }, "subjectType:FILE"},
	}
	for _, c := range calls {
		if err := c.run(); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if body := api.last().body; !strings.Contains(body, c.want) {
			t.Errorf("%s: body %q lacks %q", c.name, body, c.want)
		}
	}
	if n, err := client.Revert(ctx, "PR1"); err != nil || n != 91 {
		t.Errorf("revert %d %v", n, err)
	}
}

func TestRESTMutations(t *testing.T) {
	client, api := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) { w.WriteHeader(http.StatusAccepted) })
	ctx := t.Context()
	if err := client.UpdateBranch(ctx, apiRepo, 12, "head"); err != nil {
		t.Fatal(err)
	}
	if r := api.last(); r.method != http.MethodPut || r.path != "/repos/acme/widgets/pulls/12/update-branch" || !strings.Contains(r.body, `"expected_head_sha":"head"`) {
		t.Errorf("%+v", r)
	}
	if err := client.DeleteBranch(ctx, apiRepo, "feature/a b"); err != nil {
		t.Fatal(err)
	}
	if r := api.last(); r.method != http.MethodDelete || r.path != "/repos/acme/widgets/git/refs/heads/feature/a%20b" {
		t.Errorf("%+v", r)
	}
	if err := client.DeleteBranch(ctx, apiRepo, ""); err == nil {
		t.Error("empty branch accepted")
	}
	if err := client.RunAction(ctx, apiRepo, 66, actionRerunFailed); err != nil {
		t.Fatal(err)
	}
	if r := api.last(); r.method != http.MethodPost || r.path != "/repos/acme/widgets/actions/runs/66/rerun-failed-jobs" {
		t.Errorf("%+v", r)
	}
	for _, bad := range []func() error{
		func() error { return client.RunAction(ctx, apiRepo, 66, "delete") },
		func() error { return client.RunAction(ctx, apiRepo, 0, actionCancel) },
		func() error { return client.MarkNotification(ctx, "", false) },
		func() error { return client.MarkNotification(ctx, "a/b", false) },
	} {
		before := api.count()
		if err := bad(); err == nil || api.count() != before {
			t.Errorf("invalid input sent: %v", err)
		}
	}
	if err := client.MarkNotification(ctx, "77", false); err != nil {
		t.Fatal(err)
	}
	if r := api.last(); r.method != http.MethodPatch || r.path != "/notifications/threads/77" {
		t.Errorf("%+v", r)
	}
	if err := client.MarkNotification(ctx, "77", true); err != nil {
		t.Fatal(err)
	}
	if r := api.last(); r.method != http.MethodDelete {
		t.Errorf("%+v", r)
	}
}

func TestNotificationsConditionalRequests(t *testing.T) {
	modified := ""
	poll := "90"
	client, api := newAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		w.Header().Set("Last-Modified", "Thu, 26 Nov 2026 10:00:00 GMT")
		w.Header().Set("X-Poll-Interval", poll)
		if r.Header.Get("If-Modified-Since") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = io.WriteString(w, `[{"id":"1","unread":true,"reason":"mention","updated_at":"2026-11-26T10:00:00Z","subject":{"title":"T","url":"https://api.github.com/repos/acme/widgets/pulls/3","type":"PullRequest"}}]`)
	})
	ctx := t.Context()
	page, err := client.Notifications(ctx, apiRepo, modified)
	if err != nil || len(page.Items) != 1 || page.NotModified || page.Poll != 90*time.Second || page.Modified == "" {
		t.Fatalf("%+v %v", page, err)
	}
	if r := api.last(); r.path != "/repos/acme/widgets/notifications" || r.query != "per_page=50" {
		t.Errorf("%+v", r)
	}
	page, err = client.Notifications(ctx, apiRepo, page.Modified)
	if err != nil || !page.NotModified || len(page.Items) != 0 {
		t.Fatalf("%+v %v", page, err)
	}
	if api.last().header.Get("If-Modified-Since") == "" {
		t.Error("If-Modified-Since not sent")
	}
	poll = "10"
	page, _ = client.Notifications(ctx, apiRepo, "")
	if page.Poll != minPollInterval {
		t.Errorf("poll %v, want the minimum", page.Poll)
	}
}

func TestLocalBranchPRsQuery(t *testing.T) {
	client, api := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		gqlReply(w, `{"repository":{"b0":{"nodes":[]},"b1":{"nodes":[{"number":4,"title":"Topic","isDraft":true,"url":"u","updatedAt":"2026-11-26T10:00:00Z","author":{"login":"me"},"headRefName":"topic","baseRefName":"main","headRefOid":"abc"}]}}}`)
	})
	prs, err := client.LocalBranchPRs(t.Context(), apiRepo, []string{"main", "topic"})
	if err != nil || len(prs) != 1 || prs[0].Number != 4 || !prs[0].Draft || prs[0].HeadRef != "topic" || prs[0].Author != "me" {
		t.Fatalf("%+v %v", prs, err)
	}
	var sent struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal([]byte(api.last().body), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Variables["b1"] != "topic" || !strings.Contains(sent.Query, "b1:pullRequests") || strings.Contains(sent.Query, "topic") {
		t.Errorf("branch names must travel as variables: %+v", sent)
	}
	if prs, err := client.LocalBranchPRs(t.Context(), apiRepo, nil); err != nil || prs != nil {
		t.Errorf("%v %v", prs, err)
	}
}

func TestCompareDecoding(t *testing.T) {
	client, api := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		_, _ = io.WriteString(w, `{"merge_base_commit":{"sha":"mb"},"files":[{"filename":"a.go","status":"modified","additions":2,"deletions":1,"patch":"@@"},{"filename":"n.go","previous_filename":"o.go","status":"renamed"}],"commits":[{"sha":"c1","html_url":"u","commit":{"message":"msg","author":{"name":"Al","date":"2026-11-26T10:00:00Z"}}}]}`)
	})
	c, err := client.Compare(t.Context(), apiRepo, "base", "owner:head")
	if err != nil || c.BaseSHA != "mb" || len(c.Files) != 2 || c.Files[1].PrevPath != "o.go" || len(c.Commits) != 1 || c.Commits[0].Author != "Al" {
		t.Fatalf("%+v %v", c, err)
	}
	if !strings.HasSuffix(api.last().path, "/compare/base...owner:head") {
		t.Errorf("%s", api.last().path)
	}
}

func TestJobLogsAndViewer(t *testing.T) {
	client, api := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) { _, _ = io.WriteString(w, "log line\n") })
	text, err := client.JobLogs(t.Context(), apiRepo, 55)
	if err != nil || text != "log line\n" || api.last().path != "/repos/acme/widgets/actions/jobs/55/logs" {
		t.Errorf("%q %v %s", text, err, api.last().path)
	}
	if _, err := client.JobLogs(t.Context(), apiRepo, 0); err == nil {
		t.Error("job 0 accepted")
	}
}
