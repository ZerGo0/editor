package github

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const issueRESTFixture = `{
  "node_id": "I_node", "number": 5, "title": "Crash on start", "body": null, "state": "open",
  "html_url": "https://github.com/octo/repo/issues/5",
  "repository_url": "https://api.github.com/repos/octo/repo",
  "user": {"login": "alice"},
  "labels": [{"name": "bug", "color": "d73a4a", "description": "Broken"}],
  "assignees": [{"login": "bob"}, {"login": "carol"}],
  "milestone": {"number": 2, "title": "v1", "description": "", "state": "open", "due_on": "2026-03-01T00:00:00Z", "open_issues": 3, "closed_issues": 4},
  "comments": 6, "created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-01-03T03:04:05Z", "unknown": 1
}`

func TestSearchIssuesBuildsQueryAndFiltersPulls(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/issues" {
			t.Errorf("path %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "assignee:@me is:issue" || q.Get("page") != "2" || q.Get("per_page") != "30" {
			t.Errorf("query %v", q)
		}
		io.WriteString(w, `{"total_count": 80, "items": [`+issueRESTFixture+`, {"number": 6, "state": "open", "pull_request": {}}]}`)
	})
	page, err := c.SearchIssues(context.Background(), " assignee:@me ", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Issues) != 1 || page.Total != 80 || page.HasMore {
		t.Fatalf("%+v", page)
	}
	issue := page.Issues[0]
	if issue.Number != 5 || issue.Body != "" || issue.State != StateOpen || issue.Author != "alice" || issue.Comments != 6 {
		t.Fatalf("%+v", issue)
	}
	if issue.Repo != (Remote{Host: "github.com", Owner: "octo", Name: "repo"}) {
		t.Fatalf("repo %+v", issue.Repo)
	}
	if !reflect.DeepEqual(issue.Assignees, []string{"bob", "carol"}) || len(issue.Labels) != 1 || issue.Labels[0] != (Label{Name: "bug", Color: "d73a4a", Description: "Broken"}) {
		t.Fatalf("%+v", issue)
	}
	if issue.Milestone == nil || issue.Milestone.Number != 2 || issue.Milestone.Due.Year() != 2026 || issue.Milestone.OpenIssues != 3 {
		t.Fatalf("%+v", issue.Milestone)
	}
}

func TestSearchIssuesKeepsExplicitQualifierAndPaging(t *testing.T) {
	items := strings.TrimSuffix(strings.Repeat(`{"number": 1, "state": "open"},`, defaultPerPage), ",")
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "is:issue repo:octo/repo" {
			t.Errorf("q %q", got)
		}
		io.WriteString(w, `{"total_count": 31, "items": [`+items+`]}`)
	})
	page, err := c.SearchIssues(context.Background(), "is:issue repo:octo/repo", 0)
	if err != nil || !page.HasMore || len(page.Issues) != defaultPerPage {
		t.Fatalf("%v %+v", err, page)
	}
}

