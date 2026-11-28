//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	mockDefaultPerPage = 30
	mockPollSeconds    = "60"
	mockAPIURL         = "https://api.github.com"
	mockWebURL         = "https://github.com"
	mockTimeFormat     = time.RFC3339
	mockMeQualifier    = "@me"
)

type githubMock struct {
	srv       *httptest.Server
	repos     map[string]*mockRepo
	mu        sync.Mutex
	unhandled []string
}

var (
	sharedMock     *githubMock
	sharedMockOnce sync.Once
)

func sharedGitHubMock() *githubMock {
	sharedMockOnce.Do(func() {
		m := &githubMock{repos: map[string]*mockRepo{}}
		for _, r := range mockRepos() {
			m.repos[r.key()] = r
		}
		m.srv = httptest.NewServer(http.HandlerFunc(m.serve))
		sharedMock = m
	})
	return sharedMock
}

func stopGitHubMock() {
	if sharedMock != nil {
		sharedMock.srv.Close()
	}
}

func (m *githubMock) URL() string { return m.srv.URL }

func (m *githubMock) Unhandled() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.unhandled)
}

func (m *githubMock) miss(w http.ResponseWriter, r *http.Request, detail string) {
	m.mu.Lock()
	m.unhandled = append(m.unhandled, r.Method+" "+r.URL.RequestURI()+" "+detail)
	m.mu.Unlock()
	mockError(w, http.StatusNotFound, "mock does not serve this route")
}

func mockError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (m *githubMock) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+mockToken {
		mockError(w, http.StatusUnauthorized, "Bad credentials")
		return
	}
	path := r.URL.Path
	switch {
	case path == "/graphql" && r.Method == http.MethodPost:
		m.graphql(w, r)
	case path == "/user":
		respondJSON(w, map[string]any{"login": mockViewer, "name": "Ada Lovelace", "avatar_url": mockWebURL + "/" + mockViewer + ".png"})
	case path == "/search/issues":
		m.search(w, r)
	case strings.HasPrefix(path, "/repos/"):
		m.repoRoute(w, r)
	default:
		m.miss(w, r, "")
	}
}

func (m *githubMock) repoRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	const prefix = 3
	if len(parts) < prefix+1 {
		m.miss(w, r, "short path")
		return
	}
	repo, ok := m.repos[parts[1]+"/"+parts[2]]
	if !ok {
		mockError(w, http.StatusNotFound, "Not Found")
		return
	}
	rest := parts[prefix:]
	switch {
	case len(rest) == 1 && rest[0] == "pulls":
		repo.listPulls(w, r)
	case len(rest) == 1 && rest[0] == "notifications":
		repo.notifications(w, r)
	case len(rest) == 1 && rest[0] == "milestones":
		paginate(w, r, len(repo.Milestones), func(i int) any { return repo.Milestones[i].rest() })
	case len(rest) == 1 && rest[0] == "labels":
		paginate(w, r, len(repo.Labels), func(i int) any { return repo.Labels[i].rest() })
	case len(rest) >= 2 && rest[0] == "pulls":
		m.pullRoute(w, r, repo, rest[1:])
	case len(rest) == 3 && rest[0] == "issues" && rest[2] == "comments":
		repo.issueComments(w, r, rest[1])
	case len(rest) == 3 && rest[0] == "commits":
		repo.commitChecks(w, r, rest[1], rest[2])
	case len(rest) == 2 && rest[0] == "compare":
		respondJSON(w, map[string]any{"merge_base_commit": map[string]any{"sha": mockSHA(0)}, "files": []any{}, "commits": []any{}})
	default:
		m.miss(w, r, "")
	}
}

