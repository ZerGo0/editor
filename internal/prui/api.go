package prui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/github"
)

const (
	maxTimelinePages  = 5
	pageSizeGraphQL   = 100
	maxLocalBranchPRs = 30
	notificationsPage = 50
	minPollInterval   = 60 * time.Second
)

const overviewQuery = `query($owner:String!,$name:String!,$number:Int!){
viewer{login}
repository(owner:$owner,name:$name){
mergeCommitAllowed squashMergeAllowed rebaseMergeAllowed deleteBranchOnMerge viewerDefaultMergeMethod
pullRequest(number:$number){
id mergeable mergeStateStatus reviewDecision baseRefOid headRefOid maintainerCanModify viewerCanUpdate
assignees(first:20){nodes{login}}
milestone{number title}
closingIssuesReferences(first:10){nodes{number title url}}
commits(last:1){nodes{commit{statusCheckRollup{contexts(first:100){nodes{__typename
... on CheckRun{name status conclusion detailsUrl isRequired(pullRequestNumber:$number) databaseId checkSuite{workflowRun{databaseId}}}
... on StatusContext{context state targetUrl isRequired(pullRequestNumber:$number)}}}}}}}
}}}`

const queueQuery = `query($owner:String!,$name:String!,$number:Int!,$base:String!){
repository(owner:$owner,name:$name){
mergeQueue(branch:$base){id}
pullRequest(number:$number){
viewerCanEnableAutoMerge viewerCanDisableAutoMerge isInMergeQueue
autoMergeRequest{mergeMethod enabledBy{login}}
mergeQueueEntry{id state position}
}}}`

const timelineFragment = `__typename
... on IssueComment{id databaseId url body createdAt viewerCanUpdate viewerCanDelete author{login} reactionGroups{content viewerHasReacted users{totalCount}}}
... on PullRequestReview{id databaseId url state body submittedAt author{login}}
... on LabeledEvent{createdAt actor{login} label{name}}
... on UnlabeledEvent{createdAt actor{login} label{name}}
... on AssignedEvent{createdAt actor{login} assignee{... on User{login} ... on Bot{login}}}
... on UnassignedEvent{createdAt actor{login} assignee{... on User{login} ... on Bot{login}}}
... on ReviewRequestedEvent{createdAt actor{login} requestedReviewer{... on User{login} ... on Team{name}}}
... on ReviewRequestRemovedEvent{createdAt actor{login} requestedReviewer{... on User{login} ... on Team{name}}}
... on HeadRefForcePushedEvent{createdAt actor{login} beforeCommit{oid} afterCommit{oid}}
... on MergedEvent{createdAt actor{login} commit{oid}}
... on ClosedEvent{createdAt actor{login}}
... on ReopenedEvent{createdAt actor{login}}
... on ReadyForReviewEvent{createdAt actor{login}}
... on ConvertToDraftEvent{createdAt actor{login}}
... on AutoMergeEnabledEvent{createdAt enabler{login}}
... on AutoMergeDisabledEvent{createdAt disabler{login}}
... on RenamedTitleEvent{createdAt actor{login} previousTitle currentTitle}
... on HeadRefDeletedEvent{createdAt actor{login} headRefName}`

const timelineQuery = `query($owner:String!,$name:String!,$number:Int!,$after:String){repository(owner:$owner,name:$name){pullRequest(number:$number){timelineItems(first:100,after:$after){pageInfo{hasNextPage endCursor} nodes{` + timelineFragment + `}}}}}`

const viewedQuery = `query($owner:String!,$name:String!,$number:Int!,$after:String){repository(owner:$owner,name:$name){pullRequest(number:$number){files(first:100,after:$after){pageInfo{hasNextPage endCursor} nodes{path viewerViewedState}}}}}`

