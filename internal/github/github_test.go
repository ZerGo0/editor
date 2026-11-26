package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

var testRepo = Remote{Host: "github.com", Owner: "octo", Name: "repo"}

const pullFixture = `{
  "node_id": "PR_node", "number": 7, "title": "Add thing", "body": null, "state": "open",
  "html_url": "https://github.com/octo/repo/pull/7", "draft": true, "merged": false, "mergeable": null,
  "user": {"login": "alice"},
  "head": {"ref": "feature", "sha": "abc123", "user": {"login": "alice"}, "repo": {"name": "repo-fork", "owner": {"login": "alice"}}},
  "base": {"ref": "main", "sha": "def456"},
  "labels": [{"name": "bug"}, {"name": "ui"}],
  "requested_reviewers": [{"login": "bob"}],
  "created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-01-03T03:04:05Z",
  "additions": 10, "deletions": 2, "changed_files": 3, "comments": 1, "unknown_field": {"x": 1}
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New("github.com", "tok")
	c.SetBaseURL(srv.URL)
	return c, srv
}

func readJSON(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("bad request body %q: %v", data, err)
	}
	return out
}

func TestParseRemote(t *testing.T) {
	tests := []struct {
		in   string
		want Remote
		ok   bool
	}{
		{"https://github.com/octo/repo.git", Remote{"github.com", "octo", "repo"}, true},
		{"https://github.com/octo/repo", Remote{"github.com", "octo", "repo"}, true},
		{"https://user:pw@github.com/octo/repo/", Remote{"github.com", "octo", "repo"}, true},
		{"git@github.com:octo/repo.git", Remote{"github.com", "octo", "repo"}, true},
		{"git@ghe.example.com:octo/repo", Remote{"ghe.example.com", "octo", "repo"}, true},
		{"ssh://git@github.com/octo/repo.git", Remote{"github.com", "octo", "repo"}, true},
		{"ssh://git@ghe.example.com:2222/octo/repo", Remote{"ghe.example.com", "octo", "repo"}, true},
		{"https://github.com/octo", Remote{}, false},
		{"https://github.com/a/b/c", Remote{}, false},
		{"/local/path/repo", Remote{}, false},
		{"", Remote{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseRemote(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseRemote(%q) = %+v, %v; want %+v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func stubRunner(t *testing.T, fn Runner) {
	t.Helper()
	previous := runCommand
	runCommand = fn
	t.Cleanup(func() { runCommand = previous })
}

func TestToken(t *testing.T) {
	var gotArgs []string
	stubRunner(t, func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		gotArgs = append([]string{name}, args...)
		return []byte("secret\n"), nil
	})
	token, err := Token(context.Background(), "github.com")
	if err != nil || token != "secret" {
		t.Fatalf("got %q, %v", token, err)
	}
	if want := []string{"gh", "auth", "token", "--hostname", "github.com"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args %v", gotArgs)
	}

	stubRunner(t, func(context.Context, string, string, ...string) ([]byte, error) {
		return nil, &exec.Error{Name: "gh", Err: exec.ErrNotFound}
	})
	if _, err := Token(context.Background(), "github.com"); !errors.Is(err, ErrNoGH) {
		t.Fatalf("want ErrNoGH, got %v", err)
	}

	stubRunner(t, func(context.Context, string, string, ...string) ([]byte, error) {
		return nil, errors.New("exit status 1")
	})
	if _, err := Token(context.Background(), "github.com"); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("want ErrNotLoggedIn, got %v", err)
	}

	stubRunner(t, func(context.Context, string, string, ...string) ([]byte, error) { return []byte("  \n"), nil })
	if _, err := Token(context.Background(), "github.com"); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("want ErrNotLoggedIn for empty token, got %v", err)
	}
	if _, err := Token(context.Background(), "--evil"); err == nil {
		t.Fatal("want error for option-like host")
	}
}

func TestCheckout(t *testing.T) {
	var gotDir string
	var gotArgs []string
	stubRunner(t, func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		gotDir, gotArgs = dir, append([]string{name}, args...)
		return nil, nil
	})
	if err := Checkout(context.Background(), "/work", 12); err != nil {
		t.Fatal(err)
	}
	if gotDir != "/work" || !reflect.DeepEqual(gotArgs, []string{"gh", "pr", "checkout", "12"}) {
		t.Fatalf("dir %q args %v", gotDir, gotArgs)
	}
	if err := Checkout(context.Background(), "", 12); err == nil {
		t.Fatal("want error for empty dir")
	}
	if err := Checkout(context.Background(), "/work", 0); err == nil {
		t.Fatal("want error for bad number")
	}
	stubRunner(t, func(context.Context, string, string, ...string) ([]byte, error) {
		return nil, &exec.Error{Name: "gh", Err: exec.ErrNotFound}
	})
	if err := Checkout(context.Background(), "/work", 1); !errors.Is(err, ErrNoGH) {
		t.Fatalf("want ErrNoGH, got %v", err)
	}
}

func TestNewBaseURLs(t *testing.T) {
	c := New("github.com", "t")
	if c.restURL != "https://api.github.com" || c.graphqlURL != "https://api.github.com/graphql" {
		t.Fatalf("%s %s", c.restURL, c.graphqlURL)
	}
	c = New("ghe.example.com", "t")
	if c.restURL != "https://ghe.example.com/api/v3" || c.graphqlURL != "https://ghe.example.com/api/graphql" {
		t.Fatalf("%s %s", c.restURL, c.graphqlURL)
	}
}

func TestGetConvertsPull(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/octo/repo/pulls/7" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		io.WriteString(w, pullFixture)
	})
	pr, err := c.Get(context.Background(), testRepo, 7)
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number != 7 || pr.Title != "Add thing" || pr.Body != "" || !pr.Draft || pr.Mergeable != nil {
		t.Fatalf("%+v", pr)
	}
	if pr.HeadOwner != "alice" || pr.HeadRepo != "repo-fork" || pr.HeadSHA != "abc123" || pr.HeadRef != "feature" || pr.BaseRef != "main" {
		t.Fatalf("%+v", pr)
	}
	if !reflect.DeepEqual(pr.Labels, []string{"bug", "ui"}) || !reflect.DeepEqual(pr.Reviewers, []string{"bob"}) {
		t.Fatalf("%+v", pr)
	}
	if pr.Created.Year() != 2026 || pr.ChangedFiles != 3 || pr.NodeID != "PR_node" {
		t.Fatalf("%+v", pr)
	}
}

func TestGetValidation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"title": "no number"}`)
	})
	if _, err := c.Get(context.Background(), testRepo, 7); err == nil {
		t.Fatal("want error for missing number")
	}
	if _, err := c.Get(context.Background(), testRepo, 0); err == nil {
		t.Fatal("want error for number 0")
	}
	if _, err := c.Get(context.Background(), Remote{}, 1); err == nil {
		t.Fatal("want error for empty repo")
	}
	bad, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "not json") })
	if _, err := bad.Get(context.Background(), testRepo, 1); err == nil {
		t.Fatal("want decode error")
	}
	nohead, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"number": 1, "state": "open"}`)
	})
	if _, err := nohead.Get(context.Background(), testRepo, 1); err == nil {
		t.Fatal("want error for missing head")
	}
}

func TestETagReuse(t *testing.T) {
	var calls, conditional int
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("If-None-Match") == `"v1"` {
			conditional++
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		io.WriteString(w, pullFixture)
	})
	first, err := c.Get(context.Background(), testRepo, 7)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Get(context.Background(), testRepo, 7)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || conditional != 1 {
		t.Fatalf("calls %d conditional %d", calls, conditional)
	}
	if first.Title != second.Title || second.Number != 7 {
		t.Fatalf("%+v %+v", first, second)
	}
}

func TestCacheEvictsOldest(t *testing.T) {
	cache := newResponseCache(2)
	cache.put(cacheEntry{key: "a", etag: "1"})
	cache.put(cacheEntry{key: "b", etag: "2"})
	cache.get("a")
	cache.put(cacheEntry{key: "c", etag: "3"})
	if _, ok := cache.get("b"); ok {
		t.Fatal("b should be evicted")
	}
	if _, ok := cache.get("a"); !ok || cache.len() != 2 {
		t.Fatal("a should stay")
	}
}

func TestFilesPagination(t *testing.T) {
	var srvURL string
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			io.WriteString(w, `[{"filename": "b.go", "status": "renamed", "previous_filename": "a.go", "additions": 0, "deletions": 0}]`)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/repos/octo/repo/pulls/7/files?per_page=100&page=2>; rel="next", <%s/x>; rel="last"`, srvURL, srvURL))
		io.WriteString(w, `[{"filename": "a.go", "status": "modified", "additions": 3, "deletions": 1, "patch": "@@ -1 +1 @@"}]`)
	})
	srvURL = srv.URL
	files, err := c.Files(context.Background(), testRepo, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Path != "a.go" || files[0].Patch == "" || files[1].PrevPath != "a.go" {
		t.Fatalf("%+v", files)
	}
}

