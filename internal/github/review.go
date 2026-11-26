package github

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	SideLeft  = "LEFT"
	SideRight = "RIGHT"
)

type Comment struct {
	ID         string
	DatabaseID int64
	Author     string
	Body       string
	Created    time.Time
	URL        string
}

type Thread struct {
	ID        string
	Path      string
	Line      int
	StartLine int
	Side      string
	Resolved  bool
	Outdated  bool
	Comments  []Comment
}

type Review struct {
	ID         string
	DatabaseID int64
	Author     string
	State      string
	Body       string
	CommitID   string
	Submitted  time.Time
	URL        string
}

type apiComment struct {
	NodeID    string    `json:"node_id"`
	ID        int64     `json:"id"`
	User      *apiUser  `json:"user"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	HTMLURL   string    `json:"html_url"`
}

func (a apiComment) convert() (Comment, error) {
	if a.ID == 0 {
		return Comment{}, errors.New("github: comment lacks id")
	}
	return Comment{ID: a.NodeID, DatabaseID: a.ID, Author: userLogin(a.User), Body: a.Body, Created: a.CreatedAt, URL: a.HTMLURL}, nil
}

func (c *Client) Comments(ctx context.Context, repo Remote, number int) ([]Comment, error) {
	base, err := repoURL(repo)
	if err != nil {
		return nil, err
	}
	if err := checkNumber(number); err != nil {
		return nil, err
	}
	raw, err := listAll[apiComment](ctx, c, fmt.Sprintf("%s/issues/%d/comments?per_page=%s", base, number, maxPerPageText))
	if err != nil {
		return nil, err
	}
	comments := make([]Comment, 0, len(raw))
	for _, item := range raw {
		comment, err := item.convert()
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, nil
}

func (c *Client) Reviews(ctx context.Context, repo Remote, number int) ([]Review, error) {
	path, err := pullSubpath(repo, number, "reviews")
	if err != nil {
		return nil, err
	}
	type apiReview struct {
		NodeID      string    `json:"node_id"`
		ID          int64     `json:"id"`
		User        *apiUser  `json:"user"`
		Body        string    `json:"body"`
		State       string    `json:"state"`
		CommitID    string    `json:"commit_id"`
		SubmittedAt time.Time `json:"submitted_at"`
		HTMLURL     string    `json:"html_url"`
	}
	raw, err := listAll[apiReview](ctx, c, path)
	if err != nil {
		return nil, err
	}
	reviews := make([]Review, 0, len(raw))
	for _, item := range raw {
		if item.ID == 0 || item.State == "" {
			return nil, errors.New("github: review lacks id or state")
		}
		reviews = append(reviews, Review{ID: item.NodeID, DatabaseID: item.ID, Author: userLogin(item.User), State: item.State, Body: item.Body, CommitID: item.CommitID, Submitted: item.SubmittedAt, URL: item.HTMLURL})
	}
	return reviews, nil
}

const commentFields = `id databaseId url body createdAt author{login}`

const threadFields = `id isResolved isOutdated path diffSide line startLine originalLine originalStartLine comments(first:100){nodes{` + commentFields + `}}`

const reviewThreadsQuery = `query($owner:String!,$name:String!,$number:Int!,$after:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewThreads(first:100,after:$after){pageInfo{hasNextPage endCursor} nodes{` + threadFields + `}}}}}`

type gqlComment struct {
	ID         string    `json:"id"`
	DatabaseID int64     `json:"databaseId"`
	URL        string    `json:"url"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	Author     *apiUser  `json:"author"`
}

type gqlThread struct {
	ID                string `json:"id"`
	IsResolved        bool   `json:"isResolved"`
	IsOutdated        bool   `json:"isOutdated"`
	Path              string `json:"path"`
	DiffSide          string `json:"diffSide"`
	Line              *int   `json:"line"`
	StartLine         *int   `json:"startLine"`
	OriginalLine      *int   `json:"originalLine"`
	OriginalStartLine *int   `json:"originalStartLine"`
	Comments          *struct {
		Nodes []gqlComment `json:"nodes"`
	} `json:"comments"`
}

func firstSet(values ...*int) int {
	for _, v := range values {
		if v != nil {
			return *v
		}
	}
	return 0
}

func (g gqlComment) convert() (Comment, error) {
	if g.ID == "" {
		return Comment{}, errors.New("github: graphql comment lacks id")
	}
	return Comment{ID: g.ID, DatabaseID: g.DatabaseID, Author: userLogin(g.Author), Body: g.Body, Created: g.CreatedAt, URL: g.URL}, nil
}

func (g gqlThread) convert() (Thread, error) {
	if g.ID == "" || g.Path == "" {
		return Thread{}, errors.New("github: review thread lacks id or path")
	}
	thread := Thread{
		ID:        g.ID,
		Path:      g.Path,
		Line:      firstSet(g.Line, g.OriginalLine),
		StartLine: firstSet(g.StartLine, g.OriginalStartLine),
		Side:      g.DiffSide,
		Resolved:  g.IsResolved,
		Outdated:  g.IsOutdated,
	}
	if thread.Side == "" {
		thread.Side = SideRight
	}
	if g.Comments != nil {
		for _, node := range g.Comments.Nodes {
			comment, err := node.convert()
			if err != nil {
				return Thread{}, err
			}
			thread.Comments = append(thread.Comments, comment)
		}
	}
	return thread, nil
}

func (c *Client) ReviewThreads(ctx context.Context, repo Remote, number int) ([]Thread, error) {
	if _, err := repoURL(repo); err != nil {
		return nil, err
	}
	if err := checkNumber(number); err != nil {
		return nil, err
	}
	var threads []Thread
	var after *string
	for range maxPages {
		var data struct {
			Repository *struct {
				PullRequest *struct {
					ReviewThreads *struct {
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []gqlThread `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}
		vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number, "after": after}
		if err := c.graphql(ctx, reviewThreadsQuery, vars, &data); err != nil {
			return nil, err
		}
		if data.Repository == nil || data.Repository.PullRequest == nil || data.Repository.PullRequest.ReviewThreads == nil {
			return nil, fmt.Errorf("github: pull request %d not found", number)
		}
		page := data.Repository.PullRequest.ReviewThreads
		for _, node := range page.Nodes {
			thread, err := node.convert()
			if err != nil {
				return nil, err
			}
			threads = append(threads, thread)
		}
		if !page.PageInfo.HasNextPage {
			return threads, nil
		}
		if page.PageInfo.EndCursor == "" {
			return nil, errors.New("github: graphql page lacks end cursor")
		}
		cursor := page.PageInfo.EndCursor
		after = &cursor
	}
	return nil, fmt.Errorf("github: more than %d thread pages", maxPages)
}