func (m *githubMock) pullRoute(w http.ResponseWriter, r *http.Request, repo *mockRepo, rest []string) {
	number, err := strconv.Atoi(rest[0])
	pr, ok := repo.pr(number)
	if err != nil || !ok {
		mockError(w, http.StatusNotFound, "Not Found")
		return
	}
	if len(rest) == 1 {
		respondJSON(w, repo.prJSON(r, pr, true))
		return
	}
	switch rest[1] {
	case "files":
		paginate(w, r, len(pr.Files), func(i int) any { return pr.Files[i].rest() })
	case "commits":
		paginate(w, r, len(pr.Commits), func(i int) any { return pr.Commits[i].rest(pr.Number, i) })
	case "reviews":
		paginate(w, r, len(pr.Reviews), func(i int) any { return pr.Reviews[i].rest(repo, pr.Number) })
	default:
		m.miss(w, r, "")
	}
}

func queryInt(r *http.Request, key string, fallback int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(key)); err == nil && n > 0 {
		return n
	}
	return fallback
}

func paginate(w http.ResponseWriter, r *http.Request, total int, item func(i int) any) {
	page, per := queryInt(r, "page", 1), queryInt(r, "per_page", mockDefaultPerPage)
	start := min((page-1)*per, total)
	end := min(start+per, total)
	items := make([]any, 0, end-start)
	for i := start; i < end; i++ {
		items = append(items, item(i))
	}
	if end < total {
		next := *r.URL
		q := next.Query()
		q.Set("page", strconv.Itoa(page+1))
		next.RawQuery = q.Encode()
		w.Header().Set("Link", fmt.Sprintf("<http://%s%s>; rel=\"next\"", r.Host, next.RequestURI()))
	}
	respondJSON(w, items)
}

func mockUser(login string) map[string]any {
	return map[string]any{"login": login, "avatar_url": mockWebURL + "/" + login + ".png"}
}

func (l mockLabel) rest() map[string]any {
	return map[string]any{"name": l.Name, "color": l.Color, "description": l.Description}
}

func mockLabelsJSON(labels []mockLabel) []any {
	out := make([]any, len(labels))
	for i, x := range labels {
		out[i] = x.rest()
	}
	return out
}

func mockLabelObjects(names []string) []any {
	out := make([]any, len(names))
	for i, n := range names {
		out[i] = map[string]any{"name": n}
	}
	return out
}

func (ms mockMilestone) rest() map[string]any {
	out := map[string]any{"number": ms.Number, "title": ms.Title, "description": ms.Description, "state": "open", "open_issues": ms.Open, "closed_issues": ms.Closed, "due_on": nil}
	if ms.DueDays > 0 {
		out["due_on"] = mockEpoch.AddDate(0, 0, ms.DueDays).Format(mockTimeFormat)
	}
	return out
}

func (f mockFile) rest() map[string]any {
	out := map[string]any{"filename": f.Path, "status": f.Status, "additions": f.Additions, "deletions": f.Deletions, "patch": f.Patch}
	if f.Prev != "" {
		out["previous_filename"] = f.Prev
	}
	return out
}

func (c mockCommit) rest(pr, i int) map[string]any {
	return map[string]any{"sha": mockSHA(pr*1000 + i), "html_url": fmt.Sprintf("%s/commit/%d", mockWebURL, i), "author": mockUser(c.Author),
		"commit": map[string]any{"message": c.Message, "author": map[string]any{"name": c.Author, "date": mockAt(c.Hours).Format(mockTimeFormat)}}}
}

func (rv mockReview) rest(repo *mockRepo, pr int) map[string]any {
	return map[string]any{"node_id": fmt.Sprintf("PRR_%d", rv.ID), "id": rv.ID, "user": mockUser(rv.Author), "body": rv.Body, "state": rv.State, "commit_id": mockSHA(pr),
		"submitted_at": mockAt(rv.Hours).Format(mockTimeFormat), "html_url": fmt.Sprintf("%s/%s/pull/%d#pullrequestreview-%d", mockWebURL, repo.key(), pr, rv.ID)}
}