func TestPaginationRejectsForeignHost(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://evil.example.com/x>; rel="next"`)
		io.WriteString(w, `[]`)
	})
	if _, err := c.Files(context.Background(), testRepo, 7); err == nil {
		t.Fatal("want error for foreign pagination host")
	}
}

func TestFilesRequireFilename(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `[{"status": "added"}]`) })
	if _, err := c.Files(context.Background(), testRepo, 7); err == nil {
		t.Fatal("want error")
	}
}

func TestRateLimited(t *testing.T) {
	reset := time.Now().Add(time.Hour).Truncate(time.Second)
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprint(reset.Unix()))
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"message": "API rate limit exceeded"}`)
	})
	_, err := c.Get(context.Background(), testRepo, 7)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
	var rl *RateLimitError
	if !errors.As(err, &rl) || !rl.Reset.Equal(reset) || rl.Status != http.StatusForbidden {
		t.Fatalf("%+v", rl)
	}
}

func TestForbiddenWithoutRateLimitIsAPIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "10")
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"message": "Resource not accessible"}`)
	})
	_, err := c.Get(context.Background(), testRepo, 7)
	var apiErr *APIError
	if errors.Is(err, ErrRateLimited) || !errors.As(err, &apiErr) || apiErr.Status != 403 || apiErr.Message != "Resource not accessible" {
		t.Fatalf("%v", err)
	}
}

