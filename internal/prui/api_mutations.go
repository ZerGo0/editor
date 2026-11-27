package prui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
)

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