func TestSearchIssuesErrors(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) })
	if _, err := c.SearchIssues(context.Background(), "  ", 1); err == nil {
		t.Fatal("want error for empty query")
	}
	if _, err := c.SearchIssues(context.Background(), "x", 1); err == nil {
		t.Fatal("want error for missing items")
	}
	bad, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"total_count": 1, "items": [{"number": 0, "state": ""}]}`)
	})
	if _, err := bad.SearchIssues(context.Background(), "x", 1); err == nil {
		t.Fatal("want error for an item without number")
	}
}

func TestGetIssueGraphQL(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		vars := readJSON(t, r)["variables"].(map[string]any)
		if r.URL.Path != "/graphql" || vars["owner"] != "octo" || vars["name"] != "repo" || vars["number"] != float64(5) {
			t.Errorf("%s %v", r.URL.Path, vars)
		}
		io.WriteString(w, `{"data": {"repository": {"issue": {
		  "id": "I_1", "number": 5, "title": "Crash", "body": "It crashes", "state": "CLOSED", "url": "https://github.com/octo/repo/issues/5",
		  "createdAt": "2026-01-02T03:04:05Z", "updatedAt": "2026-01-03T03:04:05Z", "author": null,
		  "labels": {"nodes": [{"name": "bug", "color": "ff0000", "description": null}]},
		  "assignees": {"nodes": [{"login": "bob"}]},
		  "milestone": {"number": 3, "title": "v2", "description": "d", "state": "OPEN", "dueOn": null},
		  "comments": {"totalCount": 4},
		  "closedByPullRequestsReferences": {"nodes": [{"number": 9, "title": "Fix", "url": "https://github.com/octo/repo/pull/9", "state": "MERGED"}]}
		}}}}`)
	})
	issue, err := c.GetIssue(context.Background(), testRepo, 5)
	if err != nil {
		t.Fatal(err)
	}
	if issue.NodeID != "I_1" || issue.State != StateClosed || issue.Author != "" || issue.Comments != 4 || issue.Repo != testRepo {
		t.Fatalf("%+v", issue)
	}
	if issue.Milestone == nil || issue.Milestone.Title != "v2" || issue.Milestone.State != StateOpen || !issue.Milestone.Due.IsZero() {
		t.Fatalf("%+v", issue.Milestone)
	}
	if want := []LinkedPR{{Number: 9, Title: "Fix", URL: "https://github.com/octo/repo/pull/9", State: "merged"}}; !reflect.DeepEqual(issue.LinkedPRs, want) {
		t.Fatalf("%+v", issue.LinkedPRs)
	}
	if !reflect.DeepEqual(issue.Assignees, []string{"bob"}) || len(issue.Labels) != 1 || issue.Labels[0].Name != "bug" {
		t.Fatalf("%+v", issue)
	}
}

func TestGetIssueErrors(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": {"repository": {"issue": null}}}`)
	})
	if _, err := c.GetIssue(context.Background(), testRepo, 5); err == nil {
		t.Fatal("want not found error")
	}
	if _, err := c.GetIssue(context.Background(), testRepo, 0); err == nil {
		t.Fatal("want error for bad number")
	}
	if _, err := c.GetIssue(context.Background(), Remote{}, 1); err == nil {
		t.Fatal("want error for empty repo")
	}
}

func TestIssueCommentsAndAdd(t *testing.T) {
	var posted map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/octo/repo/issues/5/comments" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Method == http.MethodPost {
			posted = readJSON(t, r)
			io.WriteString(w, `{"node_id": "IC_2", "id": 22, "user": {"login": "me"}, "body": "hi", "created_at": "2026-01-01T00:00:00Z", "html_url": "u"}`)
			return
		}
		io.WriteString(w, `[{"node_id": "IC_1", "id": 11, "user": {"login": "bob"}, "body": "a", "created_at": "2026-01-01T00:00:00Z", "html_url": "u"}]`)
	})
	comments, err := c.IssueComments(context.Background(), testRepo, 5)
	if err != nil || len(comments) != 1 || comments[0].Author != "bob" {
		t.Fatalf("%v %+v", err, comments)
	}
	comment, err := c.AddIssueComment(context.Background(), testRepo, 5, "hi")
	if err != nil || comment.DatabaseID != 22 || posted["body"] != "hi" {
		t.Fatalf("%v %+v %v", err, comment, posted)
	}
	if _, err := c.AddIssueComment(context.Background(), testRepo, 5, ""); err == nil {
		t.Fatal("want error for empty body")
	}
	if _, err := c.IssueComments(context.Background(), testRepo, -1); err == nil {
		t.Fatal("want error for bad number")
	}
}

func recordIssueRequest(t *testing.T, got *map[string]any, method, path string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method || r.URL.Path != path {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		*got = readJSON(t, r)
		io.WriteString(w, issueRESTFixture)
	}
}