func TestNotFoundError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"message": "Not Found"}`)
	})
	_, err := c.Get(context.Background(), testRepo, 7)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 404 || apiErr.Message != "Not Found" {
		t.Fatalf("%v", err)
	}
}

func TestListSelection(t *testing.T) {
	var paths, queries []string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		queries = append(queries, r.URL.RawQuery)
		if strings.HasPrefix(r.URL.Path, "/search/") {
			io.WriteString(w, `{"total_count": 2, "items": [
			 {"number": 3, "title": "T", "state": "open", "html_url": "u", "draft": true, "user": {"login": "carol"}, "labels": [{"name": "x"}], "comments": 4, "pull_request": {"merged_at": null}},
			 {"number": 4, "title": "Issue", "state": "open"}]}`)
			return
		}
		io.WriteString(w, "["+pullFixture+"]")
	})
	ctx := context.Background()

	prs, err := c.List(ctx, testRepo, ListOptions{State: StateClosed, Page: 2, PerPage: 500})
	if err != nil || len(prs) != 1 || prs[0].Number != 7 {
		t.Fatalf("%v %+v", err, prs)
	}
	if paths[0] != "/repos/octo/repo/pulls" || !strings.Contains(queries[0], "state=closed") || !strings.Contains(queries[0], "page=2") || !strings.Contains(queries[0], "per_page=100") {
		t.Fatalf("%v %v", paths, queries)
	}

	prs, err = c.List(ctx, testRepo, ListOptions{ReviewRequested: "bob", Query: "label:bug"})
	if err != nil {
		t.Fatal(err)
	}
	if paths[1] != "/search/issues" {
		t.Fatalf("path %s", paths[1])
	}
	q := queryParam(t, queries[1], "q")
	for _, want := range []string{"repo:octo/repo", "is:pr", "is:open", "review-requested:bob", "label:bug"} {
		if !strings.Contains(q, want) {
			t.Errorf("q %q lacks %q", q, want)
		}
	}
	if len(prs) != 1 || prs[0].Number != 3 || !prs[0].Draft || prs[0].Author != "carol" || prs[0].Comments != 4 {
		t.Fatalf("%+v", prs)
	}

	for _, opts := range []ListOptions{{Assignee: "me"}, {Author: "me", State: StateAll}} {
		if _, err := c.List(ctx, testRepo, opts); err != nil {
			t.Fatal(err)
		}
		if paths[len(paths)-1] != "/search/issues" {
			t.Fatalf("%+v should use search", opts)
		}
	}
	if strings.Contains(queryParam(t, queries[len(queries)-1], "q"), "is:open") {
		t.Fatal("state all must not add is:open")
	}
	if _, err := c.List(ctx, testRepo, ListOptions{State: "bogus"}); err == nil {
		t.Fatal("want error for bad state")
	}
}

func queryParam(t *testing.T, raw, key string) string {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	return values.Get(key)
}

func TestSearchMissingItems(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"total_count": 0}`) })
	if _, err := c.List(context.Background(), testRepo, ListOptions{Query: "x"}); err == nil {
		t.Fatal("want error")
	}
}

