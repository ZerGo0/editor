package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

const (
	MergeMethodMerge  = "merge"
	MergeMethodSquash = "squash"
	MergeMethodRebase = "rebase"

	ReviewApprove        = "APPROVE"
	ReviewRequestChanges = "REQUEST_CHANGES"
	ReviewComment        = "COMMENT"
)

type CreateOptions struct {
	Title string
	Body  string
	Head  string
	Base  string
	Draft bool
}

type ReviewCommentOptions struct {
	Body      string
	Path      string
	Line      int
	Side      string
	StartLine int
	CommitID  string
}

func (o ReviewCommentOptions) validate() (side string, err error) {
	if o.Body == "" || o.Path == "" {
		return "", errors.New("github: review comment needs body and path")
	}
	if o.Line <= 0 {
		return "", fmt.Errorf("github: invalid line %d", o.Line)
	}
	if o.StartLine < 0 || (o.StartLine > 0 && o.StartLine >= o.Line) {
		return "", fmt.Errorf("github: start line %d must be before line %d", o.StartLine, o.Line)
	}
	switch o.Side {
	case "", SideRight:
		return SideRight, nil
	case SideLeft:
		return SideLeft, nil
	}
	return "", fmt.Errorf("github: invalid side %q", o.Side)
}

const (
	replyMutation = `mutation($thread:ID!,$body:String!){addPullRequestReviewThreadReply(input:{pullRequestReviewThreadId:$thread,body:$body}){comment{` + commentFields + `}}}`

	addThreadMutation = `mutation($review:ID!,$path:String!,$body:String!,$line:Int!,$side:DiffSide!,$startLine:Int,$startSide:DiffSide){addPullRequestReviewThread(input:{pullRequestReviewId:$review,path:$path,body:$body,line:$line,side:$side,startLine:$startLine,startSide:$startSide}){thread{` + threadFields + `}}}`

	submitReviewMutation = `mutation($review:ID!,$event:PullRequestReviewEvent!,$body:String){submitPullRequestReview(input:{pullRequestReviewId:$review,event:$event,body:$body}){pullRequestReview{id state}}}`

	resolveThreadMutation   = `mutation($thread:ID!){resolveReviewThread(input:{threadId:$thread}){thread{id isResolved}}}`
	unresolveThreadMutation = `mutation($thread:ID!){unresolveReviewThread(input:{threadId:$thread}){thread{id isResolved}}}`

	pullRequestIDQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){pullRequest(number:$number){id}}}`

	markReadyMutation = `mutation($id:ID!){markPullRequestReadyForReview(input:{pullRequestId:$id}){pullRequest{id isDraft}}}`
)

func (c *Client) AddComment(ctx context.Context, repo Remote, number int, body string) (Comment, error) {
	base, err := repoURL(repo)
	if err != nil {
		return Comment{}, err
	}
	if err := checkNumber(number); err != nil {
		return Comment{}, err
	}
	if body == "" {
		return Comment{}, errors.New("github: comment body is empty")
	}
	var raw apiComment
	path := fmt.Sprintf("%s/issues/%d/comments", base, number)
	if _, err := c.do(ctx, http.MethodPost, path, map[string]any{"body": body}, &raw); err != nil {
		return Comment{}, err
	}
	return raw.convert()
}

func (c *Client) Reply(ctx context.Context, threadID, body string) (Comment, error) {
	if threadID == "" || body == "" {
		return Comment{}, errors.New("github: reply needs thread id and body")
	}
	var data struct {
		Payload *struct {
			Comment *gqlComment `json:"comment"`
		} `json:"addPullRequestReviewThreadReply"`
	}
	if err := c.graphql(ctx, replyMutation, map[string]any{"thread": threadID, "body": body}, &data); err != nil {
		return Comment{}, err
	}
	if data.Payload == nil || data.Payload.Comment == nil {
		return Comment{}, errors.New("github: reply response lacks comment")
	}
	return data.Payload.Comment.convert()
}

func (c *Client) ReplyToComment(ctx context.Context, repo Remote, number int, commentID int64, body string) (Comment, error) {
	base, err := repoURL(repo)
	if err != nil {
		return Comment{}, err
	}
	if err := checkNumber(number); err != nil {
		return Comment{}, err
	}
	if commentID <= 0 || body == "" {
		return Comment{}, errors.New("github: reply needs comment id and body")
	}
	var raw apiComment
	path := fmt.Sprintf("%s/pulls/%d/comments/%d/replies", base, number, commentID)
	if _, err := c.do(ctx, http.MethodPost, path, map[string]any{"body": body}, &raw); err != nil {
		return Comment{}, err
	}
	return raw.convert()
}

func (c *Client) CreateReviewComment(ctx context.Context, repo Remote, number int, opts ReviewCommentOptions) (Comment, error) {
	base, err := repoURL(repo)
	if err != nil {
		return Comment{}, err
	}
	if err := checkNumber(number); err != nil {
		return Comment{}, err
	}
	side, err := opts.validate()
	if err != nil {
		return Comment{}, err
	}
	if opts.CommitID == "" {
		return Comment{}, errors.New("github: review comment needs commit id")
	}
	payload := map[string]any{"body": opts.Body, "commit_id": opts.CommitID, "path": opts.Path, "line": opts.Line, "side": side}
	if opts.StartLine > 0 {
		payload["start_line"] = opts.StartLine
		payload["start_side"] = side
	}
	var raw apiComment
	if _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("%s/pulls/%d/comments", base, number), payload, &raw); err != nil {
		return Comment{}, err
	}
	return raw.convert()
}

func (c *Client) StartReview(ctx context.Context, repo Remote, number int) (string, error) {
	base, err := repoURL(repo)
	if err != nil {
		return "", err
	}
	if err := checkNumber(number); err != nil {
		return "", err
	}
	var raw struct {
		NodeID string `json:"node_id"`
	}
	if _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("%s/pulls/%d/reviews", base, number), map[string]any{}, &raw); err != nil {
		return "", err
	}
	if raw.NodeID == "" {
		return "", errors.New("github: review response lacks node_id")
	}
	return raw.NodeID, nil
}

func (c *Client) AddReviewComment(ctx context.Context, reviewID string, opts ReviewCommentOptions) (Thread, error) {
	if reviewID == "" {
		return Thread{}, errors.New("github: review id is required")
	}
	side, err := opts.validate()
	if err != nil {
		return Thread{}, err
	}
	vars := map[string]any{"review": reviewID, "path": opts.Path, "body": opts.Body, "line": opts.Line, "side": side}
	if opts.StartLine > 0 {
		vars["startLine"] = opts.StartLine
		vars["startSide"] = side
	}
	var data struct {
		Payload *struct {
			Thread *gqlThread `json:"thread"`
		} `json:"addPullRequestReviewThread"`
	}
	if err := c.graphql(ctx, addThreadMutation, vars, &data); err != nil {
		return Thread{}, err
	}
	if data.Payload == nil || data.Payload.Thread == nil {
		return Thread{}, errors.New("github: add review comment response lacks thread")
	}
	return data.Payload.Thread.convert()
}

func (c *Client) SubmitReview(ctx context.Context, reviewID, event, body string) error {
	if reviewID == "" {
		return errors.New("github: review id is required")
	}
	switch event {
	case ReviewApprove, ReviewRequestChanges, ReviewComment:
	default:
		return fmt.Errorf("github: invalid review event %q", event)
	}
	vars := map[string]any{"review": reviewID, "event": event, "body": nil}
	if body != "" {
		vars["body"] = body
	}
	var data struct {
		Payload *struct {
			Review *struct {
				ID string `json:"id"`
			} `json:"pullRequestReview"`
		} `json:"submitPullRequestReview"`
	}
	if err := c.graphql(ctx, submitReviewMutation, vars, &data); err != nil {
		return err
	}
	if data.Payload == nil || data.Payload.Review == nil {
		return errors.New("github: submit review response lacks review")
	}
	return nil
}

func (c *Client) ResolveThread(ctx context.Context, threadID string) error {
	return c.setThreadResolved(ctx, threadID, resolveThreadMutation, "resolveReviewThread", true)
}

func (c *Client) UnresolveThread(ctx context.Context, threadID string) error {
	return c.setThreadResolved(ctx, threadID, unresolveThreadMutation, "unresolveReviewThread", false)
}

func (c *Client) setThreadResolved(ctx context.Context, threadID, mutation, field string, want bool) error {
	if threadID == "" {
		return errors.New("github: thread id is required")
	}
	var data map[string]*struct {
		Thread *struct {
			IsResolved bool `json:"isResolved"`
		} `json:"thread"`
	}
	if err := c.graphql(ctx, mutation, map[string]any{"thread": threadID}, &data); err != nil {
		return err
	}
	payload := data[field]
	if payload == nil || payload.Thread == nil {
		return fmt.Errorf("github: %s response lacks thread", field)
	}
	if payload.Thread.IsResolved != want {
		return fmt.Errorf("github: thread %s did not change state", threadID)
	}
	return nil
}

func (c *Client) Merge(ctx context.Context, repo Remote, number int, method, title, message string) error {
	base, err := repoURL(repo)
	if err != nil {
		return err
	}
	if err := checkNumber(number); err != nil {
		return err
	}
	switch method {
	case MergeMethodMerge, MergeMethodSquash, MergeMethodRebase:
	default:
		return fmt.Errorf("github: invalid merge method %q", method)
	}
	payload := map[string]any{"merge_method": method}
	if title != "" {
		payload["commit_title"] = title
	}
	if message != "" {
		payload["commit_message"] = message
	}
	var result struct {
		Merged  bool   `json:"merged"`
		Message string `json:"message"`
	}
	if _, err := c.do(ctx, http.MethodPut, fmt.Sprintf("%s/pulls/%d/merge", base, number), payload, &result); err != nil {
		return err
	}
	if !result.Merged {
		return fmt.Errorf("github: pull request %d not merged: %s", number, result.Message)
	}
	return nil
}

func (c *Client) Create(ctx context.Context, repo Remote, opts CreateOptions) (PullRequest, error) {
	base, err := repoURL(repo)
	if err != nil {
		return PullRequest{}, err
	}
	if opts.Title == "" || opts.Head == "" || opts.Base == "" {
		return PullRequest{}, errors.New("github: create needs title, head and base")
	}
	payload := map[string]any{"title": opts.Title, "body": opts.Body, "head": opts.Head, "base": opts.Base, "draft": opts.Draft}
	var raw apiPull
	if _, err := c.do(ctx, http.MethodPost, base+"/pulls", payload, &raw); err != nil {
		return PullRequest{}, err
	}
	return raw.convert()
}

func (c *Client) MarkReady(ctx context.Context, repo Remote, number int) error {
	if _, err := repoURL(repo); err != nil {
		return err
	}
	if err := checkNumber(number); err != nil {
		return err
	}
	var idData struct {
		Repository *struct {
			PullRequest *struct {
				ID string `json:"id"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number}
	if err := c.graphql(ctx, pullRequestIDQuery, vars, &idData); err != nil {
		return err
	}
	if idData.Repository == nil || idData.Repository.PullRequest == nil || idData.Repository.PullRequest.ID == "" {
		return fmt.Errorf("github: pull request %d not found", number)
	}
	var data struct {
		Payload *struct {
			PullRequest *struct {
				IsDraft bool `json:"isDraft"`
			} `json:"pullRequest"`
		} `json:"markPullRequestReadyForReview"`
	}
	if err := c.graphql(ctx, markReadyMutation, map[string]any{"id": idData.Repository.PullRequest.ID}, &data); err != nil {
		return err
	}
	if data.Payload == nil || data.Payload.PullRequest == nil || data.Payload.PullRequest.IsDraft {
		return fmt.Errorf("github: pull request %d is still a draft", number)
	}
	return nil
}

func (c *Client) RequestReviewers(ctx context.Context, repo Remote, number int, users, teams []string) error {
	base, err := repoURL(repo)
	if err != nil {
		return err
	}
	if err := checkNumber(number); err != nil {
		return err
	}
	if len(users) == 0 && len(teams) == 0 {
		return errors.New("github: no reviewers given")
	}
	payload := map[string]any{"reviewers": nonNil(users), "team_reviewers": nonNil(teams)}
	_, err = c.do(ctx, http.MethodPost, fmt.Sprintf("%s/pulls/%d/requested_reviewers", base, number), payload, nil)
	return err
}

func (c *Client) SetLabels(ctx context.Context, repo Remote, number int, labels []string) error {
	base, err := repoURL(repo)
	if err != nil {
		return err
	}
	if err := checkNumber(number); err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodPut, fmt.Sprintf("%s/issues/%d/labels", base, number), map[string]any{"labels": nonNil(labels)}, nil)
	return err
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
