//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type mockGraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func (m *githubMock) graphql(w http.ResponseWriter, r *http.Request) {
	var req mockGraphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mockError(w, http.StatusBadRequest, "invalid body")
		return
	}
	owner, _ := req.Variables["owner"].(string)
	name, _ := req.Variables["name"].(string)
	repo := m.repos[owner+"/"+name]
	number := 0
	if n, ok := req.Variables["number"].(float64); ok {
		number = int(n)
	}
	var data any
	switch q := req.Query; {
	case repo == nil && strings.Contains(q, "viewer{login}"):
		data = map[string]any{"viewer": map[string]any{"login": mockViewer}, "repository": nil}
	case repo == nil:
		data = map[string]any{"repository": nil}
	case strings.Contains(q, "mergeQueue(branch"):
		data = repo.queueData(number)
	case strings.Contains(q, "statusCheckRollup"):
		data = repo.overviewData(number)
	case strings.Contains(q, "timelineItems(first"):
		data = repo.timelineData(number)
	case strings.Contains(q, "viewerViewedState"):
		data = repo.viewedData(number)
	case strings.HasPrefix(q, "query") && strings.Contains(q, "reviewThreads(first"):
		data = repo.threadsData(number)
	case strings.Contains(q, "closedByPullRequestsReferences"):
		data = repo.issueData(number)
	case strings.Contains(q, "assignableUsers(") || strings.Contains(q, "mentionableUsers("):
		data = repo.usersData(q)
	case strings.Contains(q, "pullRequests(first:1,states"):
		data = localBranchData(req.Variables)
	default:
		m.miss(w, r, "graphql "+firstLine(req.Query))
		respondJSON(w, map[string]any{"errors": []map[string]string{{"message": "the mock does not serve this query"}}})
		return
	}
	respondJSON(w, map[string]any{"data": data})
}

func firstLine(q string) string {
	line, _, _ := strings.Cut(q, "\n")
	return line
}

func localBranchData(vars map[string]any) map[string]any {
	branches := map[string]any{}
	for k := range vars {
		if strings.HasPrefix(k, "b") {
			branches[k] = map[string]any{"nodes": []any{}}
		}
	}
	return map[string]any{"repository": branches}
}

func mockNodes(nodes []any) map[string]any {
	return map[string]any{"nodes": nodes}
}

func mockPageInfo() map[string]any {
	return map[string]any{"hasNextPage": false, "endCursor": ""}
}

func (r *mockRepo) usersData(query string) map[string]any {
	field := "assignableUsers"
	if strings.Contains(query, "mentionableUsers(") {
		field = "mentionableUsers"
	}
	nodes := make([]any, len(r.Users))
	for i, u := range r.Users {
		nodes[i] = map[string]any{"login": u, "name": strings.ToUpper(u[:1]) + u[1:], "avatarUrl": mockWebURL + "/" + u + ".png"}
	}
	return map[string]any{"repository": map[string]any{field: map[string]any{"pageInfo": mockPageInfo(), "nodes": nodes}}}
}

func (r *mockRepo) issueData(number int) map[string]any {
	is, ok := r.issue(number)
	if !ok {
		return map[string]any{"repository": map[string]any{"issue": nil}}
	}
	labels := make([]any, len(is.Labels))
	for i, l := range is.Labels {
		labels[i] = l.rest()
	}
	assignees := make([]any, len(is.Assignees))
	for i, u := range is.Assignees {
		assignees[i] = map[string]any{"login": u}
	}
	issue := map[string]any{
		"id": fmt.Sprintf("I_%d", is.Number), "number": is.Number, "title": is.Title, "body": is.Body, "state": strings.ToUpper(is.State),
		"url": fmt.Sprintf("%s/%s/issues/%d", mockWebURL, r.key(), is.Number), "createdAt": mockAt(is.Hours + 1).Format(mockTimeFormat), "updatedAt": mockAt(is.Hours).Format(mockTimeFormat),
		"author": map[string]any{"login": is.Author}, "labels": mockNodes(labels), "assignees": mockNodes(assignees), "milestone": nil,
		"comments":                       map[string]any{"totalCount": len(is.Comments)},
		"closedByPullRequestsReferences": mockNodes([]any{}),
	}
	if is.Milestone > 0 {
		ms := r.Milestones[is.Milestone-1]
		issue["milestone"] = map[string]any{"number": ms.Number, "title": ms.Title, "description": ms.Description, "state": "OPEN", "dueOn": mockEpoch.AddDate(0, 0, ms.DueDays).Format(mockTimeFormat)}
	}
	return map[string]any{"repository": map[string]any{"issue": issue}}
}

func (r *mockRepo) prData(number int, fields map[string]any) map[string]any {
	if _, ok := r.pr(number); !ok {
		return map[string]any{"repository": map[string]any{"pullRequest": nil}}
	}
	return map[string]any{"viewer": map[string]any{"login": mockViewer}, "repository": fields}
}

func (r *mockRepo) queueData(number int) map[string]any {
	return r.prData(number, map[string]any{"mergeQueue": nil, "pullRequest": map[string]any{
		"viewerCanEnableAutoMerge": true, "viewerCanDisableAutoMerge": false, "isInMergeQueue": false, "autoMergeRequest": nil, "mergeQueueEntry": nil,
	}})
}