func TestCurrentBranchPR(t *testing.T) {
	empty := false
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("head"); got != "alice:feature" {
			t.Errorf("head %q", got)
		}
		if empty {
			io.WriteString(w, "[]")
			return
		}
		io.WriteString(w, "["+pullFixture+"]")
	})
	pr, err := c.CurrentBranchPR(context.Background(), testRepo, "feature", "alice")
	if err != nil || pr == nil || pr.Number != 7 {
		t.Fatalf("%v %+v", err, pr)
	}
	empty = true
	pr, err = c.CurrentBranchPR(context.Background(), testRepo, "feature", "alice")
	if err != nil || pr != nil {
		t.Fatalf("%v %+v", err, pr)
	}
}

func TestChecksCombined(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/octo/repo/commits/abc/check-runs":
			io.WriteString(w, `{"total_count": 1, "check_runs": [{"name": "build", "status": "completed", "conclusion": "success", "html_url": "http://ci/1"}]}`)
		case "/repos/octo/repo/commits/abc/status":
			io.WriteString(w, `{"state": "pending", "statuses": [{"context": "legacy", "state": "pending", "target_url": "http://ci/2"}, {"context": "lint", "state": "failure", "target_url": null}]}`)
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	})
	checks, err := c.Checks(context.Background(), testRepo, "abc")
	if err != nil {
		t.Fatal(err)
	}
	want := []Check{
		{"build", "completed", "success", "http://ci/1"},
		{"legacy", "in_progress", "", "http://ci/2"},
		{"lint", "completed", "failure", ""},
	}
	if !reflect.DeepEqual(checks, want) {
		t.Fatalf("%+v", checks)
	}
}