func (c mockComment) rest(repo *mockRepo, number int) map[string]any {
	return map[string]any{"node_id": fmt.Sprintf("IC_%d", c.ID), "id": c.ID, "user": mockUser(c.Author), "body": c.Body, "created_at": mockAt(c.Hours).Format(mockTimeFormat),
		"html_url": fmt.Sprintf("%s/%s/issues/%d#issuecomment-%d", mockWebURL, repo.key(), number, c.ID)}
}

func (r *mockRepo) prJSON(req *http.Request, p *mockPR, detail bool) map[string]any {
	adds, dels := 0, 0
	for _, f := range p.Files {
		adds, dels = adds+f.Additions, dels+f.Deletions
	}
	reviewers := make([]any, len(p.Reviewers))
	for i, u := range p.Reviewers {
		reviewers[i] = mockUser(u)
	}
	out := map[string]any{
		"node_id": fmt.Sprintf("PR_%d", p.Number), "number": p.Number, "title": p.Title, "body": p.Body, "state": p.State, "draft": p.Draft,
		"html_url": fmt.Sprintf("%s/%s/pull/%d", mockWebURL, r.key(), p.Number), "user": mockUser(p.Author), "labels": mockLabelObjects(p.Labels),
		"requested_reviewers": reviewers, "created_at": mockAt(p.Hours + 1).Format(mockTimeFormat), "updated_at": mockAt(p.Hours).Format(mockTimeFormat),
		"head": map[string]any{"ref": fmt.Sprintf("feature/pr-%d", p.Number), "sha": mockSHA(p.Number), "user": mockUser(p.Author), "repo": map[string]any{"name": r.Name, "owner": mockUser(r.Owner)}},
		"base": map[string]any{"ref": mockBranch, "sha": mockSHA(0)},
	}
	if detail {
		out["additions"], out["deletions"], out["changed_files"], out["comments"] = adds, dels, len(p.Files), len(p.Comments)
		out["mergeable"], out["merged"] = true, false
	}
	return out
}

func (r *mockRepo) listPulls(w http.ResponseWriter, req *http.Request) {
	state := req.URL.Query().Get("state")
	var matches []*mockPR
	for i := range r.PRs {
		if p := &r.PRs[i]; state == "all" || state == "" && p.State == "open" || p.State == state {
			matches = append(matches, p)
		}
	}
	if req.URL.Query().Get("head") != "" {
		matches = nil
	}
	paginate(w, req, len(matches), func(i int) any { return r.prJSON(req, matches[i], false) })
}

func (r *mockRepo) issueComments(w http.ResponseWriter, req *http.Request, raw string) {
	number, _ := strconv.Atoi(raw)
	var comments []mockComment
	if p, ok := r.pr(number); ok {
		comments = p.Comments
	} else if i, ok := r.issue(number); ok {
		comments = i.Comments
	}
	paginate(w, req, len(comments), func(i int) any { return comments[i].rest(r, number) })
}

func (r *mockRepo) commitChecks(w http.ResponseWriter, req *http.Request, sha, kind string) {
	var pr *mockPR
	for i := range r.PRs {
		if mockSHA(r.PRs[i].Number) == sha {
			pr = &r.PRs[i]
		}
	}
	if pr == nil {
		mockError(w, http.StatusNotFound, "Not Found")
		return
	}
	if kind == "status" {
		statuses := make([]any, len(pr.Statuses))
		for i, s := range pr.Statuses {
			statuses[i] = map[string]any{"context": s.Context, "state": s.State, "target_url": mockWebURL + "/status/" + strconv.Itoa(i)}
		}
		respondJSON(w, map[string]any{"state": "pending", "statuses": statuses})
		return
	}
	runs := make([]any, len(pr.Checks))
	for i, c := range pr.Checks {
		run := map[string]any{"name": c.Name, "status": c.Status, "html_url": fmt.Sprintf("%s/runs/%d", mockWebURL, mockCheckIDBase+i), "conclusion": nil}
		if c.Conclusion != "" {
			run["conclusion"] = c.Conclusion
		}
		runs[i] = run
	}
	respondJSON(w, map[string]any{"total_count": len(runs), "check_runs": runs})
}