func TestCreateIssueBody(t *testing.T) {
	var got map[string]any
	c, _ := newTestClient(t, recordIssueRequest(t, &got, http.MethodPost, "/repos/octo/repo/issues"))
	issue, err := c.CreateIssue(context.Background(), testRepo, IssueInput{Title: "T", Body: "B", Labels: []string{"bug"}, Assignees: []string{"bob"}, Milestone: 2})
	if err != nil || issue.Number != 5 {
		t.Fatalf("%v %+v", err, issue)
	}
	want := map[string]any{"title": "T", "body": "B", "labels": []any{"bug"}, "assignees": []any{"bob"}, "milestone": float64(2)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
	if _, err := c.CreateIssue(context.Background(), testRepo, IssueInput{Title: " "}); err == nil {
		t.Fatal("want error for blank title")
	}
	if _, err := c.CreateIssue(context.Background(), testRepo, IssueInput{Title: "T", Milestone: -1}); err == nil {
		t.Fatal("want error for negative milestone")
	}
	if _, err := c.CreateIssue(context.Background(), testRepo, IssueInput{Title: "T"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["body"]; ok || got["title"] != "T" || len(got) != 1 {
		t.Fatalf("minimal body %v", got)
	}
}

func TestEditIssueBody(t *testing.T) {
	var got map[string]any
	c, _ := newTestClient(t, recordIssueRequest(t, &got, http.MethodPatch, "/repos/octo/repo/issues/5"))
	title, state, noMilestone := "New", StateClosed, 0
	labels, assignees := []string(nil), []string{"bob"}
	_, err := c.EditIssue(context.Background(), testRepo, 5, IssueEdit{Title: &title, State: &state, Labels: &labels, Assignees: &assignees, Milestone: &noMilestone})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"title": "New", "state": "closed", "labels": []any{}, "assignees": []any{"bob"}, "milestone": nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
	milestone := 4
	if _, err := c.EditIssue(context.Background(), testRepo, 5, IssueEdit{Milestone: &milestone}); err != nil || got["milestone"] != float64(4) || len(got) != 1 {
		t.Fatalf("%v %v", err, got)
	}
}

func TestEditIssueValidation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("request sent") })
	blank, bad, negative := " ", "merged", -1
	cases := map[string]IssueEdit{
		"empty":     {},
		"title":     {Title: &blank},
		"state":     {State: &bad},
		"milestone": {Milestone: &negative},
	}
	for name, edit := range cases {
		if _, err := c.EditIssue(context.Background(), testRepo, 5, edit); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	title := "x"
	if _, err := c.EditIssue(context.Background(), testRepo, 0, IssueEdit{Title: &title}); err == nil {
		t.Error("want error for bad number")
	}
}

func TestMilestonesAndLabels(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/octo/repo/milestones":
			if r.URL.Query().Get("state") != "open" {
				t.Errorf("state %q", r.URL.Query().Get("state"))
			}
			io.WriteString(w, `[{"number": 1, "title": "v1", "state": "open", "due_on": null, "open_issues": 1, "closed_issues": 2}]`)
		case "/repos/octo/repo/labels":
			io.WriteString(w, `[{"name": "bug", "color": "f00", "description": null}]`)
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	})
	milestones, err := c.Milestones(context.Background(), testRepo)
	if err != nil || len(milestones) != 1 || milestones[0].Title != "v1" || milestones[0].ClosedIssues != 2 {
		t.Fatalf("%v %+v", err, milestones)
	}
	labels, err := c.Labels(context.Background(), testRepo)
	if err != nil || len(labels) != 1 || labels[0].Name != "bug" {
		t.Fatalf("%v %+v", err, labels)
	}
	bad, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `[{"color": "f00"}]`) })
	if _, err := bad.Labels(context.Background(), testRepo); err == nil {
		t.Fatal("want error for a label without name")
	}
	if _, err := bad.Milestones(context.Background(), testRepo); err == nil {
		t.Fatal("want error for a milestone without number")
	}
}

func TestUserPages(t *testing.T) {
	var fields []string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		query := body["query"].(string)
		vars := body["variables"].(map[string]any)
		field := "assignableUsers"
		if strings.Contains(query, "mentionableUsers(") {
			field = "mentionableUsers"
		}
		fields = append(fields, field)
		if vars["after"] == nil {
			io.WriteString(w, `{"data": {"repository": {"`+field+`": {"pageInfo": {"hasNextPage": true, "endCursor": "C1"}, "nodes": [{"login": "alice", "name": "Alice", "avatarUrl": "a"}]}}}}`)
			return
		}
		if vars["after"] != "C1" || vars["first"] != float64(userPageSize) {
			t.Errorf("vars %v", vars)
		}
		io.WriteString(w, `{"data": {"repository": {"`+field+`": {"pageInfo": {"hasNextPage": false, "endCursor": null}, "nodes": [{"login": "bob", "name": null, "avatarUrl": ""}]}}}}`)
	})
	first, err := c.AssignableUsers(context.Background(), testRepo, "")
	if err != nil || first.Next != "C1" || len(first.Users) != 1 || first.Users[0] != (User{Login: "alice", Name: "Alice", AvatarURL: "a"}) {
		t.Fatalf("%v %+v", err, first)
	}
	second, err := c.AssignableUsers(context.Background(), testRepo, first.Next)
	if err != nil || second.Next != "" || len(second.Users) != 1 || second.Users[0].Login != "bob" {
		t.Fatalf("%v %+v", err, second)
	}
	if _, err := c.MentionableUsers(context.Background(), testRepo, ""); err != nil {
		t.Fatal(err)
	}
	if want := []string{"assignableUsers", "assignableUsers", "mentionableUsers"}; !reflect.DeepEqual(fields, want) {
		t.Fatalf("%v", fields)
	}
}