func TestCommits(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"sha": "s1", "html_url": "u", "commit": {"message": "msg", "author": {"name": "Alice A", "date": "2026-01-01T00:00:00Z"}}, "author": null}]`)
	})
	commits, err := c.Commits(context.Background(), testRepo, 7)
	if err != nil || len(commits) != 1 || commits[0].Author != "Alice A" || commits[0].Date.IsZero() {
		t.Fatalf("%v %+v", err, commits)
	}
}

func TestCommentsAndReviews(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/octo/repo/issues/7/comments":
			io.WriteString(w, `[{"node_id": "IC_1", "id": 11, "user": {"login": "bob"}, "body": "hi", "created_at": "2026-01-01T00:00:00Z", "html_url": "u"}]`)
		case "/repos/octo/repo/pulls/7/reviews":
			io.WriteString(w, `[{"node_id": "R_1", "id": 21, "user": {"login": "bob"}, "state": "APPROVED", "body": "ok", "commit_id": "abc", "submitted_at": "2026-01-02T00:00:00Z"}]`)
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	})
	comments, err := c.Comments(context.Background(), testRepo, 7)
	if err != nil || len(comments) != 1 || comments[0].ID != "IC_1" || comments[0].DatabaseID != 11 {
		t.Fatalf("%v %+v", err, comments)
	}
	reviews, err := c.Reviews(context.Background(), testRepo, 7)
	if err != nil || len(reviews) != 1 || reviews[0].State != "APPROVED" || reviews[0].CommitID != "abc" {
		t.Fatalf("%v %+v", err, reviews)
	}
}

func TestReviewThreadsPagination(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		body := readJSON(t, r)
		vars := body["variables"].(map[string]any)
		calls++
		if calls == 1 {
			if vars["after"] != nil {
				t.Errorf("first after %v", vars["after"])
			}
			io.WriteString(w, `{"data": {"repository": {"pullRequest": {"reviewThreads": {"pageInfo": {"hasNextPage": true, "endCursor": "CUR"}, "nodes": [
			 {"id": "T1", "isResolved": false, "isOutdated": false, "path": "a.go", "diffSide": "RIGHT", "line": 5, "startLine": null, "originalLine": 5,
			  "comments": {"nodes": [{"id": "C1", "databaseId": 100, "url": "u", "body": "b", "createdAt": "2026-01-01T00:00:00Z", "author": {"login": "bob"}}]}}]}}}}}`)
			return
		}
		if vars["after"] != "CUR" {
			t.Errorf("second after %v", vars["after"])
		}
		io.WriteString(w, `{"data": {"repository": {"pullRequest": {"reviewThreads": {"pageInfo": {"hasNextPage": false, "endCursor": null}, "nodes": [
		 {"id": "T2", "isResolved": true, "isOutdated": true, "path": "b.go", "diffSide": "LEFT", "line": null, "originalLine": 9, "originalStartLine": 7, "comments": {"nodes": []}}]}}}}}`)
	})
	threads, err := c.ReviewThreads(context.Background(), testRepo, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 2 || calls != 2 {
		t.Fatalf("%d threads, %d calls", len(threads), calls)
	}
	t1, t2 := threads[0], threads[1]
	if t1.ID != "T1" || t1.Line != 5 || t1.Side != SideRight || len(t1.Comments) != 1 || t1.Comments[0].DatabaseID != 100 || t1.Comments[0].Author != "bob" {
		t.Fatalf("%+v", t1)
	}
	if !t2.Resolved || !t2.Outdated || t2.Line != 9 || t2.StartLine != 7 || t2.Side != SideLeft {
		t.Fatalf("%+v", t2)
	}
}

func TestGraphQLErrors(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": null, "errors": [{"message": "Could not resolve", "type": "NOT_FOUND"}, {"message": "second"}]}`)
	})
	_, err := c.ReviewThreads(context.Background(), testRepo, 7)
	var gqlErr *GraphQLError
	if !errors.As(err, &gqlErr) || len(gqlErr.Errors) != 2 || !strings.Contains(err.Error(), "Could not resolve") {
		t.Fatalf("%v", err)
	}

	limited, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"errors": [{"message": "slow down", "type": "RATE_LIMITED"}]}`)
	})
	if _, err := limited.ReviewThreads(context.Background(), testRepo, 7); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("%v", err)
	}

	missing, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": {"repository": {"pullRequest": null}}}`)
	})
	if _, err := missing.ReviewThreads(context.Background(), testRepo, 7); err == nil {
		t.Fatal("want not found error")
	}

	nodata, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) })
	if _, err := nodata.ReviewThreads(context.Background(), testRepo, 7); err == nil {
		t.Fatal("want error for missing data")
	}

	badThread, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": {"repository": {"pullRequest": {"reviewThreads": {"pageInfo": {"hasNextPage": false}, "nodes": [{"path": "a"}]}}}}}`)
	})
	if _, err := badThread.ReviewThreads(context.Background(), testRepo, 7); err == nil {
		t.Fatal("want error for thread without id")
	}
}

func TestMergeRequestBody(t *testing.T) {
	var body map[string]any
	merged := true
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/repos/octo/repo/pulls/7/merge" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		body = readJSON(t, r)
		fmt.Fprintf(w, `{"merged": %v, "message": "Pull Request not mergeable", "sha": "x"}`, merged)
	})
	ctx := context.Background()
	if err := c.Merge(ctx, testRepo, 7, MergeMethodSquash, "Title", "Msg"); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"merge_method": "squash", "commit_title": "Title", "commit_message": "Msg"}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("%v", body)
	}
	if err := c.Merge(ctx, testRepo, 7, MergeMethodMerge, "", ""); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(body, map[string]any{"merge_method": "merge"}) {
		t.Fatalf("%v", body)
	}
	if err := c.Merge(ctx, testRepo, 7, "fast-forward", "", ""); err == nil {
		t.Fatal("want error for bad method")
	}
	merged = false
	if err := c.Merge(ctx, testRepo, 7, MergeMethodRebase, "", ""); err == nil {
		t.Fatal("want error when not merged")
	}
}