const (
	markViewedMutation   = `mutation($id:ID!,$path:String!){markFileAsViewed(input:{pullRequestId:$id,path:$path}){clientMutationId}}`
	unmarkViewedMutation = `mutation($id:ID!,$path:String!){unmarkFileAsViewed(input:{pullRequestId:$id,path:$path}){clientMutationId}}`
	draftMutation        = `mutation($id:ID!){convertPullRequestToDraft(input:{pullRequestId:$id}){pullRequest{id}}}`
	enableAutoMutation   = `mutation($id:ID!,$method:PullRequestMergeMethod!){enablePullRequestAutoMerge(input:{pullRequestId:$id,mergeMethod:$method}){pullRequest{id}}}`
	disableAutoMutation  = `mutation($id:ID!){disablePullRequestAutoMerge(input:{pullRequestId:$id}){pullRequest{id}}}`
	enqueueMutation      = `mutation($id:ID!){enqueuePullRequest(input:{pullRequestId:$id}){clientMutationId}}`
	dequeueMutation      = `mutation($id:ID!){dequeuePullRequest(input:{id:$id}){clientMutationId}}`
	revertMutation       = `mutation($id:ID!){revertPullRequest(input:{pullRequestId:$id}){revertPullRequest{number}}}`
	addReactionMutation  = `mutation($id:ID!,$content:ReactionContent!){addReaction(input:{subjectId:$id,content:$content}){reaction{content}}}`
	delReactionMutation  = `mutation($id:ID!,$content:ReactionContent!){removeReaction(input:{subjectId:$id,content:$content}){reaction{content}}}`
	editIssueComment     = `mutation($id:ID!,$body:String!){updateIssueComment(input:{id:$id,body:$body}){issueComment{id}}}`
	deleteIssueComment   = `mutation($id:ID!){deleteIssueComment(input:{id:$id}){clientMutationId}}`
	editReviewComment    = `mutation($id:ID!,$body:String!){updatePullRequestReviewComment(input:{pullRequestReviewCommentId:$id,body:$body}){pullRequestReviewComment{id}}}`
	deleteReviewComment  = `mutation($id:ID!){deletePullRequestReviewComment(input:{id:$id}){clientMutationId}}`
	deleteReviewMutation = `mutation($id:ID!){deletePullRequestReview(input:{pullRequestReviewId:$id}){clientMutationId}}`
	fileThreadMutation   = `mutation($review:ID!,$path:String!,$body:String!){addPullRequestReviewThread(input:{pullRequestReviewId:$review,path:$path,body:$body,subjectType:FILE}){thread{id}}}`
)

type gqlLogin struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