func TestUserPageErrors(t *testing.T) {
	missing, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": {"repository": null}}`)
	})
	if _, err := missing.AssignableUsers(context.Background(), testRepo, ""); err == nil {
		t.Fatal("want error for a missing repository")
	}
	noLogin, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": {"repository": {"assignableUsers": {"pageInfo": {"hasNextPage": false}, "nodes": [{"name": "x"}]}}}}`)
	})
	if _, err := noLogin.AssignableUsers(context.Background(), testRepo, ""); err == nil {
		t.Fatal("want error for a user without login")
	}
	noCursor, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": {"repository": {"assignableUsers": {"pageInfo": {"hasNextPage": true}, "nodes": []}}}}`)
	})
	if _, err := noCursor.AssignableUsers(context.Background(), testRepo, ""); err == nil {
		t.Fatal("want error for a page without cursor")
	}
}

func TestViewer(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("path %s", r.URL.Path)
		}
		io.WriteString(w, `{"login": "me", "name": "Me", "avatar_url": "a"}`)
	})
	user, err := c.Viewer(context.Background())
	if err != nil || user != (User{Login: "me", Name: "Me", AvatarURL: "a"}) {
		t.Fatalf("%v %+v", err, user)
	}
	empty, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) })
	if _, err := empty.Viewer(context.Background()); err == nil {
		t.Fatal("want error for missing login")
	}
}

func TestCreateLinkedBranch(t *testing.T) {
	var mutation map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		if strings.Contains(body["query"].(string), "createLinkedBranch") {
			mutation = body["variables"].(map[string]any)
			io.WriteString(w, `{"data": {"createLinkedBranch": {"linkedBranch": {"id": "LB_1", "ref": {"name": "me/issue5"}}}}}`)
			return
		}
		io.WriteString(w, `{"data": {"repository": {"defaultBranchRef": {"target": {"oid": "def456"}}, "issue": {"id": "I_5"}}}}`)
	})
	branch, err := c.CreateLinkedBranch(context.Background(), testRepo, 5, "me/issue5", "")
	if err != nil || branch != (LinkedBranch{ID: "LB_1", Name: "me/issue5"}) {
		t.Fatalf("%v %+v", err, branch)
	}
	if mutation["issue"] != "I_5" || mutation["oid"] != "def456" || mutation["name"] != "me/issue5" {
		t.Fatalf("%v", mutation)
	}
	if _, err := c.CreateLinkedBranch(context.Background(), testRepo, 5, "me/issue5", "abc"); err != nil || mutation["oid"] != "abc" {
		t.Fatalf("%v %v", err, mutation)
	}
	for _, name := range []string{"", "-x", "a b"} {
		if _, err := c.CreateLinkedBranch(context.Background(), testRepo, 5, name, ""); err == nil {
			t.Errorf("want error for name %q", name)
		}
	}
	if _, err := c.CreateLinkedBranch(context.Background(), testRepo, 0, "x", ""); err == nil {
		t.Error("want error for bad number")
	}
}

func TestCreateLinkedBranchFailures(t *testing.T) {
	noIssue, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": {"repository": {"defaultBranchRef": null, "issue": null}}}`)
	})
	if _, err := noIssue.CreateLinkedBranch(context.Background(), testRepo, 5, "x", ""); err == nil {
		t.Fatal("want error for a missing issue")
	}
	noBase, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data": {"repository": {"defaultBranchRef": null, "issue": {"id": "I_5"}}}}`)
	})
	if _, err := noBase.CreateLinkedBranch(context.Background(), testRepo, 5, "x", ""); err == nil {
		t.Fatal("want error without a base commit")
	}
	noBranch, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(readJSON(t, r)["query"].(string), "createLinkedBranch") {
			io.WriteString(w, `{"data": {"createLinkedBranch": {"linkedBranch": null}}}`)
			return
		}
		io.WriteString(w, `{"data": {"repository": {"defaultBranchRef": {"target": {"oid": "d"}}, "issue": {"id": "I_5"}}}}`)
	})
	if _, err := noBranch.CreateLinkedBranch(context.Background(), testRepo, 5, "x", ""); err == nil {
		t.Fatal("want error for a response without branch")
	}
}