func TestCreateRequestBody(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/octo/repo/pulls" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		body = readJSON(t, r)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, pullFixture)
	})
	pr, err := c.Create(context.Background(), testRepo, CreateOptions{Title: "T", Body: "B", Head: "alice:feature", Base: "main", Draft: true})
	if err != nil || pr.Number != 7 {
		t.Fatalf("%v %+v", err, pr)
	}
	want := map[string]any{"title": "T", "body": "B", "head": "alice:feature", "base": "main", "draft": true}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("%v", body)
	}
	if _, err := c.Create(context.Background(), testRepo, CreateOptions{Title: "T"}); err == nil {
		t.Fatal("want error for missing head/base")
	}
}

func TestCreateReviewComment(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/octo/repo/pulls/7/comments" {
			t.Errorf("path %s", r.URL.Path)
		}
		body = readJSON(t, r)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"node_id": "PRRC_1", "id": 55, "user": {"login": "me"}, "body": "x"}`)
	})
	opts := ReviewCommentOptions{Body: "x", Path: "a.go", Line: 10, StartLine: 8, CommitID: "abc"}
	comment, err := c.CreateReviewComment(context.Background(), testRepo, 7, opts)
	if err != nil || comment.DatabaseID != 55 {
		t.Fatalf("%v %+v", err, comment)
	}
	want := map[string]any{"body": "x", "commit_id": "abc", "path": "a.go", "line": float64(10), "side": "RIGHT", "start_line": float64(8), "start_side": "RIGHT"}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("%v", body)
	}
	for _, bad := range []ReviewCommentOptions{
		{Body: "x", Path: "a", Line: 0, CommitID: "c"},
		{Body: "x", Path: "a", Line: 3, StartLine: 3, CommitID: "c"},
		{Body: "x", Path: "a", Line: 3, Side: "UP", CommitID: "c"},
		{Body: "x", Path: "a", Line: 3},
		{Path: "a", Line: 3, CommitID: "c"},
	} {
		if _, err := c.CreateReviewComment(context.Background(), testRepo, 7, bad); err == nil {
			t.Errorf("want error for %+v", bad)
		}
	}
}

func TestPendingReviewFlow(t *testing.T) {
	var graphqlVars []map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/octo/repo/pulls/7/reviews":
			if body := readJSON(t, r); len(body) != 0 {
				t.Errorf("start review body %v", body)
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"node_id": "PRR_1", "id": 1, "state": "PENDING"}`)
		case "/graphql":
			body := readJSON(t, r)
			graphqlVars = append(graphqlVars, body["variables"].(map[string]any))
			if strings.Contains(body["query"].(string), "submitPullRequestReview") {
				io.WriteString(w, `{"data": {"submitPullRequestReview": {"pullRequestReview": {"id": "PRR_1", "state": "APPROVED"}}}}`)
				return
			}
			io.WriteString(w, `{"data": {"addPullRequestReviewThread": {"thread": {"id": "T9", "isResolved": false, "isOutdated": false, "path": "a.go", "diffSide": "LEFT", "line": 4, "comments": {"nodes": []}}}}}`)
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	})
	ctx := context.Background()
	id, err := c.StartReview(ctx, testRepo, 7)
	if err != nil || id != "PRR_1" {
		t.Fatalf("%v %q", err, id)
	}
	thread, err := c.AddReviewComment(ctx, id, ReviewCommentOptions{Body: "b", Path: "a.go", Line: 4, Side: SideLeft})
	if err != nil || thread.ID != "T9" || thread.Side != SideLeft {
		t.Fatalf("%v %+v", err, thread)
	}
	if v := graphqlVars[0]; v["review"] != "PRR_1" || v["side"] != "LEFT" || v["line"] != float64(4) {
		t.Fatalf("%v", v)
	}
	if err := c.SubmitReview(ctx, id, ReviewApprove, ""); err != nil {
		t.Fatal(err)
	}
	if v := graphqlVars[1]; v["event"] != "APPROVE" || v["body"] != nil || v["review"] != "PRR_1" {
		t.Fatalf("%v", v)
	}
	if err := c.SubmitReview(ctx, id, "MERGE", ""); err == nil {
		t.Fatal("want error for bad event")
	}
}