func (r *mockRepo) overviewData(number int) map[string]any {
	pr, ok := r.pr(number)
	if !ok {
		return r.prData(number, nil)
	}
	contexts := make([]any, 0, len(pr.Checks)+len(pr.Statuses))
	for i, c := range pr.Checks {
		node := map[string]any{"__typename": "CheckRun", "name": c.Name, "status": strings.ToUpper(c.Status), "conclusion": nil, "detailsUrl": fmt.Sprintf("%s/runs/%d", mockWebURL, mockCheckIDBase+i),
			"isRequired": c.Required, "databaseId": mockCheckIDBase + i, "checkSuite": map[string]any{"workflowRun": map[string]any{"databaseId": mockCheckIDBase}}}
		if c.Conclusion != "" {
			node["conclusion"] = strings.ToUpper(c.Conclusion)
		}
		contexts = append(contexts, node)
	}
	for i, s := range pr.Statuses {
		contexts = append(contexts, map[string]any{"__typename": "StatusContext", "context": s.Context, "state": strings.ToUpper(s.State), "targetUrl": mockWebURL + "/status/" + fmt.Sprint(i), "isRequired": false})
	}
	assignees := make([]any, len(pr.Reviewers))
	for i, u := range pr.Reviewers {
		assignees[i] = map[string]any{"login": u}
	}
	return r.prData(number, map[string]any{
		"mergeCommitAllowed": true, "squashMergeAllowed": true, "rebaseMergeAllowed": true, "deleteBranchOnMerge": false, "viewerDefaultMergeMethod": "SQUASH",
		"pullRequest": map[string]any{
			"id": fmt.Sprintf("PR_%d", pr.Number), "mergeable": "MERGEABLE", "mergeStateStatus": "BLOCKED", "reviewDecision": "CHANGES_REQUESTED",
			"baseRefOid": mockSHA(0), "headRefOid": mockSHA(pr.Number), "maintainerCanModify": true, "viewerCanUpdate": true,
			"assignees": mockNodes(assignees), "milestone": nil, "closingIssuesReferences": mockNodes([]any{}),
			"commits": mockNodes([]any{map[string]any{"commit": map[string]any{"statusCheckRollup": map[string]any{"contexts": mockNodes(contexts)}}}}),
		},
	})
}

func (r *mockRepo) timelineData(number int) map[string]any {
	pr, ok := r.pr(number)
	if !ok {
		return r.prData(number, nil)
	}
	var nodes []any
	for _, c := range pr.Comments {
		nodes = append(nodes, map[string]any{"__typename": "IssueComment", "id": fmt.Sprintf("IC_%d", c.ID), "databaseId": c.ID, "url": fmt.Sprintf("%s/%s/pull/%d#issuecomment-%d", mockWebURL, r.key(), pr.Number, c.ID),
			"body": c.Body, "createdAt": mockAt(c.Hours).Format(mockTimeFormat), "viewerCanUpdate": false, "viewerCanDelete": false, "author": map[string]any{"login": c.Author}, "reactionGroups": []any{}})
	}
	for _, rv := range pr.Reviews {
		nodes = append(nodes, map[string]any{"__typename": "PullRequestReview", "id": fmt.Sprintf("PRR_%d", rv.ID), "databaseId": rv.ID, "url": fmt.Sprintf("%s/%s/pull/%d#pullrequestreview-%d", mockWebURL, r.key(), pr.Number, rv.ID),
			"state": rv.State, "body": rv.Body, "submittedAt": mockAt(rv.Hours).Format(mockTimeFormat), "author": map[string]any{"login": rv.Author}})
	}
	for _, label := range pr.Labels {
		nodes = append(nodes, map[string]any{"__typename": "LabeledEvent", "createdAt": mockAt(pr.Hours + 1).Format(mockTimeFormat), "actor": map[string]any{"login": pr.Author}, "label": map[string]any{"name": label}})
	}
	return r.prData(number, map[string]any{"pullRequest": map[string]any{"timelineItems": map[string]any{"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""}, "nodes": nodes}}})
}

func (r *mockRepo) viewedData(number int) map[string]any {
	pr, ok := r.pr(number)
	if !ok {
		return r.prData(number, nil)
	}
	nodes := make([]any, len(pr.Files))
	for i, f := range pr.Files {
		state := "UNVIEWED"
		if i%3 == 0 {
			state = "VIEWED"
		}
		nodes[i] = map[string]any{"path": f.Path, "viewerViewedState": state}
	}
	return r.prData(number, map[string]any{"pullRequest": map[string]any{"files": map[string]any{"pageInfo": mockPageInfo(), "nodes": nodes}}})
}

func (r *mockRepo) threadsData(number int) map[string]any {
	pr, ok := r.pr(number)
	if !ok {
		return r.prData(number, nil)
	}
	nodes := make([]any, len(pr.Threads))
	for i, t := range pr.Threads {
		comments := make([]any, len(t.Comments))
		for j, c := range t.Comments {
			comments[j] = map[string]any{"id": fmt.Sprintf("PRRC_%d", c.ID), "databaseId": c.ID, "url": fmt.Sprintf("%s/%s/pull/%d#discussion_r%d", mockWebURL, r.key(), pr.Number, c.ID),
				"body": c.Body, "createdAt": mockAt(c.Hours).Format(mockTimeFormat), "author": map[string]any{"login": c.Author}}
		}
		nodes[i] = map[string]any{"id": t.ID, "isResolved": t.Resolved, "isOutdated": t.Outdated, "path": t.Path, "diffSide": "RIGHT", "line": t.Line, "startLine": nil, "originalLine": t.Line, "originalStartLine": nil, "comments": mockNodes(comments)}
	}
	return r.prData(number, map[string]any{"pullRequest": map[string]any{"reviewThreads": map[string]any{"pageInfo": mockPageInfo(), "nodes": nodes}}})
}