func (r *restClient) Overview(ctx context.Context, repo github.Remote, number int, base string) (Overview, error) {
	if _, err := repoPath(repo); err != nil {
		return Overview{}, err
	}
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number}
	var data struct {
		Viewer     gqlLogin `json:"viewer"`
		Repository *struct {
			MergeCommitAllowed  bool   `json:"mergeCommitAllowed"`
			SquashMergeAllowed  bool   `json:"squashMergeAllowed"`
			RebaseMergeAllowed  bool   `json:"rebaseMergeAllowed"`
			DeleteBranchOnMerge bool   `json:"deleteBranchOnMerge"`
			DefaultMethod       string `json:"viewerDefaultMergeMethod"`
			PullRequest         *struct {
				ID                  string `json:"id"`
				Mergeable           string `json:"mergeable"`
				MergeState          string `json:"mergeStateStatus"`
				ReviewDecision      string `json:"reviewDecision"`
				BaseSHA             string `json:"baseRefOid"`
				HeadSHA             string `json:"headRefOid"`
				MaintainerCanModify bool   `json:"maintainerCanModify"`
				CanUpdate           bool   `json:"viewerCanUpdate"`
				Assignees           struct {
					Nodes []gqlLogin `json:"nodes"`
				} `json:"assignees"`
				Milestone *struct {
					Number int    `json:"number"`
					Title  string `json:"title"`
				} `json:"milestone"`
				Linked struct {
					Nodes []gqlLinkedIssue `json:"nodes"`
				} `json:"closingIssuesReferences"`
				Commits struct {
					Nodes []struct {
						Commit struct {
							Rollup *struct {
								Contexts struct {
									Nodes []gqlCheckContext `json:"nodes"`
								} `json:"contexts"`
							} `json:"statusCheckRollup"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	if err := r.gql(ctx, overviewQuery, vars, &data); err != nil {
		return Overview{}, err
	}
	repoData := data.Repository
	if repoData == nil || repoData.PullRequest == nil {
		return Overview{}, fmt.Errorf("prui: pull request %d not found", number)
	}
	pr := repoData.PullRequest
	o := Overview{
		ID: pr.ID, Mergeable: pr.Mergeable, MergeState: pr.MergeState, ReviewDecision: pr.ReviewDecision,
		BaseSHA: pr.BaseSHA, HeadSHA: pr.HeadSHA, Viewer: data.Viewer.Login, MaintainerCanModify: pr.MaintainerCanModify,
		AllowMerge: repoData.MergeCommitAllowed, AllowSquash: repoData.SquashMergeAllowed, AllowRebase: repoData.RebaseMergeAllowed,
		DeleteBranchOnMerge: repoData.DeleteBranchOnMerge, DefaultMethod: strings.ToLower(repoData.DefaultMethod), CanUpdate: pr.CanUpdate,
	}
	for _, a := range pr.Assignees.Nodes {
		o.Assignees = append(o.Assignees, a.Login)
	}
	if pr.Milestone != nil {
		o.Milestone = &MilestoneRef{Number: pr.Milestone.Number, Title: pr.Milestone.Title}
	}
	for _, l := range pr.Linked.Nodes {
		o.Linked = append(o.Linked, LinkedIssue(l))
	}
	for _, n := range pr.Commits.Nodes {
		if n.Commit.Rollup == nil {
			continue
		}
		for _, c := range n.Commit.Rollup.Contexts.Nodes {
			if run, ok := c.convert(); ok {
				o.Checks = append(o.Checks, run)
			}
		}
	}
	if q, err := r.queueInfo(ctx, vars, base); err == nil {
		o.Queue, o.AutoMerge, o.CanEnableAutoMerge, o.CanDisableAutoMerge = q.queue, q.auto, q.canEnable, q.canDisable
	}
	return o, nil
}

type gqlLinkedIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type gqlCheckContext struct {
	Type       string `json:"__typename"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	DetailsURL string `json:"detailsUrl"`
	Required   bool   `json:"isRequired"`
	DatabaseID int64  `json:"databaseId"`
	Suite      *struct {
		Run *struct {
			DatabaseID int64 `json:"databaseId"`
		} `json:"workflowRun"`
	} `json:"checkSuite"`
	Context   string `json:"context"`
	State     string `json:"state"`
	TargetURL string `json:"targetUrl"`
}

func (c gqlCheckContext) convert() (CheckRun, bool) {
	switch c.Type {
	case "CheckRun":
		run := CheckRun{Name: c.Name, Status: strings.ToLower(c.Status), Conclusion: strings.ToLower(c.Conclusion), URL: c.DetailsURL, Required: c.Required, JobID: c.DatabaseID}
		if c.Suite != nil && c.Suite.Run != nil {
			run.RunID = c.Suite.Run.DatabaseID
		}
		return run, run.Name != ""
	case "StatusContext":
		state := strings.ToLower(c.State)
		run := CheckRun{Name: c.Context, Status: statusCompleted, Conclusion: state, URL: c.TargetURL, Required: c.Required}
		if state == "pending" || state == "expected" {
			run.Status, run.Conclusion = "in_progress", ""
		}
		return run, run.Name != ""
	}
	return CheckRun{}, false
}

type queueData struct {
	queue      MergeQueue
	auto       *AutoMerge
	canEnable  bool
	canDisable bool
}

func (r *restClient) queueInfo(ctx context.Context, vars map[string]any, base string) (queueData, error) {
	if base == "" {
		return queueData{}, errors.New("prui: base branch is required")
	}
	v := map[string]any{"owner": vars["owner"], "name": vars["name"], "number": vars["number"], "base": base}
	var data struct {
		Repository *struct {
			MergeQueue *struct {
				ID string `json:"id"`
			} `json:"mergeQueue"`
			PullRequest *struct {
				CanEnable  bool `json:"viewerCanEnableAutoMerge"`
				CanDisable bool `json:"viewerCanDisableAutoMerge"`
				InQueue    bool `json:"isInMergeQueue"`
				Auto       *struct {
					Method string   `json:"mergeMethod"`
					By     gqlLogin `json:"enabledBy"`
				} `json:"autoMergeRequest"`
				Entry *struct {
					ID       string `json:"id"`
					State    string `json:"state"`
					Position int    `json:"position"`
				} `json:"mergeQueueEntry"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	if err := r.gql(ctx, queueQuery, v, &data); err != nil {
		return queueData{}, err
	}
	if data.Repository == nil || data.Repository.PullRequest == nil {
		return queueData{}, errors.New("prui: queue response lacks pull request")
	}
	pr := data.Repository.PullRequest
	q := queueData{canEnable: pr.CanEnable, canDisable: pr.CanDisable}
	q.queue.Enabled = data.Repository.MergeQueue != nil
	q.queue.InQueue = pr.InQueue
	if pr.Entry != nil {
		q.queue.EntryID, q.queue.State, q.queue.Position = pr.Entry.ID, pr.Entry.State, pr.Entry.Position
	}
	if pr.Auto != nil {
		q.auto = &AutoMerge{Method: strings.ToLower(pr.Auto.Method), By: pr.Auto.By.Login}
	}
	return q, nil
}

type gqlReactionGroup struct {
	Content string `json:"content"`
	Viewer  bool   `json:"viewerHasReacted"`
	Users   struct {
		Total int `json:"totalCount"`
	} `json:"users"`
}

type gqlTimelineNode struct {
	Type        string    `json:"__typename"`
	ID          string    `json:"id"`
	DatabaseID  int64     `json:"databaseId"`
	URL         string    `json:"url"`
	Body        string    `json:"body"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"createdAt"`
	SubmittedAt time.Time `json:"submittedAt"`
	CanUpdate   bool      `json:"viewerCanUpdate"`
	CanDelete   bool      `json:"viewerCanDelete"`
	Author      *gqlLogin `json:"author"`
	Actor       *gqlLogin `json:"actor"`
	Enabler     *gqlLogin `json:"enabler"`
	Disabler    *gqlLogin `json:"disabler"`
	Assignee    *gqlLogin `json:"assignee"`
	Requested   *gqlLogin `json:"requestedReviewer"`
	Label       *struct {
		Name string `json:"name"`
	} `json:"label"`
	Before *struct {
		OID string `json:"oid"`
	} `json:"beforeCommit"`
	After *struct {
		OID string `json:"oid"`
	} `json:"afterCommit"`
	Commit *struct {
		OID string `json:"oid"`
	} `json:"commit"`
	PreviousTitle string             `json:"previousTitle"`
	CurrentTitle  string             `json:"currentTitle"`
	HeadRefName   string             `json:"headRefName"`
	Reactions     []gqlReactionGroup `json:"reactionGroups"`
}

func loginOf(users ...*gqlLogin) string {
	for _, u := range users {
		if u != nil {
			if u.Login != "" {
				return u.Login
			}
			if u.Name != "" {
				return u.Name
			}
		}
	}
	return ""
}

func (n gqlTimelineNode) convert() (TimelineEvent, bool) {
	e := TimelineEvent{Kind: n.Type, ID: n.ID, DatabaseID: n.DatabaseID, Time: n.CreatedAt, URL: n.URL, Body: n.Body, State: n.State, Actor: loginOf(n.Actor, n.Enabler, n.Disabler)}
	switch n.Type {
	case "IssueComment":
		e.Actor, e.CanEdit, e.CanDelete = loginOf(n.Author), n.CanUpdate, n.CanDelete
		for _, g := range n.Reactions {
			e.Reactions = append(e.Reactions, Reaction{Content: g.Content, Count: g.Users.Total, Viewer: g.Viewer})
		}
	case "PullRequestReview":
		e.Actor, e.Time = loginOf(n.Author), n.SubmittedAt
		if e.State == reviewPending {
			return e, false
		}
	case "LabeledEvent", "UnlabeledEvent":
		if n.Label != nil {
			e.Detail = n.Label.Name
		}
	case "AssignedEvent", "UnassignedEvent":
		e.Detail = loginOf(n.Assignee)
	case "ReviewRequestedEvent", "ReviewRequestRemovedEvent":
		e.Detail = loginOf(n.Requested)
	case "HeadRefForcePushedEvent":
		if n.Before != nil && n.After != nil {
			e.Detail = shortSHA(n.Before.OID) + " to " + shortSHA(n.After.OID)
		}
	case "MergedEvent":
		if n.Commit != nil {
			e.Detail = shortSHA(n.Commit.OID)
		}
	case "RenamedTitleEvent":
		e.Detail = n.PreviousTitle + " to " + n.CurrentTitle
	case "HeadRefDeletedEvent":
		e.Detail = n.HeadRefName
	case "ClosedEvent", "ReopenedEvent", "ReadyForReviewEvent", "ConvertToDraftEvent", "AutoMergeEnabledEvent", "AutoMergeDisabledEvent":
	default:
		return e, false
	}
	return e, true
}

func (r *restClient) Timeline(ctx context.Context, repo github.Remote, number int) ([]TimelineEvent, error) {
	if _, err := repoPath(repo); err != nil {
		return nil, err
	}
	var events []TimelineEvent
	var after *string
	for range maxTimelinePages {
		var data struct {
			Repository *struct {
				PullRequest *struct {
					Items struct {
						PageInfo struct {
							HasNext bool   `json:"hasNextPage"`
							End     string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []gqlTimelineNode `json:"nodes"`
					} `json:"timelineItems"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}
		vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number, "after": after}
		if err := r.gql(ctx, timelineQuery, vars, &data); err != nil {
			return nil, err
		}
		if data.Repository == nil || data.Repository.PullRequest == nil {
			return nil, fmt.Errorf("prui: pull request %d not found", number)
		}
		page := data.Repository.PullRequest.Items
		for _, n := range page.Nodes {
			if e, ok := n.convert(); ok {
				events = append(events, e)
			}
		}
		if !page.PageInfo.HasNext || page.PageInfo.End == "" {
			return events, nil
		}
		cursor := page.PageInfo.End
		after = &cursor
	}
	return events, nil
}

func (r *restClient) ViewedFiles(ctx context.Context, repo github.Remote, number int) (map[string]bool, error) {
	viewed := map[string]bool{}
	var after *string
	for range maxTimelinePages {
		var data struct {
			Repository *struct {
				PullRequest *struct {
					Files struct {
						PageInfo struct {
							HasNext bool   `json:"hasNextPage"`
							End     string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []struct {
							Path  string `json:"path"`
							State string `json:"viewerViewedState"`
						} `json:"nodes"`
					} `json:"files"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}
		vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number, "after": after}
		if err := r.gql(ctx, viewedQuery, vars, &data); err != nil {
			return nil, err
		}
		if data.Repository == nil || data.Repository.PullRequest == nil {
			return nil, fmt.Errorf("prui: pull request %d not found", number)
		}
		page := data.Repository.PullRequest.Files
		for _, f := range page.Nodes {
			viewed[f.Path] = f.State == "VIEWED"
		}
		if !page.PageInfo.HasNext || page.PageInfo.End == "" {
			return viewed, nil
		}
		cursor := page.PageInfo.End
		after = &cursor
	}
	return viewed, nil
}

func (r *restClient) mutate(ctx context.Context, query string, vars map[string]any) error {
	var data map[string]any
	return r.gql(ctx, query, vars, &data)
}

func (r *restClient) SetViewed(ctx context.Context, prID, path string, viewed bool) error {
	query := markViewedMutation
	if !viewed {
		query = unmarkViewedMutation
	}
	return r.mutate(ctx, query, map[string]any{"id": prID, "path": path})
}

func (r *restClient) Compare(ctx context.Context, repo github.Remote, base, head string) (Comparison, error) {
	root, err := repoPath(repo)
	if err != nil {
		return Comparison{}, err
	}
	var raw struct {
		MergeBase *struct {
			SHA string `json:"sha"`
		} `json:"merge_base_commit"`
		Files []struct {
			Filename string `json:"filename"`
			Previous string `json:"previous_filename"`
			Status   string `json:"status"`
			Adds     int    `json:"additions"`
			Dels     int    `json:"deletions"`
			Patch    string `json:"patch"`
		} `json:"files"`
		Commits []struct {
			SHA    string `json:"sha"`
			URL    string `json:"html_url"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name string    `json:"name"`
					Date time.Time `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		} `json:"commits"`
	}
	path := root + "/compare/" + url.PathEscape(base) + "..." + url.PathEscape(head) + "?per_page=" + strconv.Itoa(pageSizeGraphQL)
	if err := r.json(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return Comparison{}, err
	}
	out := Comparison{}
	if raw.MergeBase != nil {
		out.BaseSHA = raw.MergeBase.SHA
	}
	for _, f := range raw.Files {
		if f.Filename == "" {
			return Comparison{}, errors.New("prui: compare file lacks filename")
		}
		out.Files = append(out.Files, github.PRFile{Path: f.Filename, PrevPath: f.Previous, Status: f.Status, Additions: f.Adds, Deletions: f.Dels, Patch: f.Patch})
	}
	for _, c := range raw.Commits {
		out.Commits = append(out.Commits, github.Commit{SHA: c.SHA, Message: c.Commit.Message, Author: c.Commit.Author.Name, Date: c.Commit.Author.Date, URL: c.URL})
	}
	return out, nil
}

func (r *restClient) UpdateBranch(ctx context.Context, repo github.Remote, number int, headSHA string) error {
	root, err := repoPath(repo)
	if err != nil {
		return err
	}
	payload := map[string]any{}
	if headSHA != "" {
		payload["expected_head_sha"] = headSHA
	}
	return r.json(ctx, http.MethodPut, fmt.Sprintf("%s/pulls/%d/update-branch", root, number), payload, nil)
}

func (r *restClient) ConvertToDraft(ctx context.Context, prID string) error {
	return r.mutate(ctx, draftMutation, map[string]any{"id": prID})
}

func (r *restClient) SetAutoMerge(ctx context.Context, prID, method string, enable bool) error {
	if !enable {
		return r.mutate(ctx, disableAutoMutation, map[string]any{"id": prID})
	}
	return r.mutate(ctx, enableAutoMutation, map[string]any{"id": prID, "method": strings.ToUpper(method)})
}

func (r *restClient) Enqueue(ctx context.Context, prID string) error {
	return r.mutate(ctx, enqueueMutation, map[string]any{"id": prID})
}

func (r *restClient) Dequeue(ctx context.Context, entryID string) error {
	return r.mutate(ctx, dequeueMutation, map[string]any{"id": entryID})
}

func (r *restClient) DeleteBranch(ctx context.Context, repo github.Remote, ref string) error {
	root, err := repoPath(repo)
	if err != nil {
		return err
	}
	if ref == "" {
		return errors.New("prui: branch name is required")
	}
	return r.json(ctx, http.MethodDelete, root+"/git/refs/heads/"+escapePath(ref), nil, nil)
}

func (r *restClient) Revert(ctx context.Context, prID string) (int, error) {
	var data struct {
		Payload *struct {
			PR *struct {
				Number int `json:"number"`
			} `json:"revertPullRequest"`
		} `json:"revertPullRequest"`
	}
	if err := r.gql(ctx, revertMutation, map[string]any{"id": prID}, &data); err != nil {
		return 0, err
	}
	if data.Payload == nil || data.Payload.PR == nil {
		return 0, errors.New("prui: revert response lacks pull request")
	}
	return data.Payload.PR.Number, nil
}

func (r *restClient) React(ctx context.Context, subjectID, content string, add bool) error {
	query := addReactionMutation
	if !add {
		query = delReactionMutation
	}
	return r.mutate(ctx, query, map[string]any{"id": subjectID, "content": content})
}

func (r *restClient) EditComment(ctx context.Context, kind, id, body string) error {
	query := editIssueComment
	if kind == commentReview {
		query = editReviewComment
	}
	return r.mutate(ctx, query, map[string]any{"id": id, "body": body})
}

func (r *restClient) DeleteComment(ctx context.Context, kind, id string) error {
	query := deleteIssueComment
	if kind == commentReview {
		query = deleteReviewComment
	}
	return r.mutate(ctx, query, map[string]any{"id": id})
}

func (r *restClient) DeleteReview(ctx context.Context, reviewID string) error {
	return r.mutate(ctx, deleteReviewMutation, map[string]any{"id": reviewID})
}

func (r *restClient) AddFileThread(ctx context.Context, reviewID, path, body string) error {
	return r.mutate(ctx, fileThreadMutation, map[string]any{"review": reviewID, "path": path, "body": body})
}

func (r *restClient) JobLogs(ctx context.Context, repo github.Remote, jobID int64) (string, error) {
	root, err := repoPath(repo)
	if err != nil {
		return "", err
	}
	if jobID <= 0 {
		return "", errors.New("prui: job id is required")
	}
	res, err := r.send(ctx, request{method: http.MethodGet, target: fmt.Sprintf("%s%s/actions/jobs/%d/logs", r.base, root, jobID), accept: acceptText, limit: maxLogBytes, tail: true})
	if err != nil {
		return "", err
	}
	return string(res.data), nil
}

func (r *restClient) RunAction(ctx context.Context, repo github.Remote, runID int64, action string) error {
	root, err := repoPath(repo)
	if err != nil {
		return err
	}
	switch action {
	case actionRerun, actionRerunFailed, actionCancel, actionApprove:
	default:
		return fmt.Errorf("prui: invalid workflow action %q", action)
	}
	if runID <= 0 {
		return errors.New("prui: workflow run id is required")
	}
	return r.json(ctx, http.MethodPost, fmt.Sprintf("%s/actions/runs/%d/%s", root, runID, action), nil, nil)
}

func (r *restClient) Notifications(ctx context.Context, repo github.Remote, modifiedSince string) (NotificationPage, error) {
	root, err := repoPath(repo)
	if err != nil {
		return NotificationPage{}, err
	}
	header := map[string]string{}
	if modifiedSince != "" {
		header["If-Modified-Since"] = modifiedSince
	}
	res, err := r.send(ctx, request{method: http.MethodGet, target: fmt.Sprintf("%s%s/notifications?per_page=%d", r.base, root, notificationsPage), header: header})
	if err != nil {
		return NotificationPage{}, err
	}
	page := NotificationPage{Modified: res.header.Get("Last-Modified"), Poll: minPollInterval}
	if secs, err := strconv.Atoi(res.header.Get("X-Poll-Interval")); err == nil && time.Duration(secs)*time.Second > page.Poll {
		page.Poll = time.Duration(secs) * time.Second
	}
	if res.status == http.StatusNotModified {
		page.NotModified = true
		return page, nil
	}
	items, err := decodeNotifications(res.data)
	if err != nil {
		return NotificationPage{}, err
	}
	page.Items = items
	return page, nil
}

func (r *restClient) MarkNotification(ctx context.Context, id string, done bool) error {
	if id == "" || strings.ContainsAny(id, "/?#") {
		return fmt.Errorf("prui: invalid notification id %q", id)
	}
	method := http.MethodPatch
	if done {
		method = http.MethodDelete
	}
	return r.json(ctx, method, "/notifications/threads/"+url.PathEscape(id), nil, nil)
}

func (r *restClient) LocalBranchPRs(ctx context.Context, repo github.Remote, branches []string) ([]github.PullRequest, error) {
	if len(branches) == 0 {
		return nil, nil
	}
	branches = branches[:min(len(branches), maxLocalBranchPRs)]
	var decl, body strings.Builder
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name}
	for i, b := range branches {
		decl.WriteString(fmt.Sprintf(",$b%d:String!", i))
		body.WriteString(fmt.Sprintf("b%d:pullRequests(first:1,states:[OPEN],headRefName:$b%d,orderBy:{field:CREATED_AT,direction:DESC}){nodes{number title isDraft url updatedAt author{login} headRefName baseRefName headRefOid}} ", i, i))
		vars[fmt.Sprintf("b%d", i)] = b
	}
	query := "query($owner:String!,$name:String!" + decl.String() + "){repository(owner:$owner,name:$name){" + body.String() + "}}"
	var data struct {
		Repository map[string]struct {
			Nodes []struct {
				Number    int       `json:"number"`
				Title     string    `json:"title"`
				IsDraft   bool      `json:"isDraft"`
				URL       string    `json:"url"`
				UpdatedAt time.Time `json:"updatedAt"`
				Author    *gqlLogin `json:"author"`
				HeadRef   string    `json:"headRefName"`
				BaseRef   string    `json:"baseRefName"`
				HeadSHA   string    `json:"headRefOid"`
			} `json:"nodes"`
		} `json:"repository"`
	}
	if err := r.gql(ctx, query, vars, &data); err != nil {
		return nil, err
	}
	var prs []github.PullRequest
	for i := range branches {
		for _, n := range data.Repository[fmt.Sprintf("b%d", i)].Nodes {
			prs = append(prs, github.PullRequest{Number: n.Number, Title: n.Title, State: github.StateOpen, Draft: n.IsDraft, URL: n.URL, Updated: n.UpdatedAt, Author: loginOf(n.Author), HeadRef: n.HeadRef, BaseRef: n.BaseRef, HeadSHA: n.HeadSHA})
		}
	}
	return prs, nil
}