func TestResolveAndUnresolve(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		if strings.Contains(body["query"].(string), "unresolveReviewThread") {
			io.WriteString(w, `{"data": {"unresolveReviewThread": {"thread": {"id": "T1", "isResolved": false}}}}`)
			return
		}
		io.WriteString(w, `{"data": {"resolveReviewThread": {"thread": {"id": "T1", "isResolved": true}}}}`)
	})
	if err := c.ResolveThread(context.Background(), "T1"); err != nil {
		t.Fatal(err)
	}
	if err := c.UnresolveThread(context.Background(), "T1"); err != nil {
		t.Fatal(err)
	}
	if err := c.ResolveThread(context.Background(), ""); err == nil {
		t.Fatal("want error")
	}
}

func TestReplyAndAddComment(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/graphql":
			io.WriteString(w, `{"data": {"addPullRequestReviewThreadReply": {"comment": {"id": "C2", "databaseId": 2, "body": "re", "author": {"login": "me"}}}}}`)
		case "/repos/octo/repo/issues/7/comments":
			if readJSON(t, r)["body"] != "hello" {
				t.Error("bad body")
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"node_id": "IC_2", "id": 3, "body": "hello"}`)
		case "/repos/octo/repo/pulls/7/comments/9/replies":
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"node_id": "PRRC_3", "id": 4, "body": "re"}`)
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	})
	ctx := context.Background()
	if comment, err := c.Reply(ctx, "T1", "re"); err != nil || comment.ID != "C2" {
		t.Fatalf("%v %+v", err, comment)
	}
	if comment, err := c.AddComment(ctx, testRepo, 7, "hello"); err != nil || comment.DatabaseID != 3 {
		t.Fatalf("%v %+v", err, comment)
	}
	if comment, err := c.ReplyToComment(ctx, testRepo, 7, 9, "re"); err != nil || comment.DatabaseID != 4 {
		t.Fatalf("%v %+v", err, comment)
	}
	if _, err := c.AddComment(ctx, testRepo, 7, ""); err == nil {
		t.Fatal("want error for empty body")
	}
}

func TestMarkReady(t *testing.T) {
	step := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		step++
		body := readJSON(t, r)
		if step == 1 {
			io.WriteString(w, `{"data": {"repository": {"pullRequest": {"id": "PR_node"}}}}`)
			return
		}
		if body["variables"].(map[string]any)["id"] != "PR_node" {
			t.Errorf("%v", body["variables"])
		}
		io.WriteString(w, `{"data": {"markPullRequestReadyForReview": {"pullRequest": {"id": "PR_node", "isDraft": false}}}}`)
	})
	if err := c.MarkReady(context.Background(), testRepo, 7); err != nil || step != 2 {
		t.Fatalf("%v %d", err, step)
	}
}

func TestRequestReviewersAndLabels(t *testing.T) {
	var bodies = map[string]map[string]any{}
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		bodies[r.Method+" "+r.URL.Path] = readJSON(t, r)
		io.WriteString(w, `{}`)
	})
	ctx := context.Background()
	if err := c.RequestReviewers(ctx, testRepo, 7, []string{"bob"}, nil); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"reviewers": []any{"bob"}, "team_reviewers": []any{}}
	if got := bodies["POST /repos/octo/repo/pulls/7/requested_reviewers"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
	if err := c.SetLabels(ctx, testRepo, 7, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if got := bodies["PUT /repos/octo/repo/issues/7/labels"]; !reflect.DeepEqual(got, map[string]any{"labels": []any{"a", "b"}}) {
		t.Fatalf("%v", got)
	}
	if err := c.RequestReviewers(ctx, testRepo, 7, nil, nil); err == nil {
		t.Fatal("want error")
	}
}