func (r *mockRepo) notifications(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("X-Poll-Interval", mockPollSeconds)
	out := make([]any, len(r.Notifications))
	for i, n := range r.Notifications {
		out[i] = map[string]any{"id": n.ID, "unread": n.Unread, "reason": n.Reason, "updated_at": mockAt(n.Hours).Format(mockTimeFormat),
			"subject": map[string]any{"title": n.Title, "type": "PullRequest", "url": fmt.Sprintf("%s/repos/%s/pulls/%d", mockAPIURL, r.key(), n.Number)}}
	}
	respondJSON(w, out)
}

func mockQualifiers(q string) (terms map[string][]string) {
	terms = map[string][]string{}
	for _, f := range strings.Fields(q) {
		if k, v, ok := strings.Cut(f, ":"); ok {
			terms[k] = append(terms[k], v)
		}
	}
	return terms
}

func mockMe(v string) string {
	if v == mockMeQualifier {
		return mockViewer
	}
	return v
}

func (m *githubMock) search(w http.ResponseWriter, r *http.Request) {
	terms := mockQualifiers(r.URL.Query().Get("q"))
	repo, ok := m.repos[strings.Join(terms["repo"], "")]
	if !ok {
		respondJSON(w, map[string]any{"total_count": 0, "items": []any{}})
		return
	}
	wantPR := slices.Contains(terms["is"], "pr")
	state := "open"
	if slices.Contains(terms["is"], "closed") || slices.Contains(terms["state"], "closed") {
		state = "closed"
	}
	var items []map[string]any
	if wantPR {
		for i := range repo.PRs {
			p := &repo.PRs[i]
			if p.State != state || !matchAny(terms["author"], func(v string) bool { return p.Author == mockMe(v) }) ||
				!matchAny(terms["review-requested"], func(v string) bool { return slices.Contains(p.Reviewers, mockMe(v)) }) || len(terms["assignee"]) > 0 {
				continue
			}
			item := repo.prJSON(r, p, false)
			item["comments"], item["pull_request"] = len(p.Comments), map[string]any{"merged_at": nil}
			items = append(items, item)
		}
	} else {
		for i := range repo.Issues {
			if is := &repo.Issues[i]; is.State == state && matchAny(terms["author"], func(v string) bool { return is.Author == mockMe(v) }) &&
				matchAny(terms["assignee"], func(v string) bool { return slices.Contains(is.Assignees, mockMe(v)) }) {
				items = append(items, repo.issueJSON(is))
			}
		}
	}
	page, per := queryInt(r, "page", 1), queryInt(r, "per_page", mockDefaultPerPage)
	start := min((page-1)*per, len(items))
	end := min(start+per, len(items))
	respondJSON(w, map[string]any{"total_count": len(items), "items": items[start:end]})
}

func matchAny(wanted []string, ok func(v string) bool) bool {
	for _, v := range wanted {
		if !ok(v) {
			return false
		}
	}
	return true
}

func (r *mockRepo) issueJSON(is *mockIssue) map[string]any {
	assignees := make([]any, len(is.Assignees))
	for i, u := range is.Assignees {
		assignees[i] = mockUser(u)
	}
	out := map[string]any{
		"node_id": fmt.Sprintf("I_%d", is.Number), "number": is.Number, "title": is.Title, "body": is.Body, "state": is.State,
		"html_url": fmt.Sprintf("%s/%s/issues/%d", mockWebURL, r.key(), is.Number), "repository_url": fmt.Sprintf("%s/repos/%s", mockAPIURL, r.key()),
		"user": mockUser(is.Author), "labels": mockLabelsJSON(is.Labels), "assignees": assignees, "comments": len(is.Comments), "milestone": nil,
		"created_at": mockAt(is.Hours + 1).Format(mockTimeFormat), "updated_at": mockAt(is.Hours).Format(mockTimeFormat),
	}
	if is.Milestone > 0 {
		out["milestone"] = r.Milestones[is.Milestone-1].rest()
	}
	return out
}
